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

## CSS: one hand-written stylesheet, standard breakpoints

- `static/css/app.css`, plain CSS custom properties for the color/spacing system, no
  preprocessor, no utility framework. Every `@media` query in the file uses exactly one of three
  `min-width` breakpoints — `640px` (sm), `768px` (md), `1024px` (lg) — mobile-first
  (base rules target the smallest viewport, breakpoints add columns as space allows). No other
  breakpoint value should be introduced without a specific reason; if a grid needs an
  intermediate step (e.g. the 5-column Projects kanban and the 5-card dashboard stat grid both
  step 2/3/5 columns across sm/md/lg), stack multiple rules on the same three values rather than
  picking a one-off width.
- `e2e/tests/responsive.spec.js` screenshots the two widest grids (dashboard stat cards, Projects
  kanban) at each breakpoint plus a below-640 mobile width and a well-above-1024 desktop width,
  asserting the DOM node count stays correct at every size — a permanent regression check, not
  just a one-off visual pass.

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
  nullable columns), field-for-field matching the migrations — kept in sync by hand, a deliberate
  tradeoff against a schema-driven codegen step.
- Connection pragmas (`internal/db/pool.go`): `foreign_keys(1)` (off by default in SQLite —
  every module's `ON DELETE CASCADE`/`SET NULL`/`RESTRICT` relies on this being on),
  `journal_mode(WAL)` (concurrent readers alongside one writer), `busy_timeout(5000)`, and
  `_time_format=sqlite` (millisecond-precision `time.Time` read/write — matters for the
  polymorphic `MediaReference` table's "first row by `createdAt`" ordering).
- **SQLite quirks worth knowing if you touch a query:** `condition` and `Order` are reserved
  words needing double-quoting (`"condition"`, `"Order"`) as bare identifiers (the `Order` table
  itself was dropped along with the rest of Commerce — see below — but the reserved-word note
  still applies to `condition`, still used by `Assessment`, the renamed former `AssetState`);
  foreign keys must be declared inline in `CREATE TABLE` (no `ALTER TABLE ... ADD CONSTRAINT`) —
  the one circular reference (`Asset.currentStateId` <-> `Assessment.assetId`) relies on SQLite
  allowing a `REFERENCES` to a table that doesn't exist yet at `CREATE TABLE` time. Adding a
  column with `ALTER TABLE ... ADD COLUMN` works fine even with a `REFERENCES` clause (used for
  `Report.coverMediaId`); dropping or changing an existing column doesn't (used for `Project`'s
  `0004`/`0006` migrations and `0015`'s Project-scope rebuild of `Asset`/`Assessment`/`Treatment`/
  `Report`) — those go through the standard create-new-table/copy-data/drop-old/rename dance
  instead.

## Image processing: govips (cgo), thumbnails + a real IIIF Image API

- [`github.com/davidbyttow/govips/v2`](https://github.com/davidbyttow/govips), a cgo binding to
  `libvips` — the one part of this app that isn't pure Go. Used for upload-time thumbnail
  generation (`internal/media/service.go`) and a from-scratch IIIF Image API v3 implementation
  (`internal/iiif` — region/size/rotation/quality/format transforms + `info.json`), which backs
  the Media view's "pattern layer" (grayscale base image, `quality=gray`) and the deep-zoom viewer
  (OpenSeadragon, vendored — see the JS section below — tiling against the same `info.json`).
- `internal/media/rasterize.go`'s `RenderAnnotatedImage` flattens a media item's drawn regions and
  their legend into the image itself: it builds a standalone SVG (the image as a `data:` URI plus
  the same region/legend markup the editor renders) and rasterizes it to PNG via
  `libvips`'s librsvg-backed SVG loader — no separate SVG library. Backs report HTML/PDF exports
  for media that still has regions attached to it directly (pre-dates annotated versions, below).
- An "annotated version" (started from a true original via "+ New annotated version") is its own
  real `Media` row (`EditedFromID` pointing back to the original, `DerivedLabel` "annotated"/
  "annotated 2"/...) rather than something generated on the fly - `BakeAnnotatedVersion` (same
  file) renders the *current* region set onto a grayscale conversion of the original at full
  resolution, with a black divider + the region legend + the whole-image note appended below (the
  canvas widens only in height, never distorting the photo itself), and persists that PNG as the
  version's own file plus its own web thumbnail. Runs on the editor's explicit **Save & finish**
  button (`static/js/media-editor.js`), which also saves the whole-image note; a version's previous
  file is kept alongside as a timestamped backup rather than overwritten silently. The baked
  legend/note `<text>` is sized per-image by `bakedTextPx` for a full-width A4 export — readable
  ~11–12pt regardless of the source's megapixels, hard-capped at 20pt — rather than a fixed size.
- This is why local `go build` needs `libvips-dev` installed to compile against
  (`apt install libvips-dev` — works cleanly on plain Ubuntu 24.04, see `docs/deploy.md`), and
  why the Docker/release builds run inside a container: reproducible regardless of what's
  installed on your own machine. `libvips42`'s Ubuntu package links `librsvg` directly, so no
  separate package is needed for the SVG *decode* path — but librsvg renders the legend/note
  `<text>` in a baked annotated version through Pango/Cairo, which needs an actual font file on
  disk. A minimal server has none, so `deploy.yml` also installs `fonts-dejavu-core` (the generic
  `sans-serif` family that markup asks for); without it `BakeAnnotatedVersion` and the annotated
  report exports fail at rasterization.

## Auth: hand-rolled, no external provider

- Session cookies: HMAC-SHA256 signed, `internal/session`. Passwords: `golang.org/x/crypto/scrypt`.
  Verification/reset tokens: single-use, stored hashed. No OAuth, no third-party auth service —
  same "few dependencies, all auditable" posture as the rest of the stack.
- No dev-login picker — every account, including the first admin
  (`BOOTSTRAP_ADMIN_NAME`/`EMAIL`/`PASSWORD`, see `internal/seed/admin.go`) and any seeded example
  users (`SEED_EXAMPLE_DATA=true`, see `internal/seed/demo.go`), is a real `provider="email"` row
  with a real scrypt-hashed password and `emailVerifiedAt` already set, signing in through the
  normal `/login` form like any other account.

## JS: no bundler, plain vanilla islands only

There is no `npm run build` step for the frontend — no webpack/esbuild/vite, and (with one
deliberate exception) no vendored or CDN-loaded third-party JS. That exception is
[OpenSeadragon](https://openseadragon.github.io/) (`static/openseadragon/`, vendored — real tiled
deep zoom is enough of a lift that hand-rolling it wasn't worth it, unlike the rest of the app),
initialized against `internal/iiif`'s own Image API `info.json` route by two scripts: the Media
view page's `static/js/media-viewer.js` (a plain, read-only, true-color deep-zoom embed) and the
annotated-version editor's `static/js/media-editor.js` (grayscale, matching the bake). The editor
defaults to plain pan/zoom; **+ New annotation** arms a rect/freehand draw of one region, which is
POSTed with its type and optional note on save; existing regions are a table under the image, each
row inline-editable or deletable; **Save & finish** persists the whole-image note and re-bakes the
version's file. All of that is layered on the deep-zoom surface via OpenSeadragon's
overlay/coordinate-conversion API rather than a plain flat `<img>`. Everything else stays
hand-rolled vanilla JS, including
`internal/reporter`'s structured sections, which replaced a CDN-loaded TipTap. Every other script
is plain vanilla JS loaded via `<script type="module">`, reused across every kanban board on a
shared `data-url-template`/`data-status-field` convention (`static/js/kanban.js` — native HTML5
Drag and Drop, chosen over `dnd-kit`, which has no vanilla build) plus small single-page islands
(`static/js/album.js` for the media grid's filter/search).

## Build & deploy

- Migrations and static assets are embedded in the binary (`db/migrations/embed.go`,
  `static/embed.go`) via `go:embed` — a built binary is fully self-contained, nothing else needs
  to ship alongside it.
- `.github/workflows/release.yml` builds that binary on a pinned `ubuntu-24.04` runner and
  attaches it to a GitHub Release on every `vX.Y.Z` tag. `make release` does the same build
  locally (inside a container, so it doesn't matter whether your own machine has libvips
  installed) for a local smoke test — the binary itself isn't committed to the repo.
- Deploying is [Ansible](https://www.ansible.com/) (`ansible/`) against a plain Ubuntu 24.04 VPS:
  installs `libvips42`, downloads the release binary, runs it under systemd, Caddy in front for
  TLS. No Docker on the VPS, no separate database server, no build step there. See
  `docs/deploy.md`.
- `Dockerfile`/`docker-compose.yml` exist for local "try it without installing anything" use —
  not the deploy path itself, see above.

## Testing

- `internal/testutil` gives package tests a real, migrated, temp-file SQLite database — no mocks.
- `libvips-dev` installs cleanly via plain `apt install libvips-dev` on Ubuntu 24.04 (verified
  directly — no Ubuntu Pro/ESM needed, see `docs/deploy.md`), so `go build ./...`/`go test ./...`
  work locally without Docker on a machine that has it installed, including packages that import
  `internal/media` (govips/cgo).
- `e2e/` is a Playwright suite (`npm test`) driven against a live server, using the
  system-installed Chromium instead of a Playwright-managed download.
