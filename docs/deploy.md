# Deploying to a VPS (e.g. Hostinger)

Studio ships as a single statically-linked-except-for-libvips Go binary, `dist/server`, committed
to this repo (see [`make release`](../Makefile) — it's rebuilt inside a Debian 12 container and
extracted, so it doesn't matter whether your own machine has libvips installed). Deploying is
`git clone` + one `apt install` + a systemd unit — no Docker, no separate database server, no
build step on the VPS itself.

## Why Debian 12, specifically

The binary is dynamically linked against `libvips.so.42` (image processing — thumbnails, the
IIIF deep-zoom viewer) and its own dependency chain (glib, cairo, pango, libjpeg, libpng, ...).
On **Ubuntu**, several of those transitive packages (`libmagickcore`/`libmagickwand`, pulled in by
libvips) are gated behind Ubuntu Pro/ESM and fail to install on a plain account. **Debian doesn't
have this problem at all** — `apt install libvips42` pulls in everything it needs from the regular
archive, no subscription, no extra steps. Match the container the binary was actually built in
(`golang:1.25-bookworm`) and this just works. Hostinger's VPS plans let you pick the OS image at
provisioning — choose **Debian 12** (or any Debian 12 derivative).

If you're stuck with an Ubuntu VPS: either run `sudo apt install libvips42` and hope your image
already has Ubuntu Pro attached (`pro status`), or build the binary yourself against Ubuntu's
libvips instead (`FROM ubuntu:24.04` in a copy of the Dockerfile's builder stage) rather than
relying on the committed `dist/server`, which is built for Debian 12.

## One-time server setup

1. **Provision the VPS** with a Debian 12 image, note its IP, point a domain's `A` record at it
   if you have one.

2. **Install libvips and git:**
   ```bash
   sudo apt-get update
   sudo apt-get install -y --no-install-recommends libvips42 git ca-certificates
   ```

3. **Create a dedicated user** (the systemd unit runs as this user, not root):
   ```bash
   sudo useradd --system --create-home --home-dir /opt/studio --shell /usr/sbin/nologin studio
   ```

4. **Clone the repo:**
   ```bash
   sudo -u studio git clone <your-repo-url> /opt/studio
   cd /opt/studio
   ```
   `dist/server` — the binary this app actually runs — comes with the clone; there's nothing to
   build on the server.

5. **Configure:**
   ```bash
   sudo -u studio cp .env.example /opt/studio/.env
   sudo -u studio nano /opt/studio/.env
   ```
   At minimum, set a real `AUTH_SECRET` (`openssl rand -hex 32`), set `ALLOW_DEV_LOGIN=false`,
   set `APP_URL` to your real `https://...` origin, and fill in `SMTP_*` if you want real email
   (leaving `SMTP_HOST` blank just logs email bodies to the systemd journal instead of sending
   them — fine for a first smoke test, not for real use).

   To get a first account on a brand new database, also set `BOOTSTRAP_ADMIN_NAME` and
   `BOOTSTRAP_ADMIN_EMAIL` — the app creates that one admin user on its first boot (see
   `.env.example`). Sign in via the dev-login picker at `/login` once `ALLOW_DEV_LOGIN=true`
   temporarily allows it, then use Settings -> Users to promote a real registered account and
   turn `ALLOW_DEV_LOGIN` back off. Every Classifier the app's forms depend on (asset types,
   materials, condition states, order/quote/invoice statuses, ...) is seeded automatically on
   every boot too — nothing to run by hand for either of these.

6. **Install and start the systemd service:**
   ```bash
   sudo cp deploy/studio.service /etc/systemd/system/studio.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now studio
   sudo systemctl status studio     # should show "active (running)"
   journalctl -u studio -f          # tail logs
   ```
   First start applies every migration in `db/migrations/` automatically and creates
   `data/studio.db` — nothing to run by hand.

7. **Put a reverse proxy in front for TLS.** [Caddy](https://caddyserver.com/) is the easiest —
   automatic Let's Encrypt certificates from a two-line config:
   ```bash
   sudo apt-get install -y caddy   # or see caddyserver.com/docs/install
   ```
   `/etc/caddy/Caddyfile`:
   ```
   yourdomain.com {
       reverse_proxy localhost:3000
   }
   ```
   ```bash
   sudo systemctl reload caddy
   ```
   (Nginx + certbot works too if you'd rather; Caddy just needs less config for the same result.)

## Every deploy after that

```bash
cd /opt/studio
sudo -u studio git pull --ff-only   # brings in the already-built dist/server along with source
sudo systemctl restart studio
```

That's the whole update flow — `dist/server` is committed, so a deploy is a `git pull`, never a
build. If you change anything under `db/migrations/`, the restart applies the new migration
automatically on startup, same as every other start.

## Backups

Everything the app owns lives under one directory: `/opt/studio/data/` (`studio.db` + the
`media-storage/` subdirectory — see `.env`'s `DB_PATH`/`MEDIA_STORAGE_DIR`). Back that one
directory up however you like:

```bash
tar czf studio-backup-$(date +%F).tar.gz -C /opt/studio data
```

SQLite is a single file, so a plain file copy while the app is running is safe as long as you
copy `studio.db` together with its `-wal`/`-shm` sidecar files in the same pass (which `tar` on
the whole directory does) — SQLite's WAL mode is designed for exactly this. For a live/hot-backup
setup with zero pause, look at [Litestream](https://litestream.io/) later; a nightly `tar` cron
job is more than enough for a prototype's traffic.

## Local dev / trying it without a VPS

`docker compose up --build` still works for a quick local try (see the repo root
`docker-compose.yml`) — useful for development, not the recommended path for the VPS itself.
