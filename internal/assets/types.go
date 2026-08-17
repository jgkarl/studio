// Package assets is the Asset module: the physical object being conserved and its
// condition-state history.
package assets

import (
	"database/sql"
	"time"
)

type Asset struct {
	ID               string
	ClientID         string
	ReferenceCode    string
	AssetTypeID      string
	Title            sql.NullString
	Artist           sql.NullString
	CreationPeriod   sql.NullString
	Dimensions       sql.NullString
	Description      sql.NullString
	Medium           sql.NullString
	SignatureMarks   sql.NullString
	Weight           sql.NullString
	Provenance       sql.NullString
	AcquisitionDate  sql.NullTime
	EstimatedValue   sql.NullFloat64
	IsInsured        sql.NullBool
	LocationInStudio sql.NullString
	CurrentStateID   sql.NullString
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (a Asset) DisplayName() string {
	if a.Title.Valid && a.Title.String != "" {
		return a.Title.String
	}
	return a.ReferenceCode
}

type ListRow struct {
	ID                    string
	Title                 sql.NullString
	ReferenceCode         string
	AssetTypeTitle        string
	ClientName            string
	CurrentStateCondition sql.NullString
	ProjectCount          int
	ThumbnailMediaID      sql.NullString
}

type State struct {
	ID          string
	Condition   string
	Description string
	RecordedAt  time.Time
}

type ProjectSummary struct {
	ID    string
	Title string
	Stage string
}
