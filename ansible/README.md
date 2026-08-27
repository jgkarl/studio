# Ansible for Studio

Targets a plain **Ubuntu 24.04** VPS, no container runtime needed there at all. Two playbooks,
run in order against a fresh host:

1. **`harden.yml`** - generic host hardening: creates a dedicated `ansible` automation user
   (pubkey-only, passwordless sudo), fully disables root over SSH, restricts logins to just the
   accounts you name, and enables a baseline `ufw` firewall (SSH only). No app-specific
   assumptions - reusable for any future host under `/opt/app/*`.
2. **`deploy.yml`** - installs `libvips42`, downloads the release binary
   `.github/workflows/release.yml` publishes to GitHub Releases on every `vX.Y.Z` tag, and runs it
   under systemd at `/opt/app/studio`. Optionally installs and configures
   [Caddy](https://caddyserver.com/) (opening 80/443 in ufw itself) as the reverse proxy/TLS
   terminator.

See `../docs/deploy.md` for the full picture (backups, troubleshooting, the reasoning behind
Ubuntu 24.04).

Requires `ansible` on your own machine (`pipx install ansible-core`, or your distro's package),
SSH access to the target host, and Python on the target itself (Ubuntu 24.04 has it out of the
box).

## 0. Brand new VPS — before any of this

Neither playbook can do anything until you can already SSH into the box with *some* sudo-capable
account — that first connection is what `harden.yml` uses to create the dedicated `ansible` user
in the first place, and it can't bootstrap its own access.

1. **Provision** an Ubuntu 24.04 image from your provider, note its public IP.
2. **Log in once manually** to confirm the provider's initial access actually works — typically
   `root` with a password or provider-injected key on a totally fresh box:
   ```bash
   ssh root@<ip>
   ```
3. **Generate a local keypair per account** this box will end up with (mirrors
   `harden_extra_allowed_users`/`harden_ansible_pubkey` below — one key per human, plus one for the
   `ansible` automation user):
   ```bash
   ssh-keygen -t ed25519 -C "karl@<host>" -f ~/.ssh/<host>_karl
   ssh-keygen -t ed25519 -C "ansible@<host>" -f ~/.ssh/<host>_ansible
   ```
   `harden.yml` creates the `ansible` account itself — you only ever need *its public key*
   (`~/.ssh/<host>_ansible.pub`, goes in `harden_ansible_pubkey` below). Its private key never
   needs to touch the server or this repo; keep it for whatever will actually use that account
   later (CI, a second machine, etc).
4. **Copy your own key onto the box** so nothing here depends on password auth even for its very
   first connection:
   ```bash
   ssh-copy-id -i ~/.ssh/<host>_karl.pub root@<ip>
   ```
5. **Add a `Host` entry to `~/.ssh/config`** and confirm it connects with no password prompt:
   ```
   Host <host>
     HostName <ip>
     User root
     IdentityFile ~/.ssh/<host>_karl
   ```
   ```bash
   ssh <host> whoami
   ```
6. Point `inventory.ini` (step below) at that same account/key. Only once that connection works
   is it safe to run `harden.yml`.

## One-time local setup

```bash
cd ansible

ansible-galaxy collection install -r requirements.yml   # community.general.ufw, ansible.posix.authorized_key

cp inventory.example.ini inventory.ini
# edit inventory.ini: your VPS's IP/hostname, SSH user, key - whatever account you connect as here
# needs sudo already, since harden.yml uses it to create the "ansible" user in the first place

cp group_vars/all/all.yml.example group_vars/all/all.yml
# edit group_vars/all/all.yml: harden_ansible_pubkey, harden_extra_allowed_users, studio_domain,
# studio_release_version if you want to pin

cp group_vars/all/vault.yml.example group_vars/all/vault.yml
# edit group_vars/all/vault.yml: a real AUTH_SECRET (openssl rand -hex 32), APP_URL, SMTP if wanted,
# and vault_ansible_become_password if the account inventory.ini connects as needs a sudo password
ansible-vault encrypt group_vars/all/vault.yml
```

`inventory.ini`, `group_vars/all/all.yml`, and `group_vars/all/vault.yml` are all gitignored - only
the `.example` templates are tracked. Encrypting `vault.yml` means it's safe to keep around (or
even commit) afterwards; you'll need the vault password for every playbook run.

**The vault password** (decrypts `vault.yml` itself - a different password from
`vault_ansible_become_password`, which is just one value stored inside it) is supplied via a
password file, not typed by hand each time. `ansible.cfg` already points at `ansible/.vault-pass`
(gitignored); create it once:

```bash
openssl rand -base64 32 > .vault-pass   # or reuse whatever password you already encrypted vault.yml with
chmod 600 .vault-pass
```

Every `ansible-playbook` command below then just works with no extra flag. If you'd rather type
the password by hand instead of keeping it in a file, delete `.vault-pass` and add
`--ask-vault-pass` to each command (or remove `vault_password_file` from `ansible.cfg` to make that
the default again).

**Sudo ("become") password**: both playbooks run every task via `become: true`. If the account
`inventory.ini` connects as already has passwordless sudo (true of the dedicated `ansible` user
`harden.yml` creates, which `deploy.yml`/`update.yml` connect as) there's nothing else to do. If it
doesn't - often the case for `harden.yml`'s own first run, e.g. a VPS provider's default account, or
your own login - set `vault_ansible_become_password` in `vault.yml` (mapped to the
`ansible_become_password` magic variable in `all.yml`) instead of passing `--ask-become-pass` by
hand on every run. Leave it blank to fall back to passwordless-sudo behavior unchanged.

Already have a `vault.yml` from before this variable existed? Add the one line with
`ansible-vault edit group_vars/all/vault.yml` - everything else in this doc works either way.

## 1. Harden the host (once, before the first deploy)

```bash
ansible-playbook harden.yml
```

Fully idempotent, safe to re-run any time (e.g. after adding a name to
`harden_extra_allowed_users`). After this runs: only the accounts in `harden_extra_allowed_users`
plus the new `ansible` user can SSH in at all (`AllowUsers`), root is fully locked out
(`PermitRootLogin no` plus pubkey/password both disabled for it as belt-and-suspenders), the
`ansible` user is pubkey-only with passwordless sudo, and `ufw` is enabled with only port 22 open.

**Before running this**, make sure `harden_extra_allowed_users` in `group_vars/all/all.yml`
includes whatever account `inventory.ini` connects as - it's your one chance to avoid locking
yourself out, since `AllowUsers` takes effect the moment this run finishes.

## 2. Deploy the app (first time, and any time packages/config need to change)

```bash
ansible-playbook deploy.yml
```

Fully idempotent - installs libvips42 + Caddy, creates the `studio` system user and
`/opt/app/studio/data`, templates the env file and systemd unit from your vault values, fetches
the release binary, opens 80/443 in ufw if `studio_domain` is set, and starts everything. Safe to
re-run any time; only touches what's actually out of date.

## Update to a newer release

```bash
ansible-playbook update.yml
```

Fetches whatever `studio_release_version` resolves to (default `"latest"`, tracking the most
recent GitHub Release) and restarts the service only if the version actually changed. Doesn't
touch packages, the system user, ufw, or Caddy - just the binary.

Whenever a version change is actually about to happen (both here and in `deploy.yml`, since both
run the same `roles/studio_app/tasks/release.yml`), the database is backed up first - a
`studio.db`/`-wal`/`-shm` snapshot (`tar`, tolerant of the sidecars not existing) written to
`{{ studio_home }}/backups/studio-db-<timestamp>.tar.gz` before the new binary is installed or the
service restarts. Skipped on a no-op run (nothing to install) and before the very first deploy (no
database yet). Not automatically pruned:

```bash
# on the VPS, delete backups older than 30 days
find /opt/app/studio/backups -name '*.tar.gz' -mtime +30 -delete
```

Pin an exact version instead of always tracking latest:

```bash
ansible-playbook update.yml -e studio_release_version=v1.2.3
```

(or set `studio_release_version: "v1.2.3"` in `group_vars/all/all.yml` to make the pin permanent).

## What's not automated

- **DNS** - point `studio_domain`'s `A`/`AAAA` record at your VPS yourself before the Caddy step
  runs, or Let's Encrypt's HTTP challenge will fail.
- **Full backups** - `deploy.yml`/`update.yml` only ever auto-back-up the database itself (see
  above), never `media-storage/`. For the whole app's state, see `../docs/deploy.md`'s Backups
  section.
- **The very first login** - `harden.yml` needs an account that already has sudo on the box
  (typically whatever the VPS provider set up, or an existing account like `karl`) before it can
  create the `ansible` user; there's no way to automate that first bootstrap.
