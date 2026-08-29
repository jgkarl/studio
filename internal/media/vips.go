package media

import (
	"log/slog"

	"github.com/davidbyttow/govips/v2/vips"
)

// InitImageProcessing starts libvips once at process startup — cmd/server calls this before
// serving any request (and after slog.SetDefault, so vipsLog can use the app logger);
// ShutdownImageProcessing runs on clean exit.
func InitImageProcessing() {
	// Route libvips' own diagnostics (librsvg/Pango font failures during an annotated-image bake,
	// decode errors on an odd camera file, ...) through slog so they land in the structured log
	// file alongside everything else, instead of a bare line on stderr that only journald sees.
	vips.LoggingSettings(vipsLog, vips.LogLevelWarning)
	vips.Startup(nil)
}

func ShutdownImageProcessing() {
	vips.Shutdown()
}

// vipsLog forwards one libvips log message to slog, mapping GLib severities onto slog levels.
func vipsLog(domain string, level vips.LogLevel, message string) {
	attrs := []any{"category", "media", "event", "libvips", "domain", domain}
	switch level {
	case vips.LogLevelError, vips.LogLevelCritical:
		slog.Error("libvips: "+message, attrs...)
	case vips.LogLevelWarning:
		slog.Warn("libvips: "+message, attrs...)
	default:
		slog.Info("libvips: "+message, attrs...)
	}
}
