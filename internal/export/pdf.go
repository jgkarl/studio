package export

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	pdfMargin   = 12.0
	pdfPageW    = 210.0 // A4 width, mm
	pdfImgMaxW  = 85.0
	pdfPageBotY = 270.0 // below this, an image won't fit before the footer margin
)

// RenderPDF renders a Doc as a single-page-flow A4 PDF using only the core Helvetica font (no
// embedded TTF) — a deliberate v1 choice mirroring the original's plain react-pdf/Helvetica
// output. UTF-8 text is translated to cp1252 on a best-effort basis via
// UnicodeTranslatorFromDescriptor; characters outside that code page are dropped rather than
// corrupting the document.
func (svc *Service) RenderPDF(ctx context.Context, doc *Doc) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 20)
	pdf.MultiCell(0, 8, tr(doc.Title), "", "L", false)
	if doc.Subtitle != "" {
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 113, 108)
		pdf.MultiCell(0, 6, tr(doc.Subtitle), "", "L", false)
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.Ln(4)

	imgCounter := 0
	for _, section := range doc.Sections {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.MultiCell(0, 7, tr(section.Heading), "", "L", false)
		y := pdf.GetY()
		pdf.SetDrawColor(231, 229, 228)
		pdf.Line(pdfMargin, y, pdfPageW-pdfMargin, y)
		pdf.Ln(3)

		pdf.SetFont("Helvetica", "", 10)
		for _, p := range section.Paragraphs {
			pdf.MultiCell(0, 5, tr(p), "", "L", false)
			pdf.Ln(1)
		}

		for _, img := range section.Images {
			file, err := svc.Media.ReadMediaFile(ctx, img.MediaID, "web")
			if err != nil || file == nil {
				continue
			}
			imgCounter++
			name := fmt.Sprintf("export-img-%d", imgCounter)
			info := pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "JPG"}, bytes.NewReader(file.Data))
			if info == nil || info.Width() == 0 {
				continue
			}
			h := pdfImgMaxW * info.Height() / info.Width()
			if pdf.GetY()+h > pdfPageBotY {
				pdf.AddPage()
			}
			pdf.ImageOptions(name, pdfMargin, pdf.GetY(), pdfImgMaxW, h, false, fpdf.ImageOptions{ImageType: "JPG"}, 0, "")
			pdf.Ln(h + 3)
		}

		if len(section.Videos) > 0 {
			pdf.SetFont("Helvetica", "I", 9)
			pdf.SetTextColor(168, 162, 158)
			note := fmt.Sprintf("%d video(s) attached — available in the HTML export, not embeddable in PDF.", len(section.Videos))
			pdf.MultiCell(0, 5, tr(note), "", "L", false)
			pdf.SetTextColor(0, 0, 0)
		}
		pdf.Ln(3)
	}

	pdf.SetAutoPageBreak(false, 0)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(168, 162, 158)
	pdf.SetY(-15)
	pdf.CellFormat(0, 8, tr("Exported from Stuudio on "+time.Now().Format("2006-01-02 15:04")), "", 0, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
