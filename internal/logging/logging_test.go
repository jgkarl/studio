package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":    slog.LevelDebug,
		"DEBUG":    slog.LevelDebug,
		"info":     slog.LevelInfo,
		"":         slog.LevelInfo,
		"warn":     slog.LevelWarn,
		"warning":  slog.LevelWarn,
		"error":    slog.LevelError,
		"nonsense": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRenameLevelKey(t *testing.T) {
	a := renameLevelKey(nil, slog.String(slog.LevelKey, "INFO"))
	if a.Key != "lvl" {
		t.Errorf("top-level %q attr got renamed to %q, want %q", slog.LevelKey, a.Key, "lvl")
	}

	// Inside a group, the key must be left alone - renaming it there would silently misplace it
	// relative to whatever grouped structure the caller intended.
	a = renameLevelKey([]string{"somegroup"}, slog.String(slog.LevelKey, "INFO"))
	if a.Key != slog.LevelKey {
		t.Errorf("grouped %q attr got renamed to %q, want it left alone", slog.LevelKey, a.Key)
	}

	a = renameLevelKey(nil, slog.String("msg", "hello"))
	if a.Key != "msg" {
		t.Errorf("unrelated attr key changed to %q, want left alone", a.Key)
	}
}

func TestMultiHandlerFansOutToEveryHandler(t *testing.T) {
	var bufA, bufB bytes.Buffer
	h := multiHandler{[]slog.Handler{
		slog.NewJSONHandler(&bufA, nil),
		slog.NewTextHandler(&bufB, nil),
	}}
	logger := slog.New(h)
	logger.Info("hello", "category", "test", "event", "fanout_check")

	if !strings.Contains(bufA.String(), `"msg":"hello"`) {
		t.Errorf("JSON handler did not receive the record, got: %s", bufA.String())
	}
	if !strings.Contains(bufB.String(), "msg=hello") {
		t.Errorf("text handler did not receive the record, got: %s", bufB.String())
	}
}

func TestMultiHandlerWithAttrsAppliesToAll(t *testing.T) {
	var bufA, bufB bytes.Buffer
	h := multiHandler{[]slog.Handler{
		slog.NewJSONHandler(&bufA, nil),
		slog.NewJSONHandler(&bufB, nil),
	}}
	logger := slog.New(h).With("category", "test")
	logger.Info("hello")

	for name, buf := range map[string]*bytes.Buffer{"A": &bufA, "B": &bufB} {
		var rec map[string]any
		if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
			t.Fatalf("handler %s: invalid JSON: %v (%s)", name, err, buf.String())
		}
		if rec["category"] != "test" {
			t.Errorf("handler %s: With(...) attr missing, got: %s", name, buf.String())
		}
	}
}

func TestRotatingFileReopenFollowsPathAcrossRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "studio.log")

	rf, err := openRotatingFile(path)
	if err != nil {
		t.Fatalf("openRotatingFile: %v", err)
	}
	if _, err := rf.Write([]byte("before rotation\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Simulate what logrotate's default (non-copytruncate) rotation does: rename the current file
	// aside, leaving nothing at the original path.
	rotatedPath := path + ".1"
	if err := os.Rename(path, rotatedPath); err != nil {
		t.Fatalf("simulating logrotate rename: %v", err)
	}

	if err := rf.reopen(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if _, err := rf.Write([]byte("after rotation\n")); err != nil {
		t.Fatalf("Write after reopen: %v", err)
	}

	rotated, err := os.ReadFile(rotatedPath)
	if err != nil {
		t.Fatalf("reading rotated-away file: %v", err)
	}
	if string(rotated) != "before rotation\n" {
		t.Errorf("rotated-away file = %q, want %q", rotated, "before rotation\n")
	}

	fresh, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading new file at original path: %v", err)
	}
	if string(fresh) != "after rotation\n" {
		t.Errorf("new file at original path = %q, want %q (writes after reopen must not still be going into the renamed-away inode)", fresh, "after rotation\n")
	}
}

func TestOpenFileHandlerCreatesLogDirAndWritesJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "log") // doesn't exist yet - New must create it
	handler, err := openFileHandler(dir, &slog.HandlerOptions{Level: slog.LevelInfo, ReplaceAttr: renameLevelKey})
	if err != nil {
		t.Fatalf("openFileHandler: %v", err)
	}

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "hello", "category", "test", "event", "smoke")

	data, err := os.ReadFile(filepath.Join(dir, "studio.log"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &rec); err != nil {
		t.Fatalf("log file contents are not valid JSON: %v (%s)", err, data)
	}
	for _, key := range []string{"lvl", "msg", "category", "event", "time"} {
		if _, ok := rec[key]; !ok {
			t.Errorf("log record missing %q key, got: %s", key, data)
		}
	}
	if rec["msg"] != "hello" || rec["category"] != "test" || rec["event"] != "smoke" {
		t.Errorf("unexpected record content: %s", data)
	}
}
