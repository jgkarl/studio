-- Commerce (Quote/Order/Invoice) is out of scope for the design artifact.

-- Project.orderId carries a FOREIGN KEY to "Order", and SQLite can't ALTER TABLE ... DROP COLUMN
-- a column that's part of a foreign key constraint - it needs a full table rebuild instead
-- (leaving the column in place, as Project.stage was left dead in migration 0002, isn't an option
-- here: with foreign_keys enforcement on, any INSERT/UPDATE against Project fails once the
-- referenced "Order" table is gone, even for NULL orderId values, because SQLite resolves the
-- constraint's parent table when preparing the statement).
CREATE TABLE Project_new (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    title TEXT NOT NULL,
    stage TEXT NOT NULL DEFAULT 'ingest',
    assignedToUserId TEXT NULL,
    startedAt DATETIME NULL,
    completedAt DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Project_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Project_assignedToUserId_fkey FOREIGN KEY (assignedToUserId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);

INSERT INTO Project_new (id, assetId, title, stage, assignedToUserId, startedAt, completedAt, createdAt, updatedAt)
SELECT id, assetId, title, stage, assignedToUserId, startedAt, completedAt, createdAt, updatedAt FROM Project;

DROP TABLE Project;

ALTER TABLE Project_new RENAME TO Project;

-- Drop children before parents given the FKs (Invoice -> Order -> Quote).
DROP TABLE Invoice;
DROP TABLE "Order";
DROP TABLE Quote;
