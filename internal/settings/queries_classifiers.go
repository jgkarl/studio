package settings

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	studiodb "studio/internal/db"
	"studio/internal/i18n"
)

func scanClassifier(rows *sql.Rows) (Classifier, error) {
	var c Classifier
	err := rows.Scan(&c.ID, &c.Type, &c.Code, &c.Sequence, &c.Title, &c.TitleEt, &c.Description, &c.DescriptionEt,
		&c.Data, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

const classifierColumns = "id, type, code, sequence, title, titleEt, description, descriptionEt, data, isActive, createdAt, updatedAt"

// GetClassifiers returns active classifiers of a type, in display order — what every <select> in
// the app uses.
func GetClassifiers(ctx context.Context, q studiodb.Querier, t ClassifierType) ([]Classifier, error) {
	return studiodb.Query(ctx, q, "SELECT "+classifierColumns+" FROM Classifier WHERE type = ? AND isActive = 1 ORDER BY sequence ASC", scanClassifier, t)
}

// GetAllClassifiers returns every classifier of a type (active + inactive), in display order —
// the admin management table.
func GetAllClassifiers(ctx context.Context, q studiodb.Querier, t ClassifierType) ([]Classifier, error) {
	return studiodb.Query(ctx, q, "SELECT "+classifierColumns+" FROM Classifier WHERE type = ? ORDER BY sequence ASC", scanClassifier, t)
}

func GetClassifierByID(ctx context.Context, q studiodb.Querier, id string) (*Classifier, error) {
	return studiodb.QueryOne(ctx, q, "SELECT "+classifierColumns+" FROM Classifier WHERE id = ?", scanClassifier, id)
}

// GetClassifierLabel resolves a code to its display title (locale-resolved via ClassifierLabel),
// falling back to the code itself if the classifier row is missing.
func GetClassifierLabel(ctx context.Context, q studiodb.Querier, t ClassifierType, code string, locale i18n.Locale) (string, error) {
	row, err := studiodb.QueryOne(ctx, q, "SELECT "+classifierColumns+" FROM Classifier WHERE type = ? AND code = ?", scanClassifier, t, code)
	if err != nil || row == nil {
		return code, err
	}
	return ClassifierLabel(*row, locale), nil
}

// GetClassifierLabelMap builds a {code: title} map for a type, locale-resolved — handy for label
// lookups without N+1 queries (e.g. rendering a list of Treatments, each with its own method code).
func GetClassifierLabelMap(ctx context.Context, q studiodb.Querier, t ClassifierType, locale i18n.Locale) (map[string]string, error) {
	rows, err := studiodb.Query(ctx, q, "SELECT "+classifierColumns+" FROM Classifier WHERE type = ?", scanClassifier, t)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.Code] = ClassifierLabel(row, locale)
	}
	return m, nil
}

func CountClassifiers(ctx context.Context, q studiodb.Querier, types []ClassifierType) (int, error) {
	if len(types) == 0 {
		return 0, nil
	}
	placeholders := ""
	args := make([]any, len(types))
	for i, t := range types {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = t
	}
	row, err := studiodb.QueryOne(ctx, q, "SELECT COUNT(*) AS n FROM Classifier WHERE type IN ("+placeholders+")", scanCount, args...)
	if err != nil || row == nil {
		return 0, err
	}
	return *row, nil
}

func scanCount(rows *sql.Rows) (int, error) {
	var n int
	err := rows.Scan(&n)
	return n, err
}

type ClassifierInput struct {
	Type          ClassifierType
	Code          string
	Title         string
	TitleEt       string
	Description   string
	DescriptionEt string
	Sequence      int
	IsActive      bool
	Data          string // raw JSON text, already validated by the caller; "" means NULL
}

func CreateClassifier(ctx context.Context, q studiodb.Querier, in ClassifierInput) (string, error) {
	id := studiodb.NewID()
	_, err := studiodb.Execute(ctx, q,
		"INSERT INTO Classifier (id, type, code, title, titleEt, description, descriptionEt, sequence, data, updatedAt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, in.Type, in.Code, in.Title, nullIfEmpty(in.TitleEt), nullIfEmpty(in.Description), nullIfEmpty(in.DescriptionEt),
		in.Sequence, nullIfEmpty(in.Data), time.Now(),
	)
	return id, err
}

// UpdateClassifier saves title/titleEt/sequence/data for an existing classifier - used by the
// Settings -> Classifiers inline edit (every managed type gets title/sequence; Annotation Types
// additionally get the color/hatch pattern, see AnnotationTypeData) so those are actually
// manageable through the UI instead of only settable at creation/seed time. sequence is what every
// <select> built from GetClassifiers/GetAllClassifiers orders by - editing it is how an admin
// reorders a picklist. Code/type/isActive aren't touched here; there's no UI need to change those
// yet.
func UpdateClassifier(ctx context.Context, q studiodb.Querier, id, title, titleEt string, sequence int, data string) error {
	_, err := studiodb.Execute(ctx, q,
		"UPDATE Classifier SET title = ?, titleEt = ?, sequence = ?, data = ?, updatedAt = ? WHERE id = ?",
		title, nullIfEmpty(titleEt), sequence, nullIfEmpty(data), time.Now(), id)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// classifierUsageChecks: types backed by a real foreign key elsewhere in the schema — deleting a
// row still in use would orphan those references, so it's blocked in favor of deactivating.
var classifierUsageChecks = map[ClassifierType]string{
	ClassifierAssetType:    "SELECT COUNT(*) AS n FROM Asset WHERE assetTypeId = ?",
	ClassifierActivityType: "SELECT COUNT(*) AS n FROM Activity WHERE activityTypeId = ?",
}

func DeleteClassifier(ctx context.Context, q studiodb.Querier, id string) error {
	row, err := GetClassifierByID(ctx, q, id)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("classifier %s not found", id)
	}
	if checkSQL, ok := classifierUsageChecks[row.Type]; ok {
		count, err := studiodb.QueryOne(ctx, q, checkSQL, scanCount, id)
		if err != nil {
			return err
		}
		if count != nil && *count > 0 {
			return fmt.Errorf("%q is still used by %d record(s) — deactivate it instead of deleting", row.Title, *count)
		}
	}
	_, err = studiodb.Execute(ctx, q, "DELETE FROM Classifier WHERE id = ?", id)
	return err
}
