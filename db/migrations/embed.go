// Package migrations embeds the SQL migration files into the compiled binary — a deployed
// release doesn't need db/migrations/ shipped alongside it (see internal/db.Migrate and
// cmd/server/main.go).
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
