package seed

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"studio/internal/assessments"
	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
	studiodb "studio/internal/db"
	"studio/internal/media"
	"studio/internal/reporter"
	"studio/internal/treatments"
	"studio/internal/workflows"
)

//go:embed testdata/cats/*.jpg
var catImagesFS embed.FS

// catFiles lists the embedded fixture images explicitly (rather than embed.FS.ReadDir) so their
// upload order — and therefore which one lands as "before" vs "after" vs a plain reference photo
// below — is obvious from this file alone.
var catFiles = []string{"cat-1.jpg", "cat-2.jpg", "cat-3.jpg", "cat-4.jpg"}

// demoConservatorEmail is fixed on purpose: SeedDemoData checks for this row first and no-ops if
// it's already there, so re-running it on every `make run`/docker compose boot (see
// cmd/server/main.go) doesn't pile up duplicate demo rows.
const demoConservatorEmail = "conservator@studio.local"

// DemoUserPassword is the fixed, documented sign-in password for every example user SeedDemoData
// creates (currently just the conservator) — there's no dev-login picker any more, so this is
// how a developer actually gets into the seeded account. See docs/setup.md.
const DemoUserPassword = "StudioDemo123!"

// SeedDemoData creates one example row for every domain model in the schema — a second User (a
// "conservator", real password/already-verified/already-role-assigned, alongside the
// .env-bootstrapped admin), two Clients, two Assets, two Projects (Project is the mandatory
// parent for Assessments/Treatments/Reports — see db/migrations/0015_project_scoped_records.sql),
// three Assessments (condition history), a Treatment, a Report, and four Media images (from
// internal/seed/testdata/cats) wired into the media library via MediaReference, including one
// annotated MediaAnnotationRegion — so a fresh dev database has something on every screen
// immediately instead of starting empty.
//
// Not called unconditionally: see cmd/server/main.go, which only calls this when
// cfg.SeedExampleData is true — never true for a production deploy (see
// ansible/roles/studio_app/defaults/main.yml). A production boot only ever gets the one
// BootstrapAdmin account from BOOTSTRAP_ADMIN_NAME/EMAIL/PASSWORD.
//
// Idempotent: bails out immediately if demoConservatorEmail's User already exists, so it's safe
// to leave this running on every dev boot.
func SeedDemoData(ctx context.Context, pool *sql.DB, mediaSvc *media.Service) error {
	existing, err := studiodb.QueryOne(ctx, pool, "SELECT id FROM User WHERE email = ?", scanUserID, demoConservatorEmail)
	if err != nil {
		return fmt.Errorf("seed demo data: checking for existing conservator: %w", err)
	}
	if existing != nil {
		return nil
	}

	cats, err := loadCatImages()
	if err != nil {
		return fmt.Errorf("seed demo data: loading cat fixture images: %w", err)
	}

	// Same shape as BootstrapAdmin: provider "email", a real hashed password, emailVerifiedAt
	// set, role assigned up front — a fully active account from the moment it's created, not one
	// stuck in the self-registration flow's "pending admin approval" state.
	passwordHash, err := auth.HashPassword(DemoUserPassword)
	if err != nil {
		return fmt.Errorf("seed demo data: hashing conservator password: %w", err)
	}
	conservatorID := studiodb.NewID()
	if _, err := studiodb.Execute(ctx, pool,
		"INSERT INTO User (id, name, email, provider, passwordHash, emailVerifiedAt, role) VALUES (?, ?, ?, ?, ?, ?, ?)",
		conservatorID, "Conservator Example", demoConservatorEmail, "email", passwordHash, time.Now(), string(auth.RoleConservator)); err != nil {
		return fmt.Errorf("seed demo data: creating conservator user: %w", err)
	}

	clientEleanor, err := clients.Create(ctx, pool, clients.Input{
		Type:                   "individual",
		Name:                   "Eleanor Vance",
		Email:                  "eleanor.vance@example.com",
		Phone:                  "+1 555 0101",
		City:                   "Portland",
		Country:                "USA",
		PreferredContactMethod: "email",
		Notes:                  "Inherited piece from her grandmother; would like it ready before a family reunion.",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: creating client Eleanor Vance: %w", err)
	}

	clientRiverside, err := clients.Create(ctx, pool, clients.Input{
		Type:                   "institution",
		Name:                   "Riverside Historical Society",
		OrganizationName:       "Riverside Historical Society",
		ContactPerson:          "Marcus Webb",
		Email:                  "collections@riversidehistory.example",
		Phone:                  "+1 555 0199",
		City:                   "Riverside",
		Country:                "USA",
		PreferredContactMethod: "email",
		ReferralSource:         "Returning client",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: creating client Riverside Historical Society: %w", err)
	}

	assetPainting, err := assets.Create(ctx, pool, clientEleanor, "clsfr_asset_type_painting", "DEMO-0001", assets.Input{
		Title:            "Portrait of a Cat by the Window",
		Artist:           "Unknown",
		CreationPeriod:   "c. 1920",
		Medium:           "Oil on canvas",
		Dimensions:       "45 x 60 cm",
		Description:      "Small oil portrait of a resting cat, in the family for three generations.",
		Provenance:       "Vance family collection since c. 1950.",
		EstimatedValue:   1200.0,
		IsInsured:        true,
		LocationInStudio: "Shelf A2",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: creating asset (painting): %w", err)
	}

	assetSketch, err := assets.Create(ctx, pool, clientRiverside, "clsfr_asset_type_sketch", "DEMO-0002", assets.Input{
		Title:            "Feline Study (Sketch)",
		Artist:           "Attributed to a local artist, early 20th c.",
		Medium:           "Graphite on paper",
		Dimensions:       "20 x 25 cm",
		Description:      "Charcoal/graphite study of a cat, part of the Society's local-artists collection.",
		LocationInStudio: "Flat file 3, drawer B",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: creating asset (sketch): %w", err)
	}

	// Every Assessment/Treatment/Report requires a Project (its mandatory parent — see
	// db/migrations/0015_project_scoped_records.sql), so both assets get one before anything
	// else can be recorded against them.
	projectID, err := workflows.Create(ctx, pool, assetPainting, "Winter conservation cycle")
	if err != nil {
		return fmt.Errorf("seed demo data: creating project (painting): %w", err)
	}

	sketchProjectID, err := workflows.Create(ctx, pool, assetSketch, "Initial condition survey")
	if err != nil {
		return fmt.Errorf("seed demo data: creating project (sketch): %w", err)
	}

	// Intake assessment, then a re-assessment after treatment — a small condition history to
	// show on the project timeline.
	intakeAssessmentID, err := assessments.Create(ctx, pool, assessments.Input{
		ProjectID:   projectID,
		Condition:   "fair",
		Description: "Surface grime throughout; a few small paint losses in the lower-left corner.",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: recording intake assessment: %w", err)
	}

	sketchAssessmentID, err := assessments.Create(ctx, pool, assessments.Input{
		ProjectID:   sketchProjectID,
		Condition:   "good",
		Description: "Minor foxing at the edges; otherwise stable.",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: recording sketch assessment: %w", err)
	}

	if err := workflows.SetStage(ctx, pool, projectID, "working"); err != nil {
		return fmt.Errorf("seed demo data: advancing project stage: %w", err)
	}

	postTreatmentAssessmentID, err := assessments.Create(ctx, pool, assessments.Input{
		ProjectID:   projectID,
		Condition:   "good",
		Description: "Surface cleaned; losses stabilized. No active deterioration.",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: recording post-treatment assessment: %w", err)
	}

	if _, err := treatments.Create(ctx, pool, treatments.Input{
		ProjectID:         projectID,
		Method:            "surface_cleaning",
		Title:             "Initial surface cleaning",
		Notes:             "Removed surface grime with a dry sponge; tested a small area first. Losses left for a later retouching pass.",
		PerformedByUserID: conservatorID,
		PerformedAt:       time.Now().AddDate(0, 0, -10),
	}); err != nil {
		return fmt.Errorf("seed demo data: creating treatment: %w", err)
	}

	// Media library: cat-1 is the "before" intake photo (with one annotated loss region),
	// cat-2 the "after" progress photo, cat-3/cat-4 plain reference photos on each asset.
	beforePhoto, err := mediaSvc.UploadMedia(ctx, cats[0], "image/jpeg", conservatorID)
	if err != nil {
		return fmt.Errorf("seed demo data: uploading before-photo: %w", err)
	}
	if err := mediaSvc.AttachMediaReference(ctx, beforePhoto.ID, media.RefAssessment, intakeAssessmentID, "before", 0); err != nil {
		return fmt.Errorf("seed demo data: attaching before-photo: %w", err)
	}
	if _, err := media.CreateRegion(ctx, pool, beforePhoto.ID, "clsfr_annotation_type_loss", 20, 60, 15, 12); err != nil {
		return fmt.Errorf("seed demo data: annotating before-photo: %w", err)
	}

	afterPhoto, err := mediaSvc.UploadMedia(ctx, cats[1], "image/jpeg", conservatorID)
	if err != nil {
		return fmt.Errorf("seed demo data: uploading after-photo: %w", err)
	}
	if err := mediaSvc.AttachMediaReference(ctx, afterPhoto.ID, media.RefAssessment, postTreatmentAssessmentID, "after", 0); err != nil {
		return fmt.Errorf("seed demo data: attaching after-photo: %w", err)
	}

	referencePhoto1, err := mediaSvc.UploadMedia(ctx, cats[2], "image/jpeg", conservatorID)
	if err != nil {
		return fmt.Errorf("seed demo data: uploading reference photo (sketch): %w", err)
	}
	if err := mediaSvc.AttachMediaReference(ctx, referencePhoto1.ID, media.RefAssessment, sketchAssessmentID, "documentation", 0); err != nil {
		return fmt.Errorf("seed demo data: attaching reference photo (sketch): %w", err)
	}

	referencePhoto2, err := mediaSvc.UploadMedia(ctx, cats[3], "image/jpeg", conservatorID)
	if err != nil {
		return fmt.Errorf("seed demo data: uploading reference photo (painting): %w", err)
	}
	if err := mediaSvc.AttachMediaReference(ctx, referencePhoto2.ID, media.RefAsset, assetPainting, "", 1); err != nil {
		return fmt.Errorf("seed demo data: attaching reference photo (painting): %w", err)
	}

	reportID, err := reporter.Create(ctx, pool, projectID, "Condition Report - Portrait of a Cat", conservatorID, reporter.Sections{
		ConditionFindings:  "Surface grime throughout at intake, plus small paint losses lower-left. Stable structurally.",
		TreatmentPerformed: "Dry surface cleaning across the full painted surface; losses left for a later retouching campaign.",
	})
	if err != nil {
		return fmt.Errorf("seed demo data: creating report: %w", err)
	}
	if err := reporter.UpdateSections(ctx, pool, reportID, reporter.SectionsInput{
		Summary:            "Routine winter conservation cycle: surface cleaning completed, condition improved from fair to good.",
		ConditionFindings:  "Surface grime throughout at intake, plus small paint losses lower-left. Stable structurally.",
		TreatmentPerformed: "Dry surface cleaning across the full painted surface; losses left for a later retouching campaign.",
		MaterialsUsed:      "Dry cleaning sponge, soft natural-bristle brush.",
		Recommendations:    "Schedule a retouching pass for the lower-left losses; recheck in 12 months.",
	}); err != nil {
		return fmt.Errorf("seed demo data: updating report sections: %w", err)
	}
	if err := reporter.UpdateLayout(ctx, pool, reportID, reporter.LayoutInput{
		LayoutStyle: "standard", CoverMediaID: beforePhoto.ID,
		ShowCover: true, ShowSummary: true, ShowCondition: true, ShowTreatment: true, ShowMaterials: true, ShowRecommendations: true,
	}); err != nil {
		return fmt.Errorf("seed demo data: updating report layout: %w", err)
	}

	return nil
}

func loadCatImages() ([][]byte, error) {
	out := make([][]byte, len(catFiles))
	for i, name := range catFiles {
		b, err := catImagesFS.ReadFile("testdata/cats/" + name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		out[i] = b
	}
	return out, nil
}
