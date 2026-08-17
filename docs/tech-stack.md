# Tech Stack, Plainly

A quick-reference for anyone picking this codebase up cold.

## The one-sentence version

A single Go binary serves server-rendered HTML (templ) over a hand-written SQL layer against one
SQLite file — no ORM, no frontend build, no JS framework, no separate API layer. A handful of
vanilla JS "islands" (`static/js/`) add client-side interactivity only where a page genuinely
needs it.

## Runtime & language

- **Go 1.25+**, compiled to one static-except-for-libvips binary (`cmd/server/main.go`). `go.mod`
  pins the version; `go` auto-fetches a matching toolchain if yours is older.
- No separate backend/frontend split, no build step for the server itself (`go build`). The only
  compile-time codegen is `templ generate` (templ's `.templ` -> `_templ.go`, gitignored, run by
  the Dockerfile/Makefile before `go build`).

## Templates: templ

- [templ](https://templ.guide) compiles `.templ` files to plain Go functions returning
  `templ.Component` — type-checked at compile time, no runtime template parsing. Each module owns
  its own `views.templ`; `internal/web` holds the shared page shell (`Layout`/`AppLayout`) and the
  handful of truly reusable pieces (`Card`, `Field`, `PageHeader`, `ExportLinks`).
- Every page (forms and read views alike) renders inside one `max-width: 1200px` container
  (`internal/web/layout.templ`) — this project's one hard layout rule, chosen so no module has to
  make its own width decision.

## Database: SQLite, via a raw driver — no ORM

- **No ORM.** `internal/db` wraps [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) —
  a **pure-Go** driver (no cgo), plus four generic helpers (`Query[T]`, `QueryOne[T]`, `Execute`,
  `WithTransaction`) built on a `Querier` interface satisfied by both `*sql.DB` and `*sql.Tx`.
  Every query is hand-written SQL with `?` placeholders; every module hand-writes its own
  row-scanning function. No schema DSL, no generated client.
- **Migrations are plain `.sql` files** in `db/migrations/`, applied in filename order by
  `internal/db/migrate.go`, tracked in a `schema_migrations` table. SQLite has no
  multi-statement-per-`Exec` mode, so the runner splits each file on `;` and executes statements
  one at a time — safe here because every migration is hand-written DDL this app controls.
- **Row types are hand-written**, one Go struct per table (`sql.NullString`/`sql.NullTime` for
  nullable columns), field-for-field matching the migrations — kept in sync by hand, the same
  tradeoff as the original TypeScript app's approach, just in Go.
- Connection pragmas (`internal/db/pool.go`): `foreign_keys(1)` (off by default in SQLite —
  every module's `ON DELETE CASCADE`/`SET NULL`/`RESTRICT` relies on this being on),
  `journal_mode(WAL)` (concurrent readers alongside one writer), `busy_timeout(5000)`, and
  `_time_format=sqlite` (millisecond-precision `time.Time` read/write — matters for the
  polymorphic `MediaReference` table's "first row by `createdAt`" ordering).
- **SQLite quirks worth knowing if you touch a query:** `condition` and `Order` are reserved
  words needing double-quoting (`"condition"`, `"Order"`) as bare identifiers (the `Order` table
  itself was dropped along with the rest of Commerce — see below — but the reserved-word note
  still applies to `condition`); foreign keys must be declared inline in `CREATE TABLE` (no
  `ALTER TABLE ... ADD CONSTRAINT`) — the one circular reference (`Asset.currentStateId` <->
  `AssetState.assetId`) relies on SQLite allowing a `REFERENCES` to a table that doesn't exist yet
  at `CREATE TABLE` time. Adding a column with `ALTER TABLE ... ADD COLUMN` works fine even with a
  `REFERENCES` clause (used for `Report.coverMediaId`); dropping or changing an existing column
  doesn't (used for `Project`'s `0004`/`0006` migrations) — those go through the standard
  create-new-table/copy-data/drop-old/rename dance instead.

## Image processing: govips (cgo), thumbnails only

- [`github.com/davidbyttow/govips/v2`](https://github.com/davidbyttow/govips), a cgo binding to
  `libvips` — the one part of this app that isn't pure Go. Used for upload-time thumbnail
  generation (`internal/media/service.go`). A from-scratch IIIF Image API v3 implementation used
  to live here too (deep-zoom region/size/rotation/quality transforms) but was removed along with
  the OpenSeadragon viewer it powered — the design artifact's Media view is a lightbox (CSS
  rotate/brightness/contrast, no server-side transform), not a deep-zoom viewer.
- This is why the build/release process (below) runs inside Docker rather than `go build`
  directly on most dev machines — govips needs `libvips-dev` installed to compile against, and
  **Debian, not Ubuntu**, specifically: several of libvips's transitive dependencies
  (`libmagickcore`/`libmagickwand`) are gated behind Ubuntu Pro/ESM on Ubuntu with no plain-account
  fallback; Debian's regular archive has no such gating.

## Auth: hand-rolled, no external provider

- Session cookies: HMAC-SHA256 signed, `internal/session`. Passwords: `golang.org/x/crypto/scrypt`.
  Verification/reset tokens: single-use, stored hashed. No OAuth, no third-party auth service —
  same "few dependencies, all auditable" posture as the rest of the stack.
- `ALLOW_DEV_LOGIN=true` adds a one-click picker (any existing user, no password) for local/staging
  use only — never set it on a real deployment.

## JS: no bundler, plain vanilla islands only

There is no `npm run build` step for the frontend — no webpack/esbuild/vite, and no vendored or
CDN-loaded third-party JS at all (OpenSeadragon and a CDN-loaded TipTap both used to live here;
both were removed when the deep-zoom viewer and rich-text editor they powered were replaced —
see `docs/tech-stack.md`'s image-processing section and `internal/reporter`'s structured sections).
Every script is plain vanilla JS loaded via `<script type="module">`, reused across every kanban
board on a shared `data-url-template`/`data-status-field` convention (`static/js/kanban.js` —
native HTML5 Drag and Drop, chosen over `dnd-kit`, which has no vanilla build) plus small
single-page islands (`static/js/lightbox.js` for the Media view, `static/js/album.js` for the
media grid's filter/search).

## Build & deploy

- `make release` builds `dist/server` **inside a Debian 12 container** (matching the deploy
  target) and extracts it — `go build` doesn't need to work on your own machine for this to
  produce a correct binary. The stripped (`-ldflags="-s -w"`) binary is committed to the repo.
- Deploying is `git clone` + `apt install libvips42` + a systemd unit (`deploy/studio.service`) —
  no Docker required on the VPS itself, no separate database server, no build step there. See
  `docs/deploy.md`.
- `Dockerfile`/`docker-compose.yml` still exist for local "try it without installing anything"
  use and as the CI/verification substrate for changes touching `internal/media` (this sandbox,
  like many dev machines, can't install libvips-dev directly due to the Ubuntu/ESM issue above).

## Testing

- `internal/testutil` gives package tests a real, migrated, temp-file SQLite database — no mocks.
  Packages that don't import `internal/media` (and therefore don't need govips/cgo to compile) run
  locally with plain `go test`; packages that do (media, reporter, workflows, assets — anything
  touching photo attachments) need the Docker build, same as the app itself.
- `e2e/` is a Playwright suite (`npm test`) driven against a live server — always the real
  Debian/libvips Docker image, never mocks — using the system-installed Chromium instead of a
  Playwright-managed download.
