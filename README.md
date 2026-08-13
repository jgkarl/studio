# Studio (Go)

Go/templ/MySQL rewrite of the [Studio conservation-studio management app](../studio) — a ground-up
port from the original Next.js/TypeScript/React version, built module by module. See that repo's
`docs/features.md` for what the finished app does; this repo is being brought up to parity with it
one module at a time (see git log / commit history for progress).

Same philosophy as the original: no ORM, hand-written SQL and row types, no web framework, plain
templ templates, JS only where a page genuinely needs client-side interactivity (vendored
libraries where they have a real framework-agnostic build, small vanilla islands otherwise — see
`static/js/` and `static/vendor/` as those modules land).

## Requirements

- Go 1.25+ (the module's `go.mod` pins this — `go` itself will auto-fetch a matching toolchain if
  your installed version is older, no manual upgrade needed)
- [templ](https://templ.guide) CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- MySQL/MariaDB (a local instance, or `docker compose up db` / `podman-compose up db`)
- `libvips` (from the Media module onward — `apt install libvips-dev` on Debian/Ubuntu; the
  Docker image installs it itself, see `Dockerfile`)

## Local dev

```bash
cp .env.example .env   # then edit DATABASE_URL etc.
make run                # templ generate + go run ./cmd/server
```

Or the whole stack (app + MySQL) via compose:

```bash
cp .env.example .env
docker compose up --build   # or: podman-compose up --build
```

Either way, migrations in `db/migrations/*.sql` are applied automatically on startup (tracked in
a `schema_migrations` table — no separate migrate step to remember).

`make dev` runs `templ generate --watch` alongside the server for live template reload during
development.

## Tests

```bash
make test        # go test ./...
cd e2e && npm install && npx playwright test   # end-to-end, once e2e/ exists (module 14)
```

## Layout

See the module list in git history / commit messages for build order. Each top-level package
under `internal/` is one self-contained module (auth, clients, assets, workflows, media, commerce,
reporter, export, settings) with its own routes, templ views, and SQL — `internal/db` and
`internal/web` are the only shared infrastructure every module builds on.
