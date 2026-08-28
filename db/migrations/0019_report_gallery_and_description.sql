-- Report gets a general "Description" field (notes/misc content) replacing the old Condition
-- findings/Treatment performed/Materials used trio in the editing UI - those three columns stay
-- in place (old report data isn't lost, "don't lose old data" reasoning used elsewhere in this
-- app) but are no longer shown or editable; description/showDescription follow the same
-- nullable/boolean pattern the sections/show-flags added in 0008 already use. galleryColumns is
-- the report's own "1 column / 2 column" image gallery layout choice.
ALTER TABLE Report ADD COLUMN description TEXT NULL;
ALTER TABLE Report ADD COLUMN showDescription INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN galleryColumns INTEGER NOT NULL DEFAULT 2;

-- Per-report gallery customization (drag-drop order, per-image "stretch to column width") for a
-- MediaReference the report's gallery displays - a MediaReference itself has no notion of "this
-- report's gallery position" since the same reference can appear in several reports' galleries at
-- once (BuildGallery spans every Assessment/Treatment/Report/Project-direct upload on the
-- Project), so this lives in its own table keyed by (reportId, mediaReferenceId) rather than as
-- columns on MediaReference. A row only exists once a conservator has actually customized that
-- item for that report; absent a row, the gallery falls back to its default timestamp order and
-- non-stretched display (see BuildGallery/ReorderGallery in internal/reporter).
CREATE TABLE ReportGalleryItem (
    id TEXT NOT NULL PRIMARY KEY,
    reportId TEXT NOT NULL,
    mediaReferenceId TEXT NOT NULL,
    sortOrder INTEGER NOT NULL DEFAULT 0,
    stretch INTEGER NOT NULL DEFAULT 0,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT ReportGalleryItem_reportId_fkey FOREIGN KEY (reportId) REFERENCES Report(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT ReportGalleryItem_mediaReferenceId_fkey FOREIGN KEY (mediaReferenceId) REFERENCES MediaReference(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX ReportGalleryItem_report_ref_key ON ReportGalleryItem(reportId, mediaReferenceId);
