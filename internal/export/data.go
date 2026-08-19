package export

import (
	"context"
	"database/sql"
	"fmt"

	"studio/internal/assessments"
	"studio/internal/assets"
	"studio/internal/clients"
	"studio/internal/i18n"
	"studio/internal/media"
	"studio/internal/reporter"
	"studio/internal/settings"
	"studio/internal/workflows"
)

// exportLocale is fixed: exported HTML/PDF documents are plain, non-localized output regardless
// of the viewer's own locale preference (no *http.Request reaches this package to read a cookie
// from) — same scope decision as the rest of this package's hardcoded English strings.
const exportLocale = i18n.LocaleEN

type Service struct {
	Pool  *sql.DB
	Media *media.Service
}

func stageLabel(labels map[string]string, stage string) string {
	if label, ok := labels[stage]; ok {
		return label
	}
	return stage
}

func reversed[T any](in []T) []T {
	out := make([]T, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

func (svc *Service) mediaFor(ctx context.Context, refType media.ReferencingType, refID string) (images, videos []Image) {
	refs, err := svc.Media.GetReferencedMedia(ctx, refType, refID)
	if err != nil {
		return nil, nil
	}
	for _, ref := range refs {
		switch ref.Media.Kind {
		case media.KindImage:
			images = append(images, Image{MediaID: ref.MediaID})
		case media.KindVideo:
			videos = append(videos, Image{MediaID: ref.MediaID})
		}
	}
	return images, videos
}

func (svc *Service) GetAssetExportData(ctx context.Context, id string) (*Doc, error) {
	asset, err := assets.GetByID(ctx, svc.Pool, id)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("asset %s not found", id)
	}
	client, _ := clients.GetByID(ctx, svc.Pool, asset.ClientID)
	assetType, _ := settings.GetClassifierByID(ctx, svc.Pool, asset.AssetTypeID)
	states, _ := assessments.ListByAsset(ctx, svc.Pool, id) // DESC — reversed below for chronological order
	states = reversed(states)
	projects, _ := assets.ListProjectsForAsset(ctx, svc.Pool, id)
	conditionLabels, _ := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierConditionState, exportLocale)
	stageLabels, _ := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierProjectStage, exportLocale)

	details := []string{}
	if assetType != nil {
		details = append(details, "Type: "+assetType.Title)
	}
	if asset.Artist.Valid && asset.Artist.String != "" {
		details = append(details, "Artist: "+asset.Artist.String)
	}
	if asset.CreationPeriod.Valid && asset.CreationPeriod.String != "" {
		details = append(details, "Creation period: "+asset.CreationPeriod.String)
	}
	if asset.Dimensions.Valid && asset.Dimensions.String != "" {
		details = append(details, "Dimensions: "+asset.Dimensions.String)
	}
	if asset.Medium.Valid && asset.Medium.String != "" {
		details = append(details, "Medium: "+asset.Medium.String)
	}
	if client != nil {
		details = append(details, "Owner: "+client.Name)
	}
	if asset.Description.Valid && asset.Description.String != "" {
		details = append(details, asset.Description.String)
	}

	sections := []Section{{Heading: "Details", Paragraphs: details}}

	for _, state := range states {
		label := conditionLabels[state.Condition]
		if label == "" {
			label = state.Condition
		}
		images, videos := svc.mediaFor(ctx, media.RefAssessment, state.ID)
		sections = append(sections, Section{
			Heading:    fmt.Sprintf("Condition: %s — %s", label, state.RecordedAt.Format("2006-01-02")),
			Paragraphs: []string{state.Description},
			Images:     images,
			Videos:     videos,
		})
	}

	if len(projects) > 0 {
		var paragraphs []string
		for _, p := range projects {
			paragraphs = append(paragraphs, fmt.Sprintf("%s — stage: %s", p.Title, stageLabel(stageLabels, p.Stage)))
		}
		sections = append(sections, Section{Heading: "Projects", Paragraphs: paragraphs})
	}

	return &Doc{Title: asset.DisplayName(), Subtitle: asset.ReferenceCode, Sections: sections}, nil
}

func (svc *Service) GetProjectExportData(ctx context.Context, id string) (*Doc, error) {
	project, err := workflows.GetByID(ctx, svc.Pool, id)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project %s not found", id)
	}
	asset, err := assets.GetByID(ctx, svc.Pool, project.AssetID)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, fmt.Errorf("asset %s not found", project.AssetID)
	}
	client, _ := clients.GetByID(ctx, svc.Pool, asset.ClientID)
	activities, _ := workflows.ListActivities(ctx, svc.Pool, id) // DESC — reversed below
	activities = reversed(activities)
	stageLabels, _ := settings.GetClassifierLabelMap(ctx, svc.Pool, settings.ClassifierProjectStage, exportLocale)

	clientName := ""
	if client != nil {
		clientName = client.Name
	}
	sections := []Section{{
		Heading: "Overview",
		Paragraphs: []string{
			"Asset: " + asset.DisplayName(),
			"Owner: " + clientName,
			"Stage: " + stageLabel(stageLabels, project.Stage),
		},
	}}

	for _, activity := range activities {
		images, videos := svc.mediaFor(ctx, media.RefActivity, activity.ID)
		sections = append(sections, Section{
			Heading:    fmt.Sprintf("%s — %s", activity.ActivityTypeTitle, activity.StartedAt.Format("2006-01-02 15:04")),
			Paragraphs: []string{activity.Description, "Logged by " + activity.UserName},
			Images:     images,
			Videos:     videos,
		})
	}

	return &Doc{Title: project.Title, Subtitle: asset.DisplayName(), Sections: sections}, nil
}

// GetReportExportData renders a Report's five structured sections, skipping any the "Customize
// layout" sidebar has hidden (report.Show*) and, for the cover, rendering it as its own
// image-only section first if ShowCover and CoverMediaID are set.
func (svc *Service) GetReportExportData(ctx context.Context, id string) (*Doc, error) {
	report, err := reporter.GetByID(ctx, svc.Pool, id)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, fmt.Errorf("report %s not found", id)
	}
	asset, _ := assets.GetByID(ctx, svc.Pool, report.AssetID)

	var sections []Section
	if report.ShowCover && report.CoverMediaID.Valid {
		sections = append(sections, Section{Heading: "Cover", Images: []Image{{MediaID: report.CoverMediaID.String}}})
	}
	if report.ShowSummary && report.Summary.Valid && report.Summary.String != "" {
		sections = append(sections, Section{Heading: "Summary", Paragraphs: []string{report.Summary.String}})
	}
	if report.ShowCondition && report.ConditionFindings.Valid && report.ConditionFindings.String != "" {
		sections = append(sections, Section{Heading: "Condition findings", Paragraphs: []string{report.ConditionFindings.String}})
	}
	if report.ShowTreatment && report.TreatmentPerformed.Valid && report.TreatmentPerformed.String != "" {
		sections = append(sections, Section{Heading: "Treatment performed", Paragraphs: []string{report.TreatmentPerformed.String}})
	}
	if report.ShowMaterials && report.MaterialsUsed.Valid && report.MaterialsUsed.String != "" {
		sections = append(sections, Section{Heading: "Materials used", Paragraphs: []string{report.MaterialsUsed.String}})
	}
	if report.ShowRecommendations && report.Recommendations.Valid && report.Recommendations.String != "" {
		sections = append(sections, Section{Heading: "Recommendations", Paragraphs: []string{report.Recommendations.String}})
	}

	images, videos := svc.mediaFor(ctx, media.RefReport, report.ID)
	if len(images) > 0 || len(videos) > 0 {
		sections = append(sections, Section{Heading: "Attachments", Images: images, Videos: videos})
	}
	if len(sections) == 0 {
		sections = []Section{{Heading: "Report", Paragraphs: []string{"(empty)"}}}
	}

	subtitle := ""
	if asset != nil {
		subtitle = asset.DisplayName()
	}
	return &Doc{Title: report.Title, Subtitle: subtitle, Sections: sections}, nil
}
