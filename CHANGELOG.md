# Changelog

Notable user- and operator-visible changes, one entry per shipped version. See `git log` for full
commit-level detail. Format loosely follows [Keep a Changelog](https://keepachangelog.com).

## [v0.4.11] - 2026-08-29

### Fixed
- Saving an annotated image still 500'd for any real photo even with the font installed: the bake
  SVG embeds the full-resolution base image as a base64 `data:` URI, and librsvg/libxml2 refuse to
  parse any single XML node larger than ~10 MB (`XML parse error … Extra content at the end of the
  document`). The rasterizer now loads that SVG with libvips' `svgload` "unlimited" flag. Tiny
  CI/demo images stayed under the limit, which is why it only ever failed in production. Same fix
  applied to the annotated-image path in report HTML/PDF exports.

### Added
- The pattern-layer annotation editor is reworked. The image now just pans/zooms by default (the
  "+ Add region" mode toggle is gone). **+ New annotation** opens a small panel to mark one region
  (rectangle or freehand), pick its type, and add an optional note before saving. Existing
  annotations are listed in a table under the image — each row editable inline (type + note) or
  deletable. Regions can now carry a per-region note
  (`db/migrations/0020_media_annotation_note.sql`), separate from the whole-image caption.

### Changed
- Every stage of an annotated-version bake now logs at debug (`category=media`, `event=bake_step`
  / `bake_rasterize_failed` / `bake_saved`), and libvips' own diagnostics are routed through slog
  (`event=libvips`) instead of bare stderr. The editor's Save button surfaces the real server
  error instead of a generic "couldn't save".

## [v0.4.10] - 2026-08-29

### Fixed
- Saving an annotated image on a freshly provisioned host failed with a 500 ("cannot save
  annotated image"): librsvg renders the legend/note text in a baked version through Pango, which
  needs a font on disk, and a minimal server has none. `deploy.yml` now installs
  `fonts-dejavu-core`. Also affected the annotated-image path in report HTML/PDF exports.
- `ansible/reset-data.yml` left orphaned `ReportGalleryItem` rows (and retired `Activity` rows)
  behind — its cleanup script deletes their parent rows with foreign keys disabled, so the
  cascade never fired. Both are now wiped in the same pass.

### Changed
- Annotated-version bake failures are now logged server-side (`category=media`, `event=bake_failed`)
  with the underlying error, instead of only being returned in the HTTP response.

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
