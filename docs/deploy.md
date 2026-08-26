# Deploying to a VPS

Studio deploys to a plain **Ubuntu 24.04** VPS via [Ansible](https://www.ansible.com/) — no
Docker, no separate database server, no build step on the VPS itself. `.github/workflows/release.yml`
builds a single, fully self-contained binary (migrations and static assets are embedded — see
`db/migrations/embed.go` and `static/embed.go`) and publishes it to a GitHub Release on every
`vX.Y.Z` tag; the Ansible playbooks in `../ansible/` install `libvips42`, download that binary,
and run it under systemd, with Caddy in front for TLS.

**The actual playbooks and their usage live in [`../ansible/README.md`](../ansible/README.md).**
This page covers the reasoning and the parts Ansible doesn't automate (DNS, backups).

## Why Ubuntu 24.04, specifically

Studio used to target Debian, on the assumption that Ubuntu gated `libvips-dev`'s transitive
dependencies (`libmagickcore`/`libmagickwand`) behind Ubuntu Pro/ESM. That assumption turned out
to be wrong for **24.04 "noble"** specifically — verified directly against a stock `ubuntu:24.04`
container image with no Pro/ESM subscription attached at all:

```
$ apt-get install libvips-dev
...
Setting up libmagickcore-6.q16-7t64:amd64 (8:6.9.12.98+dfsg1-5.2build2) ...
Setting up libvips-dev (8.15.1-1.1build4) ...
```

Both come from the plain `noble/universe` archive, no extra sources needed. `release.yml` builds
on a pinned `ubuntu-24.04` GitHub Actions runner for exactly this reason — the binary is
dynamically linked against `libvips.so.42`, so the build OS needs to match the deploy OS.

## One-time server setup

1. **Provision the VPS** with an Ubuntu 24.04 image, note its IP, point a domain's `A` record at
   it if you have one.
2. **Cut a release** if you haven't yet: `git tag v0.1.0 && git push origin v0.1.0` — this is what
   `release.yml` needs to have published something for Ansible to download.
3. Follow **[`ansible/README.md`](../ansible/README.md)**: copy the inventory/vars/vault
   templates, fill in your host and secrets, `ansible-vault encrypt group_vars/all/vault.yml`,
   then `ansible-playbook harden.yml --ask-vault-pass` followed by
   `ansible-playbook deploy.yml --ask-vault-pass`.

`harden.yml` runs first, once: creates a dedicated pubkey-only `ansible` automation user, fully
disables root over SSH, restricts logins to just the accounts you name, and enables a baseline
`ufw` firewall. `deploy.yml` then installs `libvips42`, creates the `studio` system user,
downloads the release binary, templates the systemd unit and env file, opens 80/443 in `ufw`, and
(if `studio_domain` is set) installs and configures Caddy with automatic Let's Encrypt
certificates.

## Every deploy after that

```bash
cd ansible
ansible-playbook update.yml --ask-vault-pass
```

Fetches whatever `studio_release_version` resolves to (default `"latest"`) and restarts the
service only if the version actually changed — see `ansible/README.md` for pinning to an exact
tag instead of always tracking latest.

## Backups

`deploy.yml`/`update.yml` already snapshot the database itself automatically before installing an
actually-different release — see "Update to a newer release" in `../ansible/README.md`. That
covers `studio.db` but not `media-storage/`; for the whole app's state, or a backup outside of a
deploy, do it yourself.

Everything the app owns lives under one directory on the VPS: `/opt/app/studio/data/`
(`studio.db` + the `media-storage/` subdirectory — see `DB_PATH`/`MEDIA_STORAGE_DIR` in the
templated `.env`). Back that one directory up however you like:

```bash
tar czf studio-backup-$(date +%F).tar.gz -C /opt/app/studio data
```

SQLite is a single file, so a plain file copy while the app is running is safe as long as you
copy `studio.db` together with its `-wal`/`-shm` sidecar files in the same pass (which `tar` on
the whole directory does) — SQLite's WAL mode is designed for exactly this. For a live/hot-backup
setup with zero pause, look at [Litestream](https://litestream.io/) later; a nightly `tar` cron
job is more than enough for a prototype's traffic.

## Managing the database

See **[`manage.md`](manage.md)** — ad-hoc `sqlite3` access on the VPS: confirming a user's role/
verification status, promoting a user to admin, checking what a deploy actually seeded.

## Local dev / trying it without a VPS

`docker compose up --build` still works for a quick local try (see the repo root
`docker-compose.yml`) — useful for development, not the deploy path itself.
