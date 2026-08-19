// Package assets is the Asset module: the physical object being conserved and its
// condition-state history.
package assets

import (
	"database/sql"
	"time"

	"studio/internal/assessments"
	"studio/internal/reporter"
	"studio/internal/treatments"
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
	AssetTypeCode         string
	AssetTypeTitle        string
	ClientName            string
	CurrentStateCondition sql.NullString
	ProjectCount          int
	ThumbnailMediaID      sql.NullString
}

type ProjectSummary struct {
	ID    string
	Title string
	Stage string
}

// ProjectCard groups one of the Asset's Projects together with that Project's own Assessments/
// Treatments/Reports — the Asset detail page's per-project section, replacing the old flat
// all-projects-mixed-together lists.
type ProjectCard struct {
	Project     ProjectSummary
	Assessments []assessments.ListRow
	Treatments  []treatments.ListRow
	Reports     []reporter.ListRow
}

// BuildProjectCards buckets the Asset's flat cross-project Assessment/Treatment/Report lists
// (each row carries a ProjectID) into one ProjectCard per Project, preserving projects' existing
// order. Rows are Asset-scoped queries so every ProjectID is expected to match one of projects;
// any that doesn't (shouldn't happen given the FK) is silently dropped rather than erroring.
func BuildProjectCards(projects []ProjectSummary, assessmentRows []assessments.ListRow, treatmentRows []treatments.ListRow, reportRows []reporter.ListRow) []ProjectCard {
	cards := make([]ProjectCard, len(projects))
	byID := make(map[string]*ProjectCard, len(projects))
	for i, p := range projects {
		cards[i] = ProjectCard{Project: p}
		byID[p.ID] = &cards[i]
	}
	for _, a := range assessmentRows {
		if c, ok := byID[a.ProjectID]; ok {
			c.Assessments = append(c.Assessments, a)
		}
	}
	for _, t := range treatmentRows {
		if c, ok := byID[t.ProjectID]; ok {
			c.Treatments = append(c.Treatments, t)
		}
	}
	for _, rep := range reportRows {
		if c, ok := byID[rep.ProjectID]; ok {
			c.Reports = append(c.Reports, rep)
		}
	}
	return cards
}
