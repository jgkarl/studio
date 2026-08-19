-- Re-seeds the annotation_type Classifier — present in the original Next.js app, dropped when
-- stuudio was built to match a stripped-down design artifact, reintroduced here for the
-- "pattern layer" feature (see MediaAnnotationRegion, 0012). Each row's data JSON carries the
-- hatch direction + color the pattern-layer SVG overlay renders it with (internal/media
-- annotations.go / views.templ) — ported from the original app's HatchDirection type (lib/types.ts):
-- hatch-diagonal / hatch-antidiagonal / hatch-horizontal / hatch-vertical.
INSERT OR IGNORE INTO Classifier (id, type, code, sequence, title, titleEt, description, data, updatedAt) VALUES
    ('clsfr_annotation_type_loss', 'annotation_type', 'loss', 0, 'Loss / Damage', 'Kadu / Kahjustus', 'Missing or damaged material.', '{"hatch":"hatch-diagonal","color":"#dc2626"}', strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('clsfr_annotation_type_cleaning_area', 'annotation_type', 'cleaning_area', 1, 'Cleaning Area', 'Puhastusala', 'Surface cleaning performed or needed here.', '{"hatch":"hatch-horizontal","color":"#2563eb"}', strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('clsfr_annotation_type_retouching', 'annotation_type', 'retouching', 2, 'Retouching', 'Retušeerimine', 'Inpainting / retouching area.', '{"hatch":"hatch-vertical","color":"#16a34a"}', strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('clsfr_annotation_type_structural_repair', 'annotation_type', 'structural_repair', 3, 'Structural Repair', 'Struktuurne parandus', 'Structural repair area.', '{"hatch":"hatch-antidiagonal","color":"#ea580c"}', strftime('%Y-%m-%d %H:%M:%f', 'now')),
    ('clsfr_annotation_type_consolidation', 'annotation_type', 'consolidation', 4, 'Consolidation', 'Konsolideerimine', 'Loose or flaking material stabilized here.', '{"hatch":"hatch-diagonal","color":"#7c3aed"}', strftime('%Y-%m-%d %H:%M:%f', 'now'));
