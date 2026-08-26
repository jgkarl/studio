# Local Setup

Two ways to run this locally. Both apply migrations and seed the structural Classifier reference
data automatically on first boot — nothing to run by hand before the app is usable.

## Option A — Docker/Podman Compose (recommended, nothing installed locally)

```bash
cp .env.example .env
# edit .env — at minimum, set a real AUTH_SECRET (openssl rand -hex 32) and
# BOOTSTRAP_ADMIN_NAME/EMAIL/PASSWORD (there's no dev-login picker — this is how you sign in)
docker compose up --build
# or: podman-compose up --build
```

This builds the app image (inside Ubuntu 24.04, so it doesn't matter whether your own machine has
`libvips-dev`) and starts it with a bind-mounted `./data` directory. Open
<http://localhost:3000> and sign in at `/login` with the email/password you set in
`BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD`. Data persists in `./data` across restarts;
delete it to start over from an empty database.

## Option B — Native Go (needs libvips-dev installed)

```bash
go install github.com/a-h/templ/cmd/templ@v0.3.1020
sudo apt-get install -y libvips-dev pkg-config   # works cleanly on plain Ubuntu 24.04+/Debian
cp .env.example .env
# edit .env — same essentials as Option A
make run   # templ generate + go run ./cmd/server
```

Open <http://localhost:3000>, same login as above. `make dev` runs `templ generate --watch`
alongside the server for live template reload while editing `.templ` files.

## Everyday commands

| Command | Does |
|---|---|
| `make run` | `templ generate` + `go run ./cmd/server` |
| `make dev` | Same, but watches `.templ` files and restarts on change |
| `make build` | Local `go build` to `bin/server` (needs `libvips-dev` installed) |
| `make release` | Builds `dist/server` inside Ubuntu 24.04 via Docker/Podman — same build `.github/workflows/release.yml` runs for a GitHub Release, see `docs/deploy.md` |
| `make test` | `go test ./...` (needs `libvips-dev` locally — see `docs/tech-stack.md`'s Testing section for which packages don't) |
| `cd e2e && npm install && npm test` | Playwright end-to-end suite against a running server — see `e2e/README.md` |

No Prisma-Studio-style DB browser — SQLite is one file (`data/studio.db` by default); use the
`sqlite3` CLI or any regular SQLite GUI pointed at it directly.

## Environment variables

See `.env.example` for the full annotated list. The essentials:

- `DB_PATH` — SQLite file path (default `./data/studio.db`); the parent directory is created
  automatically.
- `MEDIA_STORAGE_DIR` — where uploaded files are written (local disk adapter).
- `AUTH_SECRET` — signs the session cookie; generate a real one beyond local/throwaway use.
- `BOOTSTRAP_ADMIN_NAME` / `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD` — if all three are
  set, creates that one admin user (already active, signs in through the normal `/login` form) on
  first boot against a database with no matching email yet (idempotent — safe to leave set). This
  is the only way in on a brand new database — there's no dev-login picker.
- `SEED_EXAMPLE_DATA` — `true` additionally seeds fictional demo content on first boot (see below)
  — dev/local convenience only, never set this for a production deploy.
- `SMTP_*` — leave `SMTP_HOST` blank locally: registration/reset emails print to the server
  console instead of sending, so the whole auth flow is testable without real mail infra.

## What "first boot" actually seeds

Every boot (native or Docker, dev or production — see `cmd/server/main.go`, right after
migrations):

- Every Classifier type the app's `<select>` options read from (asset types, condition states,
  activity types [historical — the Activity Notebook they fed is retired], project stages,
  priority, treatment methods, client types, contact methods), idempotent (`INSERT OR IGNORE`),
  safe to run on every start.
- One admin `User`, only if `BOOTSTRAP_ADMIN_NAME`/`EMAIL`/`PASSWORD` are all set and no `User`
  with that email exists yet — already active (`emailVerifiedAt` set), signs in through the
  normal `/login` form immediately.

Only when `SEED_EXAMPLE_DATA=true` (the `.env.example`/docker-compose default — never true for a
production deploy, see `ansible/roles/studio_app/defaults/main.yml`), one more step runs:
`internal/seed.SeedDemoData` (idempotent — skipped entirely once its conservator user exists):

- A second `User`, "Conservator Example" (`role=conservator`) — same shape as the bootstrap
  admin: real password (`internal/seed.DemoUserPassword`), already active, signs in through
  `/login` immediately — alongside the one admin `BOOTSTRAP_ADMIN_NAME`/`EMAIL`/`PASSWORD`
  creates, so there's a non-admin account to sign in as too.
- Two `Client`s, two `Asset`s (with condition-state history), a `Project`, a `Treatment`, a
  `Report`, and four `Media` images — from `internal/seed/testdata/cats` — wired into the media
  library (including one annotated region), so every screen has something to look at immediately.

A production boot (`SEED_EXAMPLE_DATA` unset/`false`) never runs this — only the Classifiers and
the one `BootstrapAdmin` account above. `e2e/tests/smoke.spec.js` still populates its own fresh
database through the app's CRUD forms rather than relying on this seed, so it exercises those
forms too.
