# Studio

Studio is a conservation-studio management app: Go, templ, and SQLite, built module by module. See
`docs/tech-stack.md` here for how it's actually built, and `docs/setup.md` for a more detailed
local-dev walkthrough than the quick version below.

Deliberately minimal: no ORM, hand-written SQL and row types, no web framework, plain templ
templates, JS only where a page genuinely needs client-side interactivity — small vanilla
islands, no bundler, almost no vendored libraries (see `static/js/`). The one deliberate exception
is [OpenSeadragon](https://openseadragon.github.io/) (`static/openseadragon/`), vendored for the
deep-zoom media viewer's real tile scheduling against the IIIF Image API — see
`static/js/osd-viewer.js`'s header comment.

## Requirements

- Go 1.25+ (the module's `go.mod` pins this — `go` itself will auto-fetch a matching toolchain if
  your installed version is older, no manual upgrade needed)
- [templ](https://templ.guide) CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- `libvips` (image processing — upload thumbnails, and the IIIF Image API in `internal/iiif`):
  `apt install libvips-dev` for local dev (needs the `-dev` headers to *build*; the deployed
  binary only needs the runtime `libvips42` package — see `docs/deploy.md`). Installs cleanly on
  plain **Ubuntu 24.04** with no extra sources or subscription needed — verified directly against
  a stock `ubuntu:24.04` image, see `docs/deploy.md`.
- SQLite itself needs nothing installed — the driver (`modernc.org/sqlite`) is pure Go, compiled
  straight into the binary.

## Local dev

```bash
cp .env.example .env
make run                # templ generate + go run ./cmd/server
```

Or via Docker (doesn't need Go/templ/libvips installed on your machine at all — useful for a
quick try, not how this app is meant to run in production, see `docs/deploy.md`):

```bash
cp .env.example .env
docker compose up --build   # or: podman-compose up --build
```

Either way, migrations in `db/migrations/*.sql` are applied automatically on startup (tracked in
a `schema_migrations` table — no separate migrate step to remember), the SQLite file is created
at `DB_PATH` (default `./data/studio.db`) if it doesn't exist yet, and the structural Classifier
reference data every `<select>` in the app reads from is seeded automatically too (idempotent —
safe on every restart). Set `BOOTSTRAP_ADMIN_NAME`/`BOOTSTRAP_ADMIN_EMAIL` in `.env` to also get a
first admin account with no manual database work — see `docs/setup.md`.

`make dev` runs `templ generate --watch` alongside the server for live template reload during
development.

## Deploying

`docs/deploy.md` — a plain Ubuntu 24.04 VPS, no Docker, no separate database server:
`.github/workflows/release.yml` publishes a self-contained release binary to GitHub on every
version tag, and Ansible (`ansible/`) installs `libvips42`, downloads it, and runs it under
systemd with Caddy in front. See `ansible/README.md` for the actual playbook usage.

## Tests

```bash
make test                                       # go test ./...
cd e2e && npm install && npm test               # end-to-end (needs a server running — see e2e/README.md)
```

## Git hooks

```bash
make install-hooks   # one-time, per clone
```

Points this clone at the tracked `.githooks/pre-commit` (git doesn't use it on its own). It gates
any commit touching Go/templ/SQL files on `templ generate` + `go build` + `go vet` + `go test`
passing, and refuses to let an unencrypted `ansible/group_vars/all/vault.yml` be committed (that
file is gitignored, so this only matters if it's ever force-added). Bypass deliberately with
`git commit --no-verify` if you really need to.

## Layout

See the module list in git history / commit messages for build order. Each top-level package
under `internal/` is one self-contained module (auth, clients, assets, assessments, treatments,
workflows [routes as `/projects`], media, reporter [routes as `/reports`], export, settings, seed)
with its own routes, templ views, and SQL — `internal/db` and `internal/web` are the only shared
infrastructure every module builds on, and `internal/testutil` is test-only support (a real
migrated SQLite database per test, no mocks). Commerce (Quote/Order/Invoice), Tags, and
Materials-as-a-relation were removed in a later refactor to match the design artifact this app now
follows — see `docs/tech-stack.md`.

Project is the mandatory organizing unit: registering an Asset leads straight into creating its
first Project, and Assessments (condition-status records, `internal/assessments` — renamed from
the old `AssetState`/"condition status" concept), Treatments, Reports, and media are all recorded
under a Project from there on, while staying filterable by Asset everywhere (every one of those
denormalizes its own `assetId` alongside the required `projectId`).

## Two deliberate behaviors worth knowing

See `docs/tech-stack.md` for the reasoning behind each:

- **Fictional demo-data seed is dev/docker-only, not a boot-time default.** Structural reference
  data (Classifiers) still seeds unconditionally on every boot. Fictional demo content (a second
  "conservator" example login, clients/assets/a project/a treatment/a report, and a few example
  media library images) only runs when `SEED_EXAMPLE_DATA=true` — see `internal/seed/demo.go` and
  "What first boot actually seeds" in `docs/setup.md`. A production deploy never sets it — only
  the one `BootstrapAdmin` account from `BOOTSTRAP_ADMIN_NAME`/`EMAIL`/`PASSWORD` boots there.
- **No dev-login picker.** Every account — the bootstrap admin and any `SEED_EXAMPLE_DATA` demo
  users included — is a real `provider="email"` row with a real password and `emailVerifiedAt`
  already set, signing in through the normal `/login` form like any other account.
