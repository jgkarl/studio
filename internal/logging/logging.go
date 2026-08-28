// Package logging builds the process-wide *slog.Logger from config.Config - the only place
// LOG_LEVEL/LOG_FORMAT/LOG_DIR get interpreted. cmd/server/main.go calls New once at startup and
// installs the result via slog.SetDefault; every other package just calls the top-level
// slog.Info/Error/... functions (or the *Context variants, to get request_id correlation - see
// internal/httplog).
//
// Every log call site in this app is expected to include "category" (which module/subsystem -
// "auth", "media", "http", ...) and "event" (a short machine-readable slug for what happened -
// "verification_email_failed", "http_request", ...) as attributes, alongside slog's own
// time/lvl/msg. Neither is enforced by the type system (that would need a wrapper type around
// every slog call, more machinery than two conventionally-named attributes are worth) - it's a
// convention future call sites should follow, not something this package can check for you.
package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/config"
	"studio/internal/httplog"
)

// New builds the logger described by cfg.LogLevel/LogFormat/LogDir. Console output (stdout,
// captured by journalctl under systemd - see docs/deploy.md) uses LogFormat (text by default, easy
// to read directly in `journalctl -f`). The on-disk file under LogDir, if any, always uses JSON
// regardless of LogFormat - it exists for later grep/jq analysis across a rotated history, where a
// consistent machine-parseable shape matters more than a human glancing at it live. A LogDir that
// can't be created/opened (e.g. a permissions problem) is a warning on stderr, not a fatal error -
// losing the on-disk copy shouldn't take the whole app down when journald still has everything.
func New(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel), ReplaceAttr: renameLevelKey}

	handlers := []slog.Handler{consoleHandler(cfg.LogFormat, opts)}

	if cfg.LogDir != "" {
		if fileHandler, err := openFileHandler(cfg.LogDir, opts); err != nil {
			fmt.Fprintf(os.Stderr, "logging: file logging under %s disabled: %v\n", cfg.LogDir, err)
		} else {
			handlers = append(handlers, fileHandler)
		}
	}

	var handler slog.Handler = handlers[0]
	if len(handlers) > 1 {
		handler = multiHandler{handlers}
	}

	// ContextHandler last (outermost), not any individual handler, so request_id gets attached
	// once and reaches every fanned-out destination - see its doc comment.
	return slog.New(httplog.ContextHandler{Handler: handler})
}

func consoleHandler(format string, opts *slog.HandlerOptions) slog.Handler {
	if strings.EqualFold(format, "json") {
		return slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.NewTextHandler(os.Stdout, opts)
}

// openFileHandler ensures logDir exists, opens logDir/studio.log in append mode, and arms it to
// reopen that same path on SIGHUP - what logrotate's default rename-then-create rotation needs:
// without reopening, writes after a rotation would keep going into the renamed-away (now .1)
// file's inode forever, since a plain *os.File doesn't follow its path across a rename. See
// ansible/roles/studio_app/templates/studio-logrotate.conf.j2, whose postrotate hook sends that
// signal.
func openFileHandler(logDir string, opts *slog.HandlerOptions) (slog.Handler, error) {
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	rf, err := openRotatingFile(filepath.Join(logDir, "studio.log"))
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	rf.watchForRotation()
	return slog.NewJSONHandler(rf, opts), nil
}

// renameLevelKey renames slog's default "level" attribute to "lvl" - the rest of the record
// (time, msg, and every other attribute) is already exactly what slog.NewJSONHandler produces.
func renameLevelKey(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.LevelKey {
		a.Key = "lvl"
	}
	return a
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// multiHandler fans out one log record to several handlers - a small hand-rolled version of what
// golang.org/x/exp/slog used to ship as a "multi handler" example, kept in-repo rather than taking
// a dependency on it (see the "minimal dependencies" note in CLAUDE.md).
type multiHandler struct {
	handlers []slog.Handler
}

func (m multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (m multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return multiHandler{next}
}

func (m multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return multiHandler{next}
}
