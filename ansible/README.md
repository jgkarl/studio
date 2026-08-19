# Ansible deploy for Studio

Targets a plain **Ubuntu 24.04** VPS, no container runtime needed there at all - installs
`libvips42`, downloads the release binary `.github/workflows/release.yml` publishes to GitHub
Releases on every `vX.Y.Z` tag, and runs it under systemd. Optionally installs and configures
[Caddy](https://caddyserver.com/) as the reverse proxy/TLS terminator. See `../docs/deploy.md` for
the full picture (backups, troubleshooting, the reasoning behind Ubuntu 24.04).

Requires `ansible` on your own machine (`pipx install ansible-core`, or your distro's package),
SSH access to the target host, and Python on the target itself (Ubuntu 24.04 has it out of the
box).

## One-time setup

```bash
cd ansible

cp inventory.example.ini inventory.ini
# edit inventory.ini: your VPS's IP/hostname, SSH user, key

cp group_vars/all/all.yml.example group_vars/all/all.yml
# edit group_vars/all/all.yml: studio_domain, studio_release_version if you want to pin

cp group_vars/all/vault.yml.example group_vars/all/vault.yml
# edit group_vars/all/vault.yml: a real AUTH_SECRET (openssl rand -hex 32), APP_URL, SMTP if wanted
ansible-vault encrypt group_vars/all/vault.yml
```

`inventory.ini`, `group_vars/all/all.yml`, and `group_vars/all/vault.yml` are all gitignored - only the
`.example` templates are tracked. Encrypting `vault.yml` means it's safe to keep around (or even
commit) afterwards; you'll need the vault password (`--ask-vault-pass`, or a
`--vault-password-file`) for every playbook run.

## Deploy (first time, and any time packages/config need to change)

```bash
ansible-playbook deploy.yml --ask-vault-pass
```

Fully idempotent - installs libvips42 + Caddy, creates the `studio` system user and
`/opt/studio/data`, templates the env file and systemd unit from your vault values, fetches the
release binary, and starts everything. Safe to re-run any time; only touches what's actually out
of date.

## Update to a newer release

```bash
ansible-playbook update.yml --ask-vault-pass
```

Fetches whatever `studio_release_version` resolves to (default `"latest"`, tracking the most
recent GitHub Release) and restarts the service only if the version actually changed. Doesn't
touch packages, the system user, or Caddy - just the binary.

Pin an exact version instead of always tracking latest:

```bash
ansible-playbook update.yml --ask-vault-pass -e studio_release_version=v1.2.3
```

(or set `studio_release_version: "v1.2.3"` in `group_vars/all/all.yml` to make the pin permanent).

## What's not automated

- **DNS** - point `studio_domain`'s `A`/`AAAA` record at your VPS yourself before the Caddy step
  runs, or Let's Encrypt's HTTP challenge will fail.
- **Backups** - see `../docs/deploy.md`'s Backups section; `/opt/studio/data/` is the whole
  app's state (SQLite DB + uploaded media).
- **Firewall** - if you run `ufw` or similar, open 80/443 (and 22 for SSH) yourself; this
  playbook doesn't touch firewall rules.
