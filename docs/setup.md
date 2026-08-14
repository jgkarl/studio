# Local Setup

Two ways to run this locally. Both apply migrations and seed the structural Classifier reference
data automatically on first boot — nothing to run by hand before the app is usable.

## Option A — Docker/Podman Compose (recommended, nothing installed locally)

```bash
cp .env.example .env
# edit .env — at minimum, set a real AUTH_SECRET (openssl rand -hex 32)
docker compose up --build
# or: podman-compose up --build
```

This builds the app image (inside Debian 12, so it doesn't matter whether your own machine has
`libvips-dev`) and starts it with a bind-mounted `./data` directory. Open
<http://localhost:3000>.

With `ALLOW_DEV_LOGIN=true` (the `.env.example` default) and `BOOTSTRAP_ADMIN_NAME`/
`BOOTSTRAP_ADMIN_EMAIL` set, a one-click **Dev login** button appears on `/login` for that admin
— no password needed locally. Data persists in `./data` across restarts; delete it to start over
from an empty database.

## Option B — Native Go (needs libvips-dev installed)

```bash
go install github.com/a-h/templ/cmd/templ@v0.3.1020
sudo apt-get install -y libvips-dev pkg-config   # Debian; see docs/tech-stack.md for why not Ubuntu
cp .env.example .env
make run   # templ generate + go run ./cmd/server
```

Open <http://localhost:3000>, same dev-login flow as above. `make dev` runs
`templ generate --watch` alongside the server for live template reload while editing `.templ`
files.

## Everyday commands

| Command | Does |
|---|---|
| `make run` | `templ generate` + `go run ./cmd/server` |
| `make dev` | Same, but watches `.templ` files and restarts on change |
| `make build` | Local `go build` to `bin/server` (needs `libvips-dev` installed) |
| `make release` | Builds `dist/server` inside Debian 12 via Docker/Podman — what actually gets deployed, see `docs/deploy.md` |
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
- `ALLOW_DEV_LOGIN` — `true` shows one-click dev-login buttons for every existing user; set
  `false` for anything beyond your own machine.
- `BOOTSTRAP_ADMIN_NAME` / `BOOTSTRAP_ADMIN_EMAIL` — if both are set, creates that one admin user
  on first boot against a database with no matching email yet (idempotent — safe to leave set).
- `SMTP_*` — leave `SMTP_HOST` blank locally: registration/reset emails print to the server
  console instead of sending, so the whole auth flow is testable without real mail infra.

## What "first boot" actually seeds

Every boot (native or Docker, dev or production — see `cmd/server/main.go`, right after
migrations):

- Every Classifier type the app's `<select>` options read from (asset types, materials, condition
  states, activity types, order/quote/invoice statuses, client types, contact methods) — 115 rows
  total, idempotent (`INSERT OR IGNORE`), safe to run on every start.
- One admin `User`, only if `BOOTSTRAP_ADMIN_NAME`/`BOOTSTRAP_ADMIN_EMAIL` are set and no `User`
  with that email exists yet.

There is deliberately no fictional-demo-data seed (the original TypeScript app's `db/seed.ts` —
several clients/assets/workflows/reports for exercising every screen immediately). That's dev-only
convenience content, not something either local dev or a production deploy strictly needs — every
entity it would create is reachable through the app's own CRUD forms, which is how `e2e/tests/
smoke.spec.js` populates a fresh database for its own run.
