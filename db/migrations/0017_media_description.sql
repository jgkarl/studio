-- A whole-image free-text note — distinct from MediaAnnotationRegion (per-shape) and
-- MediaReference.caption (per-attachment-context). The lightbox editor's notes field used to be
-- unsaved ("Add a note (not saved)…"); this is what its Save button now persists to.
ALTER TABLE Media ADD COLUMN description TEXT NULL;
