package media

import "github.com/davidbyttow/govips/v2/vips"

// InitImageProcessing starts libvips once at process startup — cmd/server calls this before
// serving any request; ShutdownImageProcessing runs on clean exit.
func InitImageProcessing() {
	vips.LoggingSettings(nil, vips.LogLevelError)
	vips.Startup(nil)
}

func ShutdownImageProcessing() {
	vips.Shutdown()
}
