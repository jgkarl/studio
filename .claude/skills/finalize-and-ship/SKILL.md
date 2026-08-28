---
name: finalize-and-ship
description: End-to-end finalization and ship checklist for the Studio repo — cleanup, tests, screenshots, docs sync, changelog, commit/push/tag, wait for the GitHub release build, then deploy via ansible update.yml (with automatic DB backup). Use when a feature/fix is implementation-complete and it's time to ship it.
---

# Finalize and ship

Run this once the actual feature/bugfix work in the conversation is done and the user wants it
shipped (they say "ship it", "finalize this", or invoke `/finalize-and-ship` directly). This skill
is the *closing* checklist, not a place to start new design decisions — if something here surfaces
a real open question (ambiguous version bump, a failing test that isn't obviously related, a
production deploy), stop and ask rather than guessing.

Work through the steps below **in order**. Each has a reason it's where it is — don't reorder
them opportunistically (e.g. don't tag before the release-workflow-triggering push has actually
landed, don't deploy before the tagged build succeeded).

## 0. Orient

- `git status` and `git fetch origin` **before touching anything** — check `git log
  main..origin/main`. If origin has moved, you will need to merge it in before pushing later
  (step 8) anyway; knowing now avoids surprises. Never `git reset --hard`/`checkout .` over
  uncommitted work without asking first.
- Confirm you're on `main` (this repo ships straight from main, no release-branch flow).
- Skim `git diff` / `git status` yourself so you know the real scope of what's about to ship —
  the steps below assume you already know which files changed and why.

## 1. Legacy cleanup pass

Before anything gets tested and locked in by a commit, do one deliberate pass for staleness this
change introduced or exposed:

- **Dead code**: a field/function/branch that's no longer called now that this change landed.
  Don't leave backwards-compat shims, `_unused` renames, or "// removed" comments — if you're
  certain it's unused, delete it outright (grep for callers first; check templ-generated
  `_templ.go` isn't the only caller before trusting "unused").
- **Stale docs**: any doc section, comment, or example that now describes the *old* behavior.
  Update or remove it in the same pass — don't leave two contradictory descriptions of the same
  feature.
- **Legacy DB columns are the one deliberate exception**: this repo's convention (see existing
  `legacy`-commented columns in `db/migrations/`) is to keep old columns in place with a comment
  rather than dropping them, so historical data isn't lost. Don't "clean up" those — only remove
  columns/tables that were never real (e.g. something added and reverted within the same
  unshipped change).

## 2. Update documentation

Check each of these against what actually changed — most ships only touch a couple:

- **`docs/schema.md` is mandatory whenever a migration changed** (CLAUDE.md enforces this as a
  standing rule, not a suggestion). Update both the DDL block and the Mermaid `erDiagram` block to
  match the new `db/migrations/*.sql` file — table-by-table, not just the delta. If you generated
  `docs/database-schema.drawio` (the draw.io ER diagram) in this same change, update it too so the
  two don't drift apart.
- `CLAUDE.md` — if the architecture, module list, tech stack, or commands actually changed.
- `docs/tech-stack.md`, `docs/setup.md`, `docs/deploy.md`, `docs/manage.md` — if setup/deploy/ops
  steps changed.
- `ansible/README.md` — if a playbook/task/var was added, renamed, or its usage changed.
- `e2e/README.md` — if the e2e suite's structure or run instructions changed.
- `README.md` — if user-facing setup or feature summary changed.

## 3. Build/test gate

```bash
templ generate
go build ./...
go vet ./...
go test ./...
```

All four must pass clean before continuing. This duplicates what the pre-commit hook will do at
step 7, but failing here is cheaper to fix than failing mid-commit.

## 4. Screenshots (only if UI-visible behavior changed)

`docs/screenshots/*.png` are generated, not hand-maintained — see `e2e/README.md`. Skip this step
entirely for backend-only/ansible-only/docs-only changes. If you touched anything a screenshot
would show (layout, new page, changed component, icon set, etc.):

```bash
make release   # or: docker build -t studio .
docker run -d --name studio-e2e -p 4070:3000 \
  -e AUTH_SECRET=some-random-string \
  -e BOOTSTRAP_ADMIN_NAME="Ada Admin" \
  -e BOOTSTRAP_ADMIN_EMAIL="ada@studio.local" \
  -e BOOTSTRAP_ADMIN_PASSWORD="correct-horse-battery-staple" \
  -v "$(mktemp -d):/data" \
  studio

cd e2e && npm install && BASE_URL=http://localhost:4070 npm test
cd .. && docker rm -f studio-e2e
```

This needs a **fresh** database (`smoke.spec.js` fails on a stale `A-0001` reference code from a
prior run — that's why it's a throwaway bind mount, not the dev DB). Both `smoke.spec.js` and
`responsive.spec.js` run and overwrite files under `docs/screenshots/`. Diff the resulting PNGs
sanely (they'll almost all show as "modified" even for unrelated-looking changes, since fonts/
timestamps shift pixels) — that's expected, not a bug.

## 5. Changelog

Keep `CHANGELOG.md` (repo root, [Keep a Changelog](https://keepachangelog.com) style) up to date.
Create it on first use if it doesn't exist yet. Add a new `## [vX.Y.Z] - YYYY-MM-DD` section for
*this* ship, one line per user-visible change (skip pure refactors/internal cleanup unless they
change behavior). Use the actual ship date, and the version you land on in step 8 — write this
section last, after you've picked the version number, so they don't drift.

## 6. Commit

- Stage specific files (`git add <path>...`), not `-A`/`.` — review `git status` after staging
  and double-check anything that touches `ansible/group_vars/**` isn't an unencrypted vault file
  before it's staged.
- Write a commit message that explains *why*, matching this repo's existing log style (see `git
  log --oneline -10` for tone/length).
- Let `.githooks/pre-commit` run for real (it re-runs `templ generate && go build && go vet && go
  test`, plus the vault-encryption check) — don't `--no-verify` unless a hook failure is a known
  false positive you've confirmed with the user.

## 7. Push

`git push origin main`. If it's rejected (remote moved since step 0's fetch — check again with
`git fetch origin && git log main..origin/main`), merge the remote changes in
(`git merge origin/main`) rather than force-pushing, resolve any conflicts, re-run step 3's gate
if anything code-relevant merged in, and push again.

## 8. Tag

Pick the next version: `git tag --sort=-v:refname | head -1` shows the last one. This repo's
observed convention is a **patch bump per ship** (`v0.4.7` → `v0.4.8`) even for fairly large
single-commit feature sets — reserve a minor bump for something that's actually a breaking change
or a genuinely new capability axis, and ask the user if it's not obvious which this is.

```bash
git tag -a vX.Y.Z -m "<one-line summary matching the commit>"
git push origin vX.Y.Z
```

Pushing the tag is what triggers `.github/workflows/release.yml` (`on: push: tags: ["v*.*.*"]`).

## 9. Wait for the release build

```bash
gh run list --workflow=release.yml --limit 1
gh run watch <run-id>          # or poll gh run list until status is "completed"
```

Do **not** proceed to step 10 until this run's conclusion is `success`. If it fails, the tag now
points at a commit with no published binary — diagnose from the run log (`gh run view <run-id>
--log-failed`), fix, and re-ship (new commit, new patch tag) rather than trying to reuse the
failed tag.

## 10. Deploy (production — confirm before running)

`ansible-playbook update.yml` fetches the just-published release and restarts the live systemd
service on the real VPS. This is the one step in this whole checklist with a blast radius beyond
GitHub — **stop and confirm with the user before running it**, even though the rest of this
checklist is meant to run without hand-holding. It already backs up the database automatically
(see `ansible/roles/studio_app/tasks/release.yml` — snapshots `studio.db` + WAL/SHM to
`{{ studio_home }}/backups/` whenever the resolved version is actually about to change, skipped
on a no-op run), so no separate manual backup step is needed.

```bash
cd ansible
ansible-playbook update.yml         # vault password comes from .vault-pass via ansible.cfg
```

After it completes, verify: the play reports `changed` on "Download the release binary" and
"Record the deployed version" (a no-op re-run reports everything green with no changes, which
means the version didn't actually update — investigate before declaring success), and a fresh
`studio-db-<timestamp>.tar.gz` exists under `{{ studio_home }}/backups/` on the host.

`ansible-playbook reset-data.yml` is a **separate, unrelated, opt-in-only** maintenance playbook
(wipes content data/media) — never run it as part of a routine ship, regardless of how this step
is phrased.

## 11. Report back

Summarize what shipped: the version tag, a one-line description of the change, confirmation the
release build succeeded, and whether production was updated (or is still waiting on the user's
go-ahead from step 10).
