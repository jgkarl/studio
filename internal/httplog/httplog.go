// Package httplog provides one access-log line per HTTP request plus a slog handler wrapper that
// stamps every log line emitted anywhere during that request (as long as the request's context is
// threaded through — which it is, everywhere in this app, via r.Context()) with the same
// request_id. That's the actual point: `grep request_id=<id>` (or `jq 'select(.request_id==...)'`
// with LOG_FORMAT=json) pulls the access log line and every app-level error it triggered together,
// which is normally the hard part of debugging a single bad request from journalctl output.
package httplog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ctxKey struct{}

var requestIDKey = ctxKey{}

// RequestID returns the current request's id, or "-" if called outside a request Middleware
// handled (e.g. from a background job).
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return "-"
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// sensitiveQueryParams are redacted before a query string is logged - "token" carries single-use
// email-verification/password-reset tokens (see internal/auth/handlers.go's ?token=... links);
// logging one verbatim would let anyone who can read the logs use a still-valid link themselves.
var sensitiveQueryParams = []string{"token"}

func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	q, err := url.ParseQuery(raw)
	if err != nil {
		return "" // malformed query string - not worth logging verbatim either
	}
	redacted := false
	for _, key := range sensitiveQueryParams {
		if q.Has(key) {
			q.Set(key, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return raw
	}
	return q.Encode()
}

// clientIP prefers the left-most X-Forwarded-For entry (the original client, as set by the Caddy
// reverse proxy in front of this app in production - see ansible/roles/studio_app) over
// r.RemoteAddr, which behind a reverse proxy is just the proxy's own address.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first, _, ok := strings.Cut(fwd, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(fwd)
	}
	return r.RemoteAddr
}

// statusRecorder wraps http.ResponseWriter to capture the status code and byte count a handler
// actually wrote - net/http doesn't expose either after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	if !rec.wroteHeader {
		rec.status = code
		rec.wroteHeader = true
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.status = http.StatusOK
		rec.wroteHeader = true
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.bytes += n
	return n, err
}

// Middleware logs one "request" line per HTTP request - method, path, redacted query, status,
// duration, response size, and client IP - and assigns a request_id (via context, picked up
// automatically by ContextHandler below) that ties this line to any app-level log line emitted
// while handling it. Wrap the top-level mux with this in cmd/server/main.go so every route,
// including static assets, is covered.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestIDKey, newRequestID())
		r = r.WithContext(ctx)

		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(rec, r)
		dur := time.Since(start)

		status := rec.status
		if status == 0 {
			status = http.StatusOK // handler never wrote anything - still want a log line
		}

		attrs := []any{
			"category", "http",
			"event", "http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", dur.Milliseconds(),
			"bytes", rec.bytes,
			"client_ip", clientIP(r),
		}
		if q := redactQuery(r.URL.RawQuery); q != "" {
			attrs = append(attrs, "query", q)
		}

		switch {
		case status >= 500:
			slog.ErrorContext(ctx, "request", attrs...)
		case status >= 400:
			slog.WarnContext(ctx, "request", attrs...)
		default:
			slog.InfoContext(ctx, "request", attrs...)
		}
	})
}

// ContextHandler wraps a slog.Handler so any log line emitted through a context descending from
// Middleware's (i.e. r.Context() from inside an HTTP handler, or anything derived from it - which
// is every ctx in this app, since it's threaded through on every DB call) gets that request's
// request_id attached automatically, without every call site having to pass it by hand.
type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs/WithGroup must be forwarded explicitly and re-wrapped - embedding alone would make
// those calls return the inner slog.Handler's own type, silently dropping request-id injection
// from every logger built via .With(...)/.WithGroup(...) after that point.
func (h ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextHandler{h.Handler.WithAttrs(attrs)}
}

func (h ContextHandler) WithGroup(name string) slog.Handler {
	return ContextHandler{h.Handler.WithGroup(name)}
}
