-- Annotated versions: a persisted, real Media row per "annotation session" on an image (see
-- internal/media/rasterize.go's BakeAnnotatedVersion) instead of the previous purely on-demand,
-- never-saved "download annotated" flattening. Media.editedFromId (already existed, previously
-- unused) links each version back to its true original; derivedLabel is the human-facing suffix
-- ("annotated", "annotated 2", ...) computed once at creation and stored rather than recomputed,
-- so it stays stable even if sibling versions are later deleted.
ALTER TABLE Media ADD COLUMN derivedLabel TEXT NULL;
