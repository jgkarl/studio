package export

import (
	"encoding/json"
	"strings"
)

// tiptapNode mirrors just the shape reporter/outline.go writes (heading/paragraph/image nodes
// with plain text runs) — Report.Content is otherwise opaque and never decoded except here and
// in the outline builder.
type tiptapNode struct {
	Type    string       `json:"type"`
	Content []tiptapNode `json:"content"`
	Text    string       `json:"text"`
	Attrs   struct {
		Src string `json:"src"`
	} `json:"attrs"`
}

type tiptapDoc struct {
	Content []tiptapNode `json:"content"`
}

func textOf(node tiptapNode) string {
	var sb strings.Builder
	for _, c := range node.Content {
		sb.WriteString(c.Text)
	}
	return sb.String()
}

func mediaIDFromImageSrc(src string) string {
	const prefix = "/api/media/"
	if !strings.HasPrefix(src, prefix) {
		return ""
	}
	rest := src[len(prefix):]
	if i := strings.Index(rest, "?"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// flattenTiptap is a small TipTap-JSON -> Section flattener: each heading starts a new section,
// paragraphs/images accumulate into the current one. It doesn't need to round-trip arbitrary
// rich text — just enough structure for the outline Reporter produces.
func flattenTiptap(rawContent string) []Section {
	var doc tiptapDoc
	if err := json.Unmarshal([]byte(rawContent), &doc); err != nil {
		return []Section{{Heading: "Report", Paragraphs: []string{"(empty)"}}}
	}

	var sections []Section
	current := Section{Heading: "Report"}
	hasContent := func(s Section) bool { return len(s.Paragraphs) > 0 || len(s.Images) > 0 }

	for _, node := range doc.Content {
		switch node.Type {
		case "heading":
			if hasContent(current) {
				sections = append(sections, current)
			}
			current = Section{Heading: textOf(node)}
		case "paragraph":
			if text := textOf(node); text != "" {
				current.Paragraphs = append(current.Paragraphs, text)
			}
		case "image":
			if id := mediaIDFromImageSrc(node.Attrs.Src); id != "" {
				current.Images = append(current.Images, Image{MediaID: id})
			}
		}
	}
	if hasContent(current) {
		sections = append(sections, current)
	}

	if len(sections) == 0 {
		return []Section{{Heading: "Report", Paragraphs: []string{"(empty)"}}}
	}
	return sections
}
