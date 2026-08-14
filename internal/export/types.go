// Package export builds a flattened, format-agnostic document (Doc) from any of the app's
// exportable model types (Asset, Project/Workflow, Order, Report) and renders it as either a
// self-contained HTML page (images inlined as base64) or a PDF.
package export

// Image is a single embedded image or video reference within a Section.
type Image struct {
	MediaID string
	Caption string
}

// Section is one heading + body block of an exported document.
type Section struct {
	Heading    string
	Paragraphs []string
	Images     []Image
	Videos     []Image
}

// Doc is the common shape every exportable model type is flattened into before rendering.
type Doc struct {
	Title    string
	Subtitle string
	Sections []Section
}

// Type identifies which model an export document was built from.
type Type string

const (
	TypeAsset   Type = "asset"
	TypeProject Type = "project"
	TypeOrder   Type = "order"
	TypeReport  Type = "report"
)

func IsValidType(t string) bool {
	switch Type(t) {
	case TypeAsset, TypeProject, TypeOrder, TypeReport:
		return true
	default:
		return false
	}
}
