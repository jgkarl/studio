package httplog

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactQuery(t *testing.T) {
	cases := map[string]string{
		"":                     "",
		"tab=assessments":      "tab=assessments",
		"token=super-secret":   "token=REDACTED",
		"a=1&token=secret&b=2": "a=1&b=2&token=REDACTED",
		"%zz":                  "", // malformed - not logged verbatim
	}
	for in, want := range cases {
		got := redactQuery(in)
		if got != want {
			t.Errorf("redactQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMiddlewareLogsStatusAndRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(ContextHandler{Handler: slog.NewTextHandler(&buf, nil)})
	old := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(old)

	var sawRequestID string
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequestID = RequestID(r.Context())
		slog.ErrorContext(r.Context(), "handler-level error", "detail", "boom")
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/clients?token=abc123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if sawRequestID == "" || sawRequestID == "-" {
		t.Fatalf("handler did not see a real request id, got %q", sawRequestID)
	}

	out := buf.String()
	if !strings.Contains(out, "status=404") {
		t.Errorf("access log missing status=404, got: %s", out)
	}
	if !strings.Contains(out, "token=REDACTED") {
		t.Errorf("access log did not redact token query param, got: %s", out)
	}
	if strings.Contains(out, "abc123") {
		t.Errorf("access log leaked the raw token value, got: %s", out)
	}

	// Both the access-log line and the handler's own error log must carry the same request_id -
	// that correlation is the entire point of ContextHandler.
	firstIDIdx := strings.Index(out, "request_id="+sawRequestID)
	if firstIDIdx == -1 {
		t.Fatalf("no log line carries request_id=%s, got: %s", sawRequestID, out)
	}
	if strings.Count(out, "request_id="+sawRequestID) < 2 {
		t.Errorf("expected both the access log and the handler-level log to carry request_id=%s, got: %s", sawRequestID, out)
	}
	if !strings.Contains(out, "handler-level error") {
		t.Errorf("handler-level log line missing entirely, got: %s", out)
	}
}

func TestMiddlewareDefaultsTo200WhenHandlerWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(ContextHandler{Handler: slog.NewTextHandler(&buf, nil)}))
	defer slog.SetDefault(old)

	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), "status=200") {
		t.Errorf("expected status=200 for a handler that never wrote, got: %s", buf.String())
	}
}
