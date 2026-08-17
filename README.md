# Studio (Go)

Go/templ/SQLite rewrite of the [Studio conservation-studio management app](../studio) — a
ground-up port from the original Next.js/TypeScript/React version, built module by module. See
that repo's `docs/features.md` for what the app does; see `docs/tech-stack.md` here for how this
rewrite is actually built, and `docs/setup.md` for a more detailed local-dev walkthrough than the
quick version below.

Same philosophy as the original: no ORM, hand-written SQL and row types, no web framework, plain
templ templates, JS only where a page genuinely needs client-side interactivity — small vanilla
islands, no bundler, no vendored libraries (see `static/js/`).

## Requirements

- Go 1.25+ (the module's `go.mod` pins this — `go` itself will auto-fetch a matching toolchain if
  your installed version is older, no manual upgrade needed)
- [templ](https://templ.guide) CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- `libvips` (image processing — upload thumbnails): `apt install libvips-dev` on Debian/Ubuntu for
  local dev (needs the `-dev` headers to *build*; the deployed binary only needs the runtime
  `libvips42` package — see `docs/deploy.md`). **Debian, not Ubuntu, if you can choose** — Ubuntu
  gates several of libvips's transitive dependencies behind Ubuntu Pro/ESM and the plain-account
  install fails; Debian has no such restriction.
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

`docs/deploy.md` — a VPS (Hostinger or any other), no Docker, no separate database server: the
build already lives at `dist/server`, committed to this repo (rebuilt via `make release`, which
runs inside a Debian 12 container so it doesn't matter whether your own machine has libvips
installed). Deploying is `git clone`, `apt install libvips42`, and a systemd unit.

## Tests

```bash
make test                                       # go test ./...
cd e2e && npm install && npm test               # end-to-end (needs a server running — see e2e/README.md)
```

## Layout

See the module list in git history / commit messages for build order. Each top-level package
under `internal/` is one self-contained module (auth, clients, assets, treatments, workflows
[routes as `/projects`], media, reporter [routes as `/reports`], export, settings, seed) with its
own routes, templ views, and SQL — `internal/db` and `internal/web` are the only shared
infrastructure every module builds on, and `internal/testutil` is test-only support (a real
migrated SQLite database per test, no mocks). Commerce (Quote/Order/Invoice), Tags, and
Materials-as-a-relation were removed in a later refactor to match the design artifact this app now
follows — see `docs/tech-stack.md`.

## Known, disclosed differences from the original app

A few spots where this rewrite made a different call than the original TypeScript app, on
purpose — see `docs/tech-stack.md` for the reasoning behind each:

- **No fictional demo-data seed.** Structural reference data (Classifiers) seeds automatically;
  the original's `db/seed.ts` demo clients/assets/workflows does not have a Go equivalent — every
  entity it would create is reachable through the app's own forms instead.
- **Debian, not Ubuntu**, is the one supported build/deploy target — see `docs/tech-stack.md`'s
  govips section for why.
