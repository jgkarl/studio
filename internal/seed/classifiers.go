// Package seed provides idempotent structural-reference-data bootstrap — the Classifier rows
// every <select> in the app reads from, plus an optional admin user for a brand new database.
// Ported from db/seedData/classifiers.ts + db/seed-bootstrap.ts. Unlike the original's separate
// `npm run db:bootstrap` script, this runs automatically on every server start (cmd/server/
// main.go, right after migrations) — a single static binary has no separate deploy-script step,
// and re-running is free: every insert is `INSERT OR IGNORE` against the Classifier(type, code)
// unique index.
//
// This intentionally does not port db/seed.ts's ~800 lines of fictional demo clients/assets/
// workflows/orders — that's dev-only convenience content, not something a production deploy
// needs, and every entity it would create is already reachable through the app's own CRUD forms
// (exercised end-to-end via curl in the Module 11/12 smoke tests).
package seed

import (
	"context"
	"encoding/json"
	"time"

	studiodb "studio/internal/db"
)

type classifierRow struct {
	Code        string
	Title       string
	Description string
	Data        any
}

func seedClassifiers(ctx context.Context, q studiodb.Querier, classifierType string, rows []classifierRow) error {
	for i, row := range rows {
		var dataJSON any
		if row.Data != nil {
			b, err := json.Marshal(row.Data)
			if err != nil {
				return err
			}
			dataJSON = string(b)
		}
		var description any
		if row.Description != "" {
			description = row.Description
		}
		if _, err := studiodb.Execute(ctx, q,
			`INSERT OR IGNORE INTO Classifier (id, type, code, title, description, sequence, data, updatedAt)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			studiodb.NewID(), classifierType, row.Code, row.Title, description, i, dataJSON, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

// SeedAllClassifiers seeds every Classifier type the app's <select> options read from — see
// internal/settings for the admin UI that manages these at runtime. Safe to call on every boot.
func SeedAllClassifiers(ctx context.Context, q studiodb.Querier) error {
	steps := []struct {
		classifierType string
		rows           []classifierRow
	}{
		{"client_type", []classifierRow{
			{Code: "individual", Title: "Individual", Description: "A private collector or individual owner."},
			{Code: "institution", Title: "Institution", Description: "A museum, gallery, church, or other organization."},
		}},
		{"contact_method", []classifierRow{
			{Code: "email", Title: "Email"},
			{Code: "phone", Title: "Phone"},
			{Code: "mail", Title: "Postal Mail"},
			{Code: "in_person", Title: "In Person"},
		}},
		{"asset_type", []classifierRow{
			{Code: "painting", Title: "Painting"},
			{Code: "sketch", Title: "Sketch / Drawing"},
			{Code: "paper", Title: "Paper / Document"},
			{Code: "sculpture", Title: "Sculpture"},
			{Code: "textile", Title: "Textile"},
			{Code: "ceramic", Title: "Ceramic"},
			{Code: "photograph", Title: "Photograph"},
			{Code: "furniture", Title: "Furniture"},
			{Code: "book_manuscript", Title: "Book / Manuscript"},
			{Code: "metalwork", Title: "Metalwork / Objet d'Art"},
			{Code: "mixed_media", Title: "Mixed Media"},
			{Code: "other", Title: "Other"},
		}},
		{"material", []classifierRow{
			{Code: "canvas", Title: "Canvas"},
			{Code: "linen", Title: "Linen"},
			{Code: "cotton_duck", Title: "Cotton Duck"},
			{Code: "oil_paint", Title: "Oil Paint"},
			{Code: "acrylic_paint", Title: "Acrylic Paint"},
			{Code: "tempera", Title: "Tempera"},
			{Code: "gesso", Title: "Gesso"},
			{Code: "varnish", Title: "Varnish"},
			{Code: "wood_panel", Title: "Wood Panel"},
			{Code: "plywood", Title: "Plywood"},
			{Code: "mdf", Title: "MDF"},
			{Code: "watercolor_paper", Title: "Watercolor Paper"},
			{Code: "rag_paper", Title: "Rag Paper"},
			{Code: "newsprint", Title: "Newsprint"},
			{Code: "parchment", Title: "Parchment"},
			{Code: "vellum", Title: "Vellum"},
			{Code: "graphite", Title: "Graphite"},
			{Code: "charcoal", Title: "Charcoal"},
			{Code: "ink", Title: "Ink"},
			{Code: "pastel", Title: "Pastel"},
			{Code: "gouache", Title: "Gouache"},
			{Code: "bronze", Title: "Bronze"},
			{Code: "marble", Title: "Marble"},
			{Code: "stone", Title: "Stone"},
			{Code: "terracotta", Title: "Terracotta / Clay"},
			{Code: "plaster", Title: "Plaster"},
			{Code: "wax", Title: "Wax"},
			{Code: "ivory", Title: "Ivory"},
			{Code: "bone", Title: "Bone"},
			{Code: "iron_steel", Title: "Iron / Steel"},
			{Code: "silver", Title: "Silver"},
			{Code: "gold_leaf", Title: "Gold Leaf"},
			{Code: "silk", Title: "Silk"},
			{Code: "wool", Title: "Wool"},
			{Code: "leather", Title: "Leather"},
			{Code: "glass", Title: "Glass"},
			{Code: "ceramic_glaze", Title: "Ceramic Glaze"},
			{Code: "synthetic_resin", Title: "Synthetic Resin / Plastic"},
			{Code: "photographic_paper", Title: "Photographic Paper"},
			{Code: "adhesive_residue", Title: "Adhesive Residue"},
			{Code: "lacquer", Title: "Lacquer"},
			{Code: "shellac", Title: "Shellac"},
		}},
		{"condition_state", []classifierRow{
			{Code: "intake", Title: "Intake", Description: "As received, not yet assessed.", Data: map[string]any{"severity": nil}},
			{Code: "excellent", Title: "Excellent", Description: "No visible deterioration; original materials fully intact.", Data: map[string]any{"severity": 1}},
			{Code: "good", Title: "Good", Description: "Minor, stable signs of age; no active deterioration.", Data: map[string]any{"severity": 1}},
			{Code: "fair", Title: "Fair", Description: "Noticeable wear or soiling; stable but would benefit from treatment.", Data: map[string]any{"severity": 2}},
			{Code: "stable", Title: "Stable", Description: "Condition is not actively changing.", Data: map[string]any{"severity": 2}},
			{Code: "soiled", Title: "Soiled / Dirty", Description: "Surface grime or dust accumulation.", Data: map[string]any{"severity": 2}},
			{Code: "discolored", Title: "Discolored / Yellowed", Description: "Color shift from aging, varnish, or light exposure.", Data: map[string]any{"severity": 2}},
			{Code: "faded", Title: "Faded", Description: "Loss of color saturation, typically from light exposure.", Data: map[string]any{"severity": 2}},
			{Code: "deteriorating", Title: "Deteriorating (Active)", Description: "Condition is actively worsening; treatment priority.", Data: map[string]any{"severity": 4}},
			{Code: "degraded", Title: "Degraded", Description: "General material breakdown beyond normal aging.", Data: map[string]any{"severity": 3}},
			{Code: "flaking", Title: "Flaking / Losses", Description: "Paint or surface layer lifting or missing in areas.", Data: map[string]any{"severity": 4}},
			{Code: "brittle", Title: "Brittle", Description: "Material has lost flexibility and is prone to cracking.", Data: map[string]any{"severity": 3}},
			{Code: "warped", Title: "Warped / Distorted", Description: "Physical deformation, often from humidity or heat.", Data: map[string]any{"severity": 3}},
			{Code: "water_damaged", Title: "Water Damaged", Description: "Staining, tide lines, or structural weakening from moisture.", Data: map[string]any{"severity": 4}},
			{Code: "mold", Title: "Mold / Biological Growth", Description: "Active or historical fungal/biological contamination.", Data: map[string]any{"severity": 5}},
			{Code: "insect_damage", Title: "Insect Damage", Description: "Boring, frass, or larval damage from pests.", Data: map[string]any{"severity": 4}},
			{Code: "ripped", Title: "Ripped / Torn", Description: "Tear or puncture in the support or surface.", Data: map[string]any{"severity": 4}},
			{Code: "broken", Title: "Broken / Fractured", Description: "Structural break, typically in rigid materials.", Data: map[string]any{"severity": 4}},
			{Code: "poor", Title: "Poor", Description: "Extensive damage across multiple areas; treatment needed soon.", Data: map[string]any{"severity": 4}},
			{Code: "critical", Title: "Critical / Severely Deteriorated", Description: "At risk of further loss without immediate intervention.", Data: map[string]any{"severity": 5}},
			{Code: "in_treatment", Title: "In Treatment", Description: "Actively undergoing conservation work.", Data: map[string]any{"severity": nil}},
			{Code: "consolidated", Title: "Consolidated", Description: "Loose or flaking material has been stabilized.", Data: map[string]any{"severity": 2}},
			{Code: "restored", Title: "Restored", Description: "Treatment complete; losses addressed and surface stabilized.", Data: map[string]any{"severity": 1}},
			{Code: "fully_conserved", Title: "Fully Conserved", Description: "Comprehensive treatment complete; excellent stable condition.", Data: map[string]any{"severity": 1}},
			{Code: "other", Title: "Other", Description: "Condition not covered by the standard list — see notes.", Data: map[string]any{"severity": nil}},
		}},
		{"activity_type", []classifierRow{
			{Code: "ingest", Title: "Ingest", Data: map[string]any{"defaultRate": 40}},
			{Code: "condition_assessment", Title: "Condition Assessment", Data: map[string]any{"defaultRate": 60, "isStateFixation": true}},
			{Code: "documentation", Title: "Documentation / Photography", Data: map[string]any{"defaultRate": 35}},
			{Code: "surface_cleaning", Title: "Surface Cleaning", Data: map[string]any{"defaultRate": 65}},
			{Code: "consolidation", Title: "Consolidation", Data: map[string]any{"defaultRate": 70}},
			{Code: "structural_repair", Title: "Structural Repair", Data: map[string]any{"defaultRate": 80}},
			{Code: "deacidification", Title: "Deacidification", Data: map[string]any{"defaultRate": 55}},
			{Code: "inpainting", Title: "Inpainting / Retouching", Data: map[string]any{"defaultRate": 75}},
			{Code: "varnishing", Title: "Varnishing", Data: map[string]any{"defaultRate": 65}},
			{Code: "drying_curing", Title: "Drying / Curing", Data: map[string]any{"defaultRate": 0, "isWaitingPeriod": true}},
			{Code: "reassessment", Title: "Re-assessment", Data: map[string]any{"defaultRate": 60, "isStateFixation": true}},
			{Code: "handover", Title: "Handover", Data: map[string]any{"defaultRate": 30}},
		}},
		{"order_status", []classifierRow{
			{Code: "inquiry", Title: "Inquiry"},
			{Code: "in_queue", Title: "In Queue"},
			{Code: "in_progress", Title: "In Progress"},
			{Code: "completed", Title: "Completed"},
			{Code: "archived", Title: "Archived"},
		}},
		{"quote_status", []classifierRow{
			{Code: "draft", Title: "Draft"},
			{Code: "sent", Title: "Sent"},
			{Code: "accepted", Title: "Accepted"},
			{Code: "declined", Title: "Declined"},
			{Code: "expired", Title: "Expired"},
		}},
		{"invoice_status", []classifierRow{
			{Code: "draft", Title: "Draft"},
			{Code: "sent", Title: "Sent"},
			{Code: "paid", Title: "Paid"},
			{Code: "overdue", Title: "Overdue"},
			{Code: "void", Title: "Void"},
		}},
	}

	for _, step := range steps {
		if err := seedClassifiers(ctx, q, step.classifierType, step.rows); err != nil {
			return err
		}
	}
	return nil
}
