package settings

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	studiodb "studio/internal/db"
)

// AppSetting is a generic key/value config row — the Dashboard's per-module list caps and the
// Reports "reportable field" checkboxes are both stored here rather than in bespoke tables (see
// db/migrations/0016_app_setting.sql).
type AppSetting struct {
	ID        string
	Key       string
	Value     string
	UpdatedAt time.Time
}

func getSetting(ctx context.Context, q studiodb.Querier, key string) (string, bool) {
	val, err := studiodb.QueryOne(ctx, q, "SELECT value FROM AppSetting WHERE key = ?",
		func(rows *sql.Rows) (string, error) { var v string; err := rows.Scan(&v); return v, err }, key)
	if err != nil || val == nil {
		return "", false
	}
	return *val, true
}

func setSetting(ctx context.Context, q studiodb.Querier, key, value string) error {
	_, err := studiodb.Execute(ctx, q, `
		INSERT INTO AppSetting (id, key, value, updatedAt) VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updatedAt = excluded.updatedAt`,
		studiodb.NewID(), key, value, time.Now())
	return err
}

// GetInt reads a numeric setting, falling back to def when unset or unparsable — every caller
// site is a display cap (never validated input), so a bad stored value degrading to the default
// is preferable to an error page.
func GetInt(ctx context.Context, q studiodb.Querier, key string, def int) int {
	raw, ok := getSetting(ctx, q, key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func SetInt(ctx context.Context, q studiodb.Querier, key string, value int) error {
	return setSetting(ctx, q, key, strconv.Itoa(value))
}

// GetBool reads a boolean setting, falling back to def when unset — used for the reportable-field
// registry, where def is true (a field stays in reports until someone explicitly turns it off).
func GetBool(ctx context.Context, q studiodb.Querier, key string, def bool) bool {
	raw, ok := getSetting(ctx, q, key)
	if !ok {
		return def
	}
	return raw == "true"
}

func SetBool(ctx context.Context, q studiodb.Querier, key string, value bool) error {
	v := "false"
	if value {
		v = "true"
	}
	return setSetting(ctx, q, key, v)
}
