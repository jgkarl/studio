// Package static embeds the app's CSS/JS/icon/font assets into the compiled binary — a deployed
// release doesn't need static/ shipped alongside it (see cmd/server/main.go).
package static

import "embed"

//go:embed css js icons fonts
var Files embed.FS
