// Package reporter is the Reporter module: composes the final conservation write-up. The rich
// text editor is TipTap (vendored per the "vendor libraries with a real framework-agnostic
// build" decision) - loaded live via an ESM CDN (esm.sh) through a plain <script type=
// "importmap">, not bundled or downloaded into static/vendor/ like OpenSeadragon. TipTap's own
// npm packages aren't a single flat browser bundle the way OpenSeadragon's UMD build is -
// getting one without a real JS bundler (which this project deliberately has none of) isn't
// practical, so this is the honest middle ground: the genuine TipTap library, zero build step,
// at the cost of a runtime dependency on esm.sh being reachable. See static/js/report-editor.js.
package reporter

import (
	"database/sql"
	"time"
)

type Report struct {
	ID        string
	ProjectID sql.NullString
	AssetID   string
	Title     string
	Content   string // raw TipTap JSON doc, opaque - never decoded server-side except by the outline builder that constructs it
	Status    string
	AuthorID  sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListRow struct {
	ID                 string
	Title              string
	Status             string
	AssetTitle         sql.NullString
	AssetReferenceCode string
	AuthorName         sql.NullString
}

type AssetOption struct {
	ID            string
	Title         sql.NullString
	ReferenceCode string
	ClientName    string
}

type ProjectOption struct {
	ID    string
	Title string
}
