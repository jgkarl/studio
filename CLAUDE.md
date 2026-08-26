# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## App summary

Studio is a conservation-studio management app: clients bring in assets (objects), which move
through condition assessments, treatments, and reports, organized under Projects. Server-rendered,
single Go binary, one SQLite file — no separate API layer, no frontend build.

Feature areas (one `internal/` package each, see Architecture below):
- **Auth** — email/password, session cookies, no OAuth/third-party provider, no dev-login picker.
- **Clients** — profile CRUD.
- **Assets** — the objects being conserved; registering one leads straight into its first Project.
- **Assessments** — condition-status records (renamed from an earlier `AssetState` concept).
- **Treatments** — conservation work performed.
- **Workflows** (routes as `/projects`) — Project is the mandatory organizing unit; Assessments,
  Treatments, Reports, and Media are all recorded under a Project and stay filterable by Asset too.
  Includes a kanban board (native HTML5 drag-and-drop, `static/js/kanban.js`).
- **Reporter** (routes as `/reports`) — structured report sections, exportable.
- **Media** — upload, thumbnails, deep-zoom viewer (OpenSeadragon against a from-scratch IIIF
  Image API in `internal/iiif`), and a "pattern layer" (hand-drawn annotation regions with
  hatch-pattern overlays, `internal/media/annotations.go`).
- **Export** — HTML and PDF (Helvetica-only, no embedded fonts) report export.
- **Settings** — Classifiers (structural reference data every `<select>` reads from), feature
  toggles, user role management (admin-only).
- **Dashboard** — landing/stats page.
- **i18n** — hand-rolled Estonian (default) / English dictionary + cookie toggle; only covers
  nav + dashboard so far, other modules still hardcode English.

## Design principles

- **No ORM.** Hand-written SQL, hand-written row-scanning functions, hand-written Go structs
  matching the migrations field-for-field — kept in sync by hand, deliberately.
- **No web framework.** Stdlib `net/http` with Go 1.22+ method+wildcard patterns
  (`mux.HandleFunc("GET /clients/{id}", ...)`).
- **No JS bundler, no JS framework.** Small vanilla `static/js/*.js` islands only where a page
  genuinely needs client-side interactivity. The one vendored exception is OpenSeadragon
  (`static/openseadragon/`) — real tiled deep zoom was enough of a lift to not hand-roll.
- **Minimal dependencies overall**, auditable — see go.mod (five direct deps).
- **One hard layout rule**: every page renders inside one `max-width: 1200px` container
  (`internal/web/layout.templ`) so no module makes its own width decision.
- **Module-per-package**: each `internal/<name>` package owns its own routes, templ views, and
  SQL, and is independently testable against a real (not mocked) SQLite database.

## Tech stack

- **Go 1.25+**, one binary (`cmd/server/main.go`). `go.mod` pins the version; `go` auto-fetches a
  matching toolchain if the local one is older.
- **[templ](https://templ.guide)** compiles `.templ` → `_templ.go` (gitignored, regenerate with
  `templ generate` — `make run`/`make dev`/`make test` do this automatically).
- **SQLite via `modernc.org/sqlite`** — pure Go, no cgo, compiled into the binary. Generic query
  helpers in `internal/db` (see Architecture). Pragmas: `foreign_keys(1)`, `journal_mode(WAL)`,
  `busy_timeout(5000)`, `_time_format=sqlite` (millisecond precision).
- **`github.com/davidbyttow/govips/v2`** — cgo binding to libvips; the one non-pure-Go dependency.
  Used for thumbnail generation and the IIIF Image API's live transforms. Requires `libvips-dev`
  to build locally (`libvips42` at runtime only).
- **`github.com/go-pdf/fpdf`** — PDF export, core Helvetica font only (no embedded TTF).
- **`golang.org/x/crypto/scrypt`** — password hashing.
- CSS: one hand-written stylesheet (`static/css/app.css`), plain custom properties, three
  breakpoints (640/768/1024, mobile-first). No preprocessor, no utility framework.

## Architecture

**Composition root**: `cmd/server/main.go` — loads config, opens the SQLite pool, runs migrations,
constructs each module's `Service`, calls every module's `Mount(mux, svc)` in sequence.

**Module shape** (consistent across every `internal/<name>` package):
- `types.go` — hand-written Go structs matching the migrations.
- `queries.go` — raw SQL against `internal/db`'s generics: `Query[T]`/`QueryOne[T]` (SELECT +
  hand-written `ScanFunc[T]`), `Execute` (INSERT/UPDATE/DELETE), `WithTransaction`. All four take
  a `db.Querier` interface satisfied by both `*sql.DB` and `*sql.Tx`, so the same query code works
  pool-level or inside a transaction.
- `handlers.go` — a `Service{Pool *sql.DB, Auth *auth.Service, ...}` struct, a `Mount(mux, svc)`
  registering that module's routes (gated via `svc.Auth.RequireUser`/`RequireAdmin`), and the
  handler methods themselves.
- `views.templ` (+ generated `views_templ.go`) — templ views for that module.
- `queries_test.go` — tests against `internal/testutil.OpenTestDB(t)`, a real migrated temp-file
  SQLite database (no mocks — the query helpers are thin enough that mocking them would test
  nothing).

**Shared infrastructure** (the only packages every module depends on):
- `internal/db` — the query generics above, `pool.go` (connection + pragmas), `migrate.go`
  (applies `db/migrations/*.sql` in filename order, splitting each file on `;` since SQLite has no
  multi-statement `Exec`, tracked in a `schema_migrations` table).
- `internal/web` — `Layout`/`AppLayout` page shell, `Navbar`, and the handful of truly shared
  components (`Card`, `Field`, `PageHeader`, `ExportLinks`).
- `internal/auth` — session cookies (HMAC-SHA256, `internal/session`), `RequireUser`/
  `RequireAdmin` middleware, verification/reset tokens.
- `internal/testutil` — test-only, see above.

**IIIF/media pipeline**: `internal/media` handles upload + a "web" thumbnail variant (non-fatal on
failure); `internal/iiif` is a from-scratch IIIF Image API v3 implementation (region/size/rotation/
quality/format transforms via govips, not run through the official validator) backing both the
deep-zoom viewer (`static/js/osd-viewer.js` + vendored OpenSeadragon) and the pattern-layer
grayscale base image. `internal/media/rasterize.go` flattens a media item's drawn annotation
regions + legend into a downloadable PNG via libvips' SVG rasterizer.

**Seeding** (`internal/seed`, runs from `cmd/server/main.go` on every boot): `BootstrapAdmin`
creates the one admin account from `BOOTSTRAP_ADMIN_NAME`/`EMAIL`/`PASSWORD` if set (idempotent,
safe in production). `SeedDemoData` additionally seeds fictional demo content, but only when
`SEED_EXAMPLE_DATA=true` — dev/Docker convenience only, never set in production (see
`ansible/roles/studio_app/defaults/main.yml`).

## Data

- **SQLite**, one file at `DB_PATH` (default `./data/studio.db`), plus `-wal`/`-shm` sidecars —
  back all three up together.
- **Migrations**: `db/migrations/*.sql`, embedded into the binary via `go:embed`
  (`db/migrations/embed.go`) — applied automatically on every boot, no separate migrate step.
  Quirks: `condition` and `Order` are reserved words needing double-quoting; foreign keys must be
  declared inline in `CREATE TABLE` (no `ALTER TABLE ... ADD CONSTRAINT`); dropping/changing a
  column requires the create-new-table/copy/drop/rename dance (SQLite can't do it directly).
- **Media files**: local disk via the `StorageAdapter` interface (`internal/media/storage.go`),
  root at `MEDIA_STORAGE_DIR` — `LocalDiskAdapter` is the only implementation so far; a future
  S3-compatible backend means implementing the interface, nothing else.

## Commands

```bash
make run                              # templ generate + go run ./cmd/server
make dev                              # same, watches .templ files and restarts on change
make build                            # go build -> bin/server (needs libvips-dev locally)
make test                             # templ generate + go test ./...
make release                          # builds dist/server inside Ubuntu 24.04 via Docker/Podman
make install-hooks                    # one-time per clone, see Git hooks below
go test ./internal/clients/...        # a single package
go test ./internal/clients/... -run TestCreateAndGetByID   # a single test
go vet ./...                          # no separate linter configured — go vet is the gate
cd e2e && npm install && npm test     # Playwright e2e, needs a running server (see e2e/README.md)
```

`libvips-dev` must be installed locally (`apt install libvips-dev`) to build or test any package
that imports `internal/media` (govips is cgo) — installs cleanly on plain Ubuntu 24.04/Debian, no
extra sources needed. `templ generate` must run before `go build`/`go test` after editing any
`.templ` file; every `make` target above does this for you.

Local dev: `cp .env.example .env` (set a real `AUTH_SECRET` and `BOOTSTRAP_ADMIN_NAME`/`EMAIL`/
`PASSWORD` — there's no dev-login picker). `docker compose up --build` works too, and needs
nothing but Docker/Podman installed. See `docs/setup.md` for the full walkthrough.

## Git hooks

`make install-hooks` points this clone at the tracked `.githooks/pre-commit` (git doesn't use it
on its own). It runs `templ generate && go build && go vet && go test` whenever staged files touch
`.go`/`.templ`/`.sql`, and refuses to let an unencrypted `ansible/group_vars/all/vault.yml` be
committed. Bypass deliberately with `git commit --no-verify`.

## Deployment

Plain **Ubuntu 24.04** VPS via Ansible (`ansible/`) — no Docker, no separate database server, no
build step on the VPS. `.github/workflows/release.yml` builds a self-contained binary (migrations
and static assets embedded via `go:embed`) and publishes it to a GitHub Release on every `vX.Y.Z`
tag.

Two playbooks, run in order against a fresh host (see `ansible/README.md` for full setup,
including the pre-Ansible VPS bootstrap steps):
1. **`harden.yml`** — generic, reusable host hardening: creates a dedicated `ansible` automation
   user (pubkey-only SSH, passwordless sudo), fully disables root login, restricts SSH to named
   accounts (`AllowUsers`), enables a baseline `ufw` firewall.
2. **`deploy.yml`** — installs `libvips42`, downloads the release binary, runs it under systemd at
   `/opt/app/studio`, optionally installs Caddy (opens 80/443 in ufw itself) as reverse
   proxy/TLS terminator with automatic Let's Encrypt certs.

`update.yml` is the lightweight day-to-day path: fetches whatever `studio_release_version`
resolves to (default `"latest"`) and restarts only if the version actually changed — doesn't touch
packages, users, or Caddy. Both `deploy.yml` and `update.yml` run the same
`roles/studio_app/tasks/release.yml`, which snapshots `studio.db` (+ `-wal`/`-shm`) to
`{{ studio_home }}/backups/` before installing an actually-different version — skipped on a no-op
run or before the first deploy. `Dockerfile`/`docker-compose.yml` exist only for local "try it
without installing anything" use, not the deploy path.

## Integrations

- **SMTP** (`internal/mail`, stdlib `net/smtp`) for verification/reset emails — falls back to a
  console transport (logs the message) when `SMTP_HOST` is unset, so the whole auth flow is
  testable without real mail infrastructure.
- **GitHub Releases** — the deploy artifact distribution channel (see Deployment above); no other
  GitHub API usage.
- **Caddy** (optional, VPS only) — reverse proxy + automatic Let's Encrypt TLS.
- No OAuth, no third-party auth provider, no external database, no message queue, no cache layer.

## Docs map

- `README.md` — project overview and quick start.
- `docs/tech-stack.md` — the same material as this file's Tech Stack/Architecture sections, in
  more narrative form.
- `docs/setup.md` — full local-dev walkthrough (Docker and native), env var reference, exactly
  what first boot seeds.
- `docs/deploy.md` — deployment reasoning (why Ubuntu 24.04 specifically), backups.
- `docs/manage.md` — ad-hoc `sqlite3` access on the VPS (user status/role fixes, sanity checks).
- `ansible/README.md` — the actual playbook usage, one-time setup, VPS bootstrap steps.
- `e2e/README.md` — Playwright suite usage.
