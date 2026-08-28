# Changelog

Notable user- and operator-visible changes, one entry per shipped version. See `git log` for full
commit-level detail. Format loosely follows [Keep a Changelog](https://keepachangelog.com).

## [v0.4.9] - 2026-08-28

### Added
- Structured request/application logging (`log/slog`): every HTTP request now gets one access-log
  line (method, path, status, duration, client IP), and every log line carries a `request_id`
  tying it to whatever request triggered it, plus `category`/`event` fields for filtering. See
  `LOG_LEVEL`, `LOG_FORMAT`, and `LOG_DIR` in `.env.example`.
- A rotation-friendly on-disk JSON log file (`LOG_DIR/studio.log`, default `data/log/`) in addition
  to the existing console/journald output, with a logrotate config (10MB, keep 5) wired into the
  ansible deploy.
- `ansible/reset-data.yml`: opt-in, `confirm=RESET-DATA`-gated playbook to wipe content
  data and/or the media library filesystem while preserving the bootstrap admin account, with a
  pre-flight backup.

### Fixed
- `ansible/reset-data.yml` failed outright with undefined-variable errors, and (once patched) a
  second time with an ACL/permissions error — three stacked bugs in how it sourced the
  `studio_app` role's shared config and ran privileged commands, now fixed.

## [v0.4.8] and earlier

Not retroactively documented here — see `git log --oneline` and each version's tag message.
