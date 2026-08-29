-- Per-region optional free-text note for the pattern layer (see MediaAnnotationRegion, 0012). An
-- annotation is now "a marked region + its annotation type + an optional note", edited row by row
-- in the media editor. Distinct from Media.description (0017), which stays a single whole-image
-- caption baked under the photo; per-region notes are editor/table metadata only for now.
ALTER TABLE MediaAnnotationRegion ADD COLUMN note TEXT;
