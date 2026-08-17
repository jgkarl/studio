-- Treatments: the design artifact's sole way to record conservation work on an Asset (the
-- Activity Notebook is retired — see migration 0006/Phase 3). Asset-scoped only, no projectId:
-- the design's Treatment data has no project relation.
CREATE TABLE Treatment (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    method TEXT NOT NULL,
    title TEXT NOT NULL,
    notes TEXT NOT NULL,
    performedByUserId TEXT NULL,
    performedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Treatment_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Treatment_performedByUserId_fkey FOREIGN KEY (performedByUserId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX Treatment_assetId_idx ON Treatment(assetId);
