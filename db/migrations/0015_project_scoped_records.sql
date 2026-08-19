-- Project becomes the mandatory parent for Treatment, Assessment (renamed from AssetState — the
-- old "condition status" quick-add), and Report: registering an Asset now leads into creating its
-- first Project, and everything else is recorded under a Project from then on (still filterable
-- by Asset via the join to Project.assetId — see each module's ListByAsset).
--
-- This app has no real deployment yet — data/studio.db is local throwaway dev data (confirmed
-- with the app owner) — so these are plain rebuilds with no row-preserving INSERT/SELECT dance:
-- Treatment/Report/AssetState are dropped and recreated with their final shape. Asset is the one
-- exception that IS rebuilt with its data carried forward, because unlike the other three it
-- isn't itself being reset — only the FK target for its circular currentStateId reference (see
-- 0001_init.sql's header comment) is changing name from AssetState to Assessment. Asset is
-- rebuilt *before* AssetState is dropped, same cautious ordering 0004 used for Project/Order:
-- never leave a table's schema text pointing at an already-dropped table, even transiently.

-- 1. Assessment: same shape AssetState had, minus the dead recordedViaActivityId (nothing has
--    written to it since Treatments replaced the Activity Notebook — see internal/treatments'
--    package doc), plus projectId is now required and the record gets an updatedAt/deletedAt
--    since Assessments are now a full editable, unlinkable module like Treatments/Reports.
CREATE TABLE Assessment (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    projectId TEXT NOT NULL,
    "condition" TEXT NOT NULL,
    description TEXT NOT NULL,
    recordedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    deletedAt DATETIME NULL,
    CONSTRAINT Assessment_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT Assessment_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX Assessment_assetId_idx ON Assessment(assetId);
CREATE INDEX Assessment_projectId_idx ON Assessment(projectId);

-- 2. Rebuild Asset so its currentStateId FK points at Assessment instead of AssetState.
--    currentStateId is cleared (no old Assessment row can possibly match an AssetState-era id).
CREATE TABLE Asset_new (
    id TEXT NOT NULL PRIMARY KEY,
    clientId TEXT NOT NULL,
    referenceCode TEXT NOT NULL,
    assetTypeId TEXT NOT NULL,
    title TEXT NULL,
    artist TEXT NULL,
    creationPeriod TEXT NULL,
    dimensions TEXT NULL,
    description TEXT NULL,
    medium TEXT NULL,
    signatureMarks TEXT NULL,
    weight TEXT NULL,
    provenance TEXT NULL,
    acquisitionDate DATETIME NULL,
    estimatedValue REAL NULL,
    isInsured INTEGER NULL,
    locationInStudio TEXT NULL,
    currentStateId TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Asset_clientId_fkey FOREIGN KEY (clientId) REFERENCES Client(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Asset_assetTypeId_fkey FOREIGN KEY (assetTypeId) REFERENCES Classifier(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Asset_currentStateId_fkey FOREIGN KEY (currentStateId) REFERENCES Assessment(id) ON DELETE SET NULL ON UPDATE CASCADE
);

INSERT INTO Asset_new (id, clientId, referenceCode, assetTypeId, title, artist, creationPeriod, dimensions,
    description, medium, signatureMarks, weight, provenance, acquisitionDate, estimatedValue, isInsured,
    locationInStudio, currentStateId, createdAt, updatedAt)
SELECT id, clientId, referenceCode, assetTypeId, title, artist, creationPeriod, dimensions,
    description, medium, signatureMarks, weight, provenance, acquisitionDate, estimatedValue, isInsured,
    locationInStudio, NULL, createdAt, updatedAt
FROM Asset;

DROP TABLE Asset;
ALTER TABLE Asset_new RENAME TO Asset;
CREATE UNIQUE INDEX Asset_referenceCode_key ON Asset(referenceCode);
CREATE UNIQUE INDEX Asset_currentStateId_key ON Asset(currentStateId);

-- 3. Now safe to drop the old table — nothing references it any more.
DROP TABLE AssetState;

-- 4. Treatment: add the required projectId. No data worth preserving (see header).
DROP TABLE Treatment;
CREATE TABLE Treatment (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    projectId TEXT NOT NULL,
    method TEXT NOT NULL,
    title TEXT NOT NULL,
    notes TEXT NOT NULL,
    performedByUserId TEXT NULL,
    performedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    deletedAt DATETIME NULL,
    CONSTRAINT Treatment_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Treatment_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Treatment_performedByUserId_fkey FOREIGN KEY (performedByUserId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE INDEX Treatment_assetId_idx ON Treatment(assetId);
CREATE INDEX Treatment_projectId_idx ON Treatment(projectId);

-- 5. Report: projectId goes from optional to required. No data worth preserving (see header).
DROP TABLE Report;
CREATE TABLE Report (
    id TEXT NOT NULL PRIMARY KEY,
    projectId TEXT NOT NULL,
    assetId TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    authorId TEXT NULL,
    summary TEXT NULL,
    conditionFindings TEXT NULL,
    treatmentPerformed TEXT NULL,
    materialsUsed TEXT NULL,
    recommendations TEXT NULL,
    coverMediaId TEXT NULL,
    layoutStyle TEXT NOT NULL DEFAULT 'standard',
    showCover INTEGER NOT NULL DEFAULT 1,
    showSummary INTEGER NOT NULL DEFAULT 1,
    showCondition INTEGER NOT NULL DEFAULT 1,
    showTreatment INTEGER NOT NULL DEFAULT 1,
    showMaterials INTEGER NOT NULL DEFAULT 1,
    showRecommendations INTEGER NOT NULL DEFAULT 1,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    deletedAt DATETIME NULL,
    CONSTRAINT Report_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Report_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Report_authorId_fkey FOREIGN KEY (authorId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT Report_coverMediaId_fkey FOREIGN KEY (coverMediaId) REFERENCES Media(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE INDEX Report_assetId_idx ON Report(assetId);
CREATE INDEX Report_projectId_idx ON Report(projectId);

-- 6. MediaReference gains a caption, used by the Report image gallery's per-image description
--    field (and generically available anywhere else a caption makes sense).
ALTER TABLE MediaReference ADD COLUMN caption TEXT NULL;
