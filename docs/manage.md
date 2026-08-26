# Managing the production database

Ad-hoc SQLite access on the VPS — confirming a user's status, fixing a role, checking what a
deploy actually seeded. For routine backups see `docs/deploy.md`'s Backups section; this page is
about poking at the live database directly.

## Where it lives

`/opt/app/studio/data/studio.db`, plus its `-wal`/`-shm` sidecars (WAL mode — see
`internal/db/pool.go`). Owned by the `studio` system user, and the containing `data/` directory is
`0750` (see `ansible/roles/studio_app/tasks/main.yml`), so only `studio` itself or `sudo` can read
it — a plain `sqlite3 ...` as `karl`/`ansible` will fail with a permission error.

SSH in as `karl` or `ansible` per `ansible/README.md` (once `harden.yml` has run against the box).

## One-time: install the sqlite3 CLI

`deploy.yml` doesn't install it — the app itself needs no CLI, it embeds a pure-Go SQLite driver.

```bash
sudo apt install sqlite3
```

## Opening the database

Run as the owning user rather than as root — `sudo -u` executes the command directly, so the
`studio` system user's `nologin` shell doesn't get in the way:

```bash
sudo -u studio sqlite3 /opt/app/studio/data/studio.db
```

For read-only poking while the app keeps running (WAL mode allows any number of concurrent
readers alongside the app's own writer connection, safely):

```bash
sudo -u studio sqlite3 -readonly /opt/app/studio/data/studio.db
```

Handy settings inside the `sqlite3` shell:

```sql
.headers on
.mode column
.tables
.schema User
```

## Confirming a user's status

```sql
SELECT id, name, email, role, emailVerifiedAt, createdAt FROM User ORDER BY createdAt DESC;
SELECT id, name, role, emailVerifiedAt FROM User WHERE email = 'someone@example.com';
```

- `role` is one of `pending` / `conservator` / `admin` (`internal/auth/types.go`).
- `emailVerifiedAt` is `NULL` until verified. Every account created by `BOOTSTRAP_ADMIN_*` or the
  `SEED_EXAMPLE_DATA` demo seed already has it set; a real self-registered account won't until
  they click the verification link (or SMTP isn't configured and the link never arrived — see
  below).

## Fixing a user

Promote to admin:

```sql
UPDATE User SET role = 'admin' WHERE email = 'someone@example.com';
```

Mark an account verified by hand (e.g. `SMTP_HOST` wasn't set yet when they registered, so the
verification email only ever hit the journal — see `internal/mail/mailer.go`):

```sql
UPDATE User SET emailVerifiedAt = strftime('%Y-%m-%d %H:%M:%f', 'now') WHERE email = 'someone@example.com';
```

Match `strftime('%Y-%m-%d %H:%M:%f', 'now')` exactly — every `DATETIME` column in this schema is
written the same way, matching `internal/db/pool.go`'s `_time_format=sqlite` pragma.

**Don't hand-write `passwordHash`.** It's a scrypt hash with an embedded salt
(`internal/auth/password.go`) — there's no way to construct a valid one by hand, and there's no
supported raw-SQL password reset. If someone's locked out, the real fix is the app's own
"forgot password" flow, which needs `SMTP_HOST` configured (`ansible/group_vars/all/vault.yml`,
`vault_studio_smtp_*`) to actually deliver the reset email.

## Other useful checks

```sql
-- Confirm migrations applied after a deploy
SELECT filename FROM schema_migrations ORDER BY filename;

-- Classifier reference data seeded correctly (every <select> in the app reads from this)
SELECT type, COUNT(*) FROM Classifier GROUP BY type ORDER BY type;

-- Settings/dashboard limits currently overridden from default (internal/settings/features.go)
SELECT key, value, updatedAt FROM AppSetting ORDER BY key;

-- Quick sanity counts after a fresh deploy
SELECT
  (SELECT COUNT(*) FROM User)    AS users,
  (SELECT COUNT(*) FROM Client)  AS clients,
  (SELECT COUNT(*) FROM Asset)   AS assets,
  (SELECT COUNT(*) FROM Project) AS projects;

-- Cheap integrity check, safe to run any time
PRAGMA integrity_check;
```

## Editing safely

Single-row `UPDATE`s like the ones above are safe with the service still running — WAL mode's one
writer / many readers model covers the `sqlite3` CLI just as much as the app's own connections.

For anything bulkier or structural, stop the service first so nothing else is writing concurrently:

```bash
sudo systemctl stop studio
sudo -u studio sqlite3 /opt/app/studio/data/studio.db
# ... your changes ...
sudo systemctl start studio
```

## App logs, for context alongside DB changes

```bash
sudo journalctl -u studio -f                  # live tail
sudo journalctl -u studio --since "1 hour ago"
```
