package export

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"time"
)

func (svc *Service) imageTag(ctx context.Context, img Image) string {
	data, mimeType, err := svc.exportImageBytes(ctx, img.MediaID)
	if err != nil || data == nil {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	captionHTML := ""
	if img.Caption != "" {
		captionHTML = `<p style="font-size:0.85rem;color:#57534e;margin:4px 0 0;">` + html.EscapeString(img.Caption) + `</p>`
	}
	return fmt.Sprintf(`<div style="margin:8px 0;">
    <img src="data:%s;base64,%s" alt="%s" style="max-width:100%%;border-radius:6px;display:block;" />
    %s
    <a href="/media/view/%s" target="_blank" rel="noreferrer" style="font-size:0.8rem;color:#78716c;">🔍 View full resolution</a>
  </div>`, mimeType, b64, html.EscapeString(img.Caption), captionHTML, img.MediaID)
}

func videoTag(img Image) string {
	return fmt.Sprintf(`<video controls style="max-width:100%%;border-radius:6px;margin:8px 0;" src="/api/media/%s?variant=original"></video>`, img.MediaID)
}

// RenderHTML produces a self-contained HTML page — images are inlined as base64 so the export
// stays viewable even with the app server unreachable; video stays a live <video src> (fully
// offline/zipped export is a possible later upgrade, not attempted here).
func (svc *Service) RenderHTML(ctx context.Context, doc *Doc) string {
	var sectionsHTML strings.Builder
	for _, section := range doc.Sections {
		sectionsHTML.WriteString("\n<section>\n<h2>")
		sectionsHTML.WriteString(html.EscapeString(section.Heading))
		sectionsHTML.WriteString("</h2>\n")
		for _, p := range section.Paragraphs {
			sectionsHTML.WriteString("<p>")
			sectionsHTML.WriteString(html.EscapeString(p))
			sectionsHTML.WriteString("</p>\n")
		}
		for _, img := range section.Images {
			sectionsHTML.WriteString(svc.imageTag(ctx, img))
			sectionsHTML.WriteString("\n")
		}
		for _, vid := range section.Videos {
			sectionsHTML.WriteString(videoTag(vid))
			sectionsHTML.WriteString("\n")
		}
		sectionsHTML.WriteString("</section>")
	}

	subtitleHTML := ""
	if doc.Subtitle != "" {
		subtitleHTML = `<div class="subtitle">` + html.EscapeString(doc.Subtitle) + `</div>`
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>%s</title>
<style>
  body { font-family: -apple-system, Segoe UI, Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; color: #1c1917; }
  h1 { font-size: 1.75rem; margin-bottom: 0; }
  .subtitle { color: #78716c; margin-top: 4px; }
  h2 { font-size: 1.1rem; border-bottom: 1px solid #e7e5e4; padding-bottom: 4px; margin-top: 2rem; }
  p { line-height: 1.6; }
  footer { margin-top: 3rem; color: #a8a29e; font-size: 0.75rem; }
</style>
</head>
<body>
  <h1>%s</h1>
  %s
  %s
  <footer>Exported from Studio on %s</footer>
</body>
</html>`, html.EscapeString(doc.Title), html.EscapeString(doc.Title), subtitleHTML, sectionsHTML.String(), time.Now().Format("2006-01-02 15:04"))
}
