package assets

import (
	"testing"

	"studio/internal/assessments"
	"studio/internal/reporter"
	"studio/internal/treatments"
)

func TestBuildProjectCardsBucketsByProject(t *testing.T) {
	projects := []ProjectSummary{
		{ID: "p1", Title: "Project One"},
		{ID: "p2", Title: "Project Two"},
	}
	assessmentRows := []assessments.ListRow{
		{ID: "a1", ProjectID: "p1"},
		{ID: "a2", ProjectID: "p2"},
		{ID: "a3", ProjectID: "p1"},
	}
	treatmentRows := []treatments.ListRow{
		{ID: "t1", ProjectID: "p2"},
	}
	reportRows := []reporter.ListRow{
		{ID: "r1", ProjectID: "p1"},
	}

	cards := BuildProjectCards(projects, assessmentRows, treatmentRows, reportRows)

	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2 (one per project, in project order)", len(cards))
	}
	if cards[0].Project.ID != "p1" || cards[1].Project.ID != "p2" {
		t.Fatalf("cards not in project order: got %s, %s", cards[0].Project.ID, cards[1].Project.ID)
	}
	if len(cards[0].Assessments) != 2 {
		t.Fatalf("p1 got %d assessments, want 2", len(cards[0].Assessments))
	}
	if len(cards[0].Treatments) != 0 {
		t.Fatalf("p1 got %d treatments, want 0", len(cards[0].Treatments))
	}
	if len(cards[0].Reports) != 1 {
		t.Fatalf("p1 got %d reports, want 1", len(cards[0].Reports))
	}
	if len(cards[1].Assessments) != 1 || len(cards[1].Treatments) != 1 {
		t.Fatalf("p2 got %d assessments / %d treatments, want 1/1", len(cards[1].Assessments), len(cards[1].Treatments))
	}
}

// A row whose ProjectID doesn't match any of the Asset's own projects (shouldn't happen given the
// FK, per BuildProjectCards' own doc comment) must be dropped silently, not panic or corrupt
// another project's bucket.
func TestBuildProjectCardsDropsUnmatchedRows(t *testing.T) {
	projects := []ProjectSummary{{ID: "p1", Title: "Only Project"}}
	assessmentRows := []assessments.ListRow{{ID: "a1", ProjectID: "does-not-exist"}}

	cards := BuildProjectCards(projects, assessmentRows, nil, nil)

	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if len(cards[0].Assessments) != 0 {
		t.Fatalf("got %d assessments bucketed onto the only project, want 0 (unmatched row should be dropped)", len(cards[0].Assessments))
	}
}

func TestBuildProjectCardsNoProjects(t *testing.T) {
	cards := BuildProjectCards(nil, nil, nil, nil)
	if len(cards) != 0 {
		t.Fatalf("got %d cards for zero projects, want 0", len(cards))
	}
}
