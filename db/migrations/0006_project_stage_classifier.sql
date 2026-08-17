-- Repoint Project.stage at the new 5-value project_stage Classifier (inquiry/queue/working/
-- review/completed) instead of the old 7-value workflow state machine it was left dead against
-- since migration 0002. Also add priority and targetReviewDate, from the design's Projects
-- kanban + intake-form "Assignment" section. Table rebuild (not a plain ALTER) so the stale
-- default and old CHECK-free but now-meaningless values get replaced cleanly.
CREATE TABLE Project_new (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    title TEXT NOT NULL,
    stage TEXT NOT NULL DEFAULT 'inquiry',
    priority TEXT NOT NULL DEFAULT 'standard',
    targetReviewDate DATETIME NULL,
    assignedToUserId TEXT NULL,
    startedAt DATETIME NULL,
    completedAt DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Project_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Project_assignedToUserId_fkey FOREIGN KEY (assignedToUserId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);

-- One-time default: existing rows land on 'working' (or 'completed' if already marked done) —
-- there's no way to recover which of the 5 new stages an old row was "really" in.
INSERT INTO Project_new (id, assetId, title, stage, priority, targetReviewDate, assignedToUserId, startedAt, completedAt, createdAt, updatedAt)
SELECT id, assetId, title,
       CASE WHEN completedAt IS NOT NULL THEN 'completed' ELSE 'working' END,
       'standard', NULL, assignedToUserId, startedAt, completedAt, createdAt, updatedAt
FROM Project;

DROP TABLE Project;

ALTER TABLE Project_new RENAME TO Project;
