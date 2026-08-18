-- Backs the asset/media "pattern layer": simple rectangle regions drawn over a Media item, each
-- tagged with an annotation_type Classifier (hatch pattern + color — see
-- 0013_seed_annotation_types.sql). Percentage-based coordinates (0-100) rather than pixels so a
-- region stays correct regardless of which size the client happens to be viewing the image at —
-- no resize bookkeeping needed.
CREATE TABLE MediaAnnotationRegion (
    id TEXT NOT NULL PRIMARY KEY,
    mediaId TEXT NOT NULL,
    annotationTypeId TEXT NOT NULL,
    xPct REAL NOT NULL,
    yPct REAL NOT NULL,
    widthPct REAL NOT NULL,
    heightPct REAL NOT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT MediaAnnotationRegion_mediaId_fkey FOREIGN KEY (mediaId) REFERENCES Media(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT MediaAnnotationRegion_annotationTypeId_fkey FOREIGN KEY (annotationTypeId) REFERENCES Classifier(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX MediaAnnotationRegion_mediaId_idx ON MediaAnnotationRegion(mediaId);
