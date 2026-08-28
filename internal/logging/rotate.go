package logging

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const rotatingFilePerm = 0o640

// rotatingFile is an io.Writer over an append-mode *os.File that can reopen its own path on
// demand - see watchForRotation. Safe for concurrent Write calls from multiple goroutines (every
// HTTP request logs concurrently), guarded by mu.
type rotatingFile struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

func openRotatingFile(path string) (*rotatingFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, rotatingFilePerm)
	if err != nil {
		return nil, err
	}
	return &rotatingFile{path: path, f: f}, nil
}

func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Write(p)
}

// reopen closes the current file handle and opens path fresh - after logrotate renames the old
// file aside and this creates a new empty one in its place, this is what makes subsequent writes
// land in the new file instead of the (now differently-named) old one.
func (w *rotatingFile) reopen() error {
	nf, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, rotatingFilePerm)
	if err != nil {
		return err
	}
	w.mu.Lock()
	old := w.f
	w.f = nf
	w.mu.Unlock()
	return old.Close()
}

// watchForRotation reopens the log file whenever the process receives SIGHUP - point logrotate's
// postrotate hook at `systemctl kill -s SIGHUP studio.service` (see
// ansible/roles/studio_app/templates/studio-logrotate.conf.j2) so a rotation doesn't silently
// leave every future write going into the file logrotate just renamed away. Runs for the lifetime
// of the process; there's no corresponding Stop since the app has no graceful-shutdown path to
// call it from (see cmd/server/main.go) and an unstopped signal.Notify channel is harmless at
// process exit either way.
func (w *rotatingFile) watchForRotation() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			if err := w.reopen(); err != nil {
				slog.Error("reopening log file after SIGHUP", "err", err, "category", "logging", "event", "log_reopen_failed")
			}
		}
	}()
}
