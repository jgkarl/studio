package settings

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	studiodb "studio/internal/db"
)

func scanTag(rows *sql.Rows) (Tag, error) {
	var t Tag
	err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Sequence)
	return t, err
}

const tagColumns = "id, name, category, sequence"

func getOrCreateTagByName(ctx context.Context, q studiodb.Querier, name string) (*Tag, error) {
	existing, err := studiodb.QueryOne(ctx, q, "SELECT "+tagColumns+" FROM Tag WHERE name = ?", scanTag, name)
	if err != nil || existing != nil {
		return existing, err
	}
	id := studiodb.NewID()
	if _, err := studiodb.Execute(ctx, q, "INSERT INTO Tag (id, name) VALUES (?, ?)", id, name); err != nil {
		return nil, err
	}
	return studiodb.QueryOne(ctx, q, "SELECT "+tagColumns+" FROM Tag WHERE id = ?", scanTag, id)
}

// SetTags parses a comma-separated tag input, creating any Tag rows that don't exist yet, and
// replaces every TagAssignment for (taggableType, taggableId) with the parsed set.
func SetTags(ctx context.Context, pool *sql.DB, taggableType, taggableID, rawInput string) error {
	var names []string
	for _, s := range strings.Split(rawInput, ",") {
		if s = strings.TrimSpace(s); s != "" {
			names = append(names, s)
		}
	}

	_, err := studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (struct{}, error) {
		if _, err := studiodb.Execute(ctx, tx, "DELETE FROM TagAssignment WHERE taggableType = ? AND taggableId = ?", taggableType, taggableID); err != nil {
			return struct{}{}, err
		}
		for _, name := range names {
			tag, err := getOrCreateTagByName(ctx, tx, name)
			if err != nil {
				return struct{}{}, err
			}
			if _, err := studiodb.Execute(ctx, tx,
				"INSERT IGNORE INTO TagAssignment (id, tagId, taggableType, taggableId) VALUES (?, ?, ?, ?)",
				studiodb.NewID(), tag.ID, taggableType, taggableID); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	return err
}

func GetTags(ctx context.Context, q studiodb.Querier, taggableType, taggableID string) ([]Tag, error) {
	return studiodb.Query(ctx, q,
		`SELECT t.id, t.name, t.category, t.sequence FROM TagAssignment ta JOIN Tag t ON t.id = ta.tagId
		 WHERE ta.taggableType = ? AND ta.taggableId = ? ORDER BY ta.createdAt ASC`,
		scanTag, taggableType, taggableID)
}

// TagAssignment is one row of "this tag is attached to this entity" — used to render an
// editable per-row list (with its own assignment ID to delete by).
type TagAssignment struct {
	AssignmentID string
	Tag          Tag
}

func scanTagAssignment(rows *sql.Rows) (TagAssignment, error) {
	var a TagAssignment
	err := rows.Scan(&a.AssignmentID, &a.Tag.ID, &a.Tag.Name, &a.Tag.Category, &a.Tag.Sequence)
	return a, err
}

func GetTagAssignments(ctx context.Context, q studiodb.Querier, taggableType, taggableID string) ([]TagAssignment, error) {
	return studiodb.Query(ctx, q,
		`SELECT ta.id, t.id, t.name, t.category, t.sequence FROM TagAssignment ta JOIN Tag t ON t.id = ta.tagId
		 WHERE ta.taggableType = ? AND ta.taggableId = ? ORDER BY ta.createdAt ASC`,
		scanTagAssignment, taggableType, taggableID)
}

// AddTagToEntity attaches one tag (creating it if new) to an entity — the add-row sibling of
// SetTags, used by detail pages' per-row tag add form.
func AddTagToEntity(ctx context.Context, pool *sql.DB, taggableType, taggableID, rawName string) error {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return fmt.Errorf("tag name is required")
	}
	_, err := studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (struct{}, error) {
		tag, err := getOrCreateTagByName(ctx, tx, name)
		if err != nil {
			return struct{}{}, err
		}
		_, err = studiodb.Execute(ctx, tx,
			"INSERT IGNORE INTO TagAssignment (id, tagId, taggableType, taggableId) VALUES (?, ?, ?, ?)",
			studiodb.NewID(), tag.ID, taggableType, taggableID)
		return struct{}{}, err
	})
	return err
}

func RemoveTagAssignment(ctx context.Context, q studiodb.Querier, assignmentID string) error {
	_, err := studiodb.Execute(ctx, q, "DELETE FROM TagAssignment WHERE id = ?", assignmentID)
	return err
}

// GetAllTagsWithUsage returns every tag with total + per-taggableType usage counts, ordered by
// sequence — the Settings → Tags overview.
func GetAllTagsWithUsage(ctx context.Context, q studiodb.Querier) ([]TagUsage, error) {
	tags, err := studiodb.Query(ctx, q, "SELECT "+tagColumns+" FROM Tag ORDER BY sequence ASC", scanTag)
	if err != nil {
		return nil, err
	}
	type usageRow struct {
		TagID        string
		TaggableType string
		N            int
	}
	scanUsage := func(rows *sql.Rows) (usageRow, error) {
		var u usageRow
		err := rows.Scan(&u.TagID, &u.TaggableType, &u.N)
		return u, err
	}
	counts, err := studiodb.Query(ctx, q, "SELECT tagId, taggableType, COUNT(*) AS n FROM TagAssignment GROUP BY tagId, taggableType", scanUsage)
	if err != nil {
		return nil, err
	}

	result := make([]TagUsage, len(tags))
	for i, tag := range tags {
		result[i] = TagUsage{Tag: tag, ByType: map[string]int{}}
		for _, c := range counts {
			if c.TagID != tag.ID {
				continue
			}
			result[i].ByType[c.TaggableType] = c.N
			result[i].Total += c.N
		}
	}
	return result, nil
}

func CreateTag(ctx context.Context, q studiodb.Querier, name, category string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tag name is required")
	}
	maxRow, err := studiodb.QueryOne(ctx, q, "SELECT COALESCE(MAX(sequence), -1) AS m FROM Tag", scanCount)
	if err != nil {
		return err
	}
	next := 0
	if maxRow != nil {
		next = *maxRow + 1
	}
	_, err = studiodb.Execute(ctx, q, "INSERT INTO Tag (id, name, category, sequence) VALUES (?, ?, ?, ?)",
		studiodb.NewID(), name, nullIfEmpty(strings.TrimSpace(category)), next)
	return err
}

func RenameTag(ctx context.Context, q studiodb.Querier, id, name, category string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tag name is required")
	}
	_, err := studiodb.Execute(ctx, q, "UPDATE Tag SET name = ?, category = ? WHERE id = ?", name, nullIfEmpty(strings.TrimSpace(category)), id)
	return err
}

func DeleteTag(ctx context.Context, q studiodb.Querier, id string) error {
	// TagAssignment.tag has ON DELETE CASCADE — removing a tag removes its assignments too.
	_, err := studiodb.Execute(ctx, q, "DELETE FROM Tag WHERE id = ?", id)
	return err
}

func findNeighborTag(ctx context.Context, q studiodb.Querier, row *Tag, direction string) (*Tag, error) {
	if direction == "up" {
		return studiodb.QueryOne(ctx, q,
			"SELECT "+tagColumns+" FROM Tag WHERE id != ? AND sequence <= ? ORDER BY sequence DESC, id DESC LIMIT 1",
			scanTag, row.ID, row.Sequence)
	}
	return studiodb.QueryOne(ctx, q,
		"SELECT "+tagColumns+" FROM Tag WHERE id != ? AND sequence >= ? ORDER BY sequence ASC, id ASC LIMIT 1",
		scanTag, row.ID, row.Sequence)
}

func ReorderTag(ctx context.Context, pool *sql.DB, id, direction string) error {
	row, err := studiodb.QueryOne(ctx, pool, "SELECT "+tagColumns+" FROM Tag WHERE id = ?", scanTag, id)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("tag %s not found", id)
	}
	neighbor, err := findNeighborTag(ctx, pool, row, direction)
	if err != nil || neighbor == nil {
		return err
	}
	_, err = studiodb.WithTransaction(ctx, pool, func(tx *sql.Tx) (struct{}, error) {
		if _, err := studiodb.Execute(ctx, tx, "UPDATE Tag SET sequence = ? WHERE id = ?", neighbor.Sequence, row.ID); err != nil {
			return struct{}{}, err
		}
		_, err := studiodb.Execute(ctx, tx, "UPDATE Tag SET sequence = ? WHERE id = ?", row.Sequence, neighbor.ID)
		return struct{}{}, err
	})
	return err
}
