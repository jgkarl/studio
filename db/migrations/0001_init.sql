-- Stuudio schema, SQLite dialect. Foreign keys are declared inline (table-level CONSTRAINT
-- clauses) since SQLite's ALTER TABLE can't add constraints after the fact the way the original
-- MySQL migration did. SQLite allows a REFERENCES target that doesn't exist yet at CREATE TABLE
-- time (only checked when FK enforcement actually runs), so the schema's one circular reference
-- (Asset.currentStateId <-> AssetState.assetId) and a couple of forward references below are
-- fine as plain forward declarations, not a problem to work around.
--
-- Timestamp columns are declared exactly `DATETIME` (not TEXT, not `DATETIME(3)`) on purpose:
-- the modernc.org/sqlite driver auto-converts SQLITE_TEXT values to time.Time on scan only when
-- the column's declared type string is exactly "DATE", "DATETIME", or "TIMESTAMP" (see its
-- rows.go) — anything else (including a precision suffix) skips that conversion and hands back a
-- plain string instead, which is not the SQL type modifier itself. Written by internal/db's pool
-- (bound as Go time.Time, formatted via the `_time_format=sqlite` DSN option — see pool.go).

CREATE TABLE User (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    avatarUrl TEXT NULL,
    provider TEXT NULL,
    passwordHash TEXT NULL,
    emailVerifiedAt DATETIME NULL,
    role TEXT NOT NULL DEFAULT 'pending',
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now'))
);
CREATE UNIQUE INDEX User_email_key ON User(email);

CREATE TABLE VerificationToken (
    id TEXT NOT NULL PRIMARY KEY,
    userId TEXT NOT NULL,
    tokenHash TEXT NOT NULL,
    type TEXT NOT NULL,
    expiresAt DATETIME NOT NULL,
    usedAt DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT VerificationToken_userId_fkey FOREIGN KEY (userId) REFERENCES User(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX VerificationToken_tokenHash_key ON VerificationToken(tokenHash);
CREATE INDEX VerificationToken_userId_type_idx ON VerificationToken(userId, type);

CREATE TABLE Classifier (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL,
    code TEXT NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL,
    description TEXT NULL,
    data TEXT NULL,
    isActive INTEGER NOT NULL DEFAULT 1,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL
);
CREATE INDEX Classifier_type_sequence_idx ON Classifier(type, sequence);
CREATE UNIQUE INDEX Classifier_type_code_key ON Classifier(type, code);

CREATE TABLE Client (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'individual',
    name TEXT NOT NULL,
    email TEXT NULL,
    phone TEXT NULL,
    address TEXT NULL,
    city TEXT NULL,
    postalCode TEXT NULL,
    country TEXT NULL,
    notes TEXT NULL,
    organizationName TEXT NULL,
    contactPerson TEXT NULL,
    taxId TEXT NULL,
    preferredContactMethod TEXT NULL,
    referralSource TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL
);

CREATE TABLE Asset (
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
    CONSTRAINT Asset_currentStateId_fkey FOREIGN KEY (currentStateId) REFERENCES AssetState(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE UNIQUE INDEX Asset_referenceCode_key ON Asset(referenceCode);
CREATE UNIQUE INDEX Asset_currentStateId_key ON Asset(currentStateId);

CREATE TABLE AssetMaterial (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    materialId TEXT NOT NULL,
    role TEXT NULL,
    CONSTRAINT AssetMaterial_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT AssetMaterial_materialId_fkey FOREIGN KEY (materialId) REFERENCES Classifier(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX AssetMaterial_assetId_materialId_key ON AssetMaterial(assetId, materialId);

CREATE TABLE Tag (
    id TEXT NOT NULL PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NULL,
    sequence INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX Tag_name_key ON Tag(name);
CREATE INDEX Tag_sequence_idx ON Tag(sequence);

CREATE TABLE TagAssignment (
    id TEXT NOT NULL PRIMARY KEY,
    tagId TEXT NOT NULL,
    taggableType TEXT NOT NULL,
    taggableId TEXT NOT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT TagAssignment_tagId_fkey FOREIGN KEY (tagId) REFERENCES Tag(id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE UNIQUE INDEX TagAssignment_tagId_taggableType_taggableId_key ON TagAssignment(tagId, taggableType, taggableId);

CREATE TABLE Activity (
    id TEXT NOT NULL PRIMARY KEY,
    projectId TEXT NOT NULL,
    activityTypeId TEXT NOT NULL,
    userId TEXT NOT NULL,
    description TEXT NOT NULL,
    startedAt DATETIME NOT NULL,
    endedAt DATETIME NULL,
    durationMinutes INTEGER NULL,
    materialsUsed TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT Activity_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT Activity_activityTypeId_fkey FOREIGN KEY (activityTypeId) REFERENCES Classifier(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Activity_userId_fkey FOREIGN KEY (userId) REFERENCES User(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE TABLE AssetState (
    id TEXT NOT NULL PRIMARY KEY,
    assetId TEXT NOT NULL,
    projectId TEXT NULL,
    recordedViaActivityId TEXT NULL,
    condition TEXT NOT NULL,
    description TEXT NOT NULL,
    recordedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT AssetState_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT AssetState_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT AssetState_recordedViaActivityId_fkey FOREIGN KEY (recordedViaActivityId) REFERENCES Activity(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE TABLE Project (
    id TEXT NOT NULL PRIMARY KEY,
    orderId TEXT NULL,
    assetId TEXT NOT NULL,
    title TEXT NOT NULL,
    stage TEXT NOT NULL DEFAULT 'ingest',
    assignedToUserId TEXT NULL,
    startedAt DATETIME NULL,
    completedAt DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Project_orderId_fkey FOREIGN KEY (orderId) REFERENCES "Order"(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT Project_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Project_assignedToUserId_fkey FOREIGN KEY (assignedToUserId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE TABLE Media (
    id TEXT NOT NULL PRIMARY KEY,
    storageKey TEXT NOT NULL,
    kind TEXT NOT NULL,
    mimeType TEXT NOT NULL,
    sizeBytes INTEGER NOT NULL,
    width INTEGER NULL,
    height INTEGER NULL,
    durationSeconds INTEGER NULL,
    checksum TEXT NOT NULL,
    uploadedByUserId TEXT NOT NULL,
    editedFromId TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT Media_uploadedByUserId_fkey FOREIGN KEY (uploadedByUserId) REFERENCES User(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Media_editedFromId_fkey FOREIGN KEY (editedFromId) REFERENCES Media(id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE TABLE MediaReference (
    id TEXT NOT NULL PRIMARY KEY,
    mediaId TEXT NOT NULL,
    referencingType TEXT NOT NULL,
    referencingId TEXT NOT NULL,
    role TEXT NULL,
    sortOrder INTEGER NOT NULL DEFAULT 0,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    CONSTRAINT MediaReference_mediaId_fkey FOREIGN KEY (mediaId) REFERENCES Media(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE Quote (
    id TEXT NOT NULL PRIMARY KEY,
    clientId TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    lineItems TEXT NOT NULL,
    totalEstimate REAL NOT NULL,
    validUntil DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Quote_clientId_fkey FOREIGN KEY (clientId) REFERENCES Client(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE TABLE "Order" (
    id TEXT NOT NULL PRIMARY KEY,
    clientId TEXT NOT NULL,
    quoteId TEXT NULL,
    orderNumber TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'inquiry',
    notes TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Order_clientId_fkey FOREIGN KEY (clientId) REFERENCES Client(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Order_quoteId_fkey FOREIGN KEY (quoteId) REFERENCES Quote(id) ON DELETE SET NULL ON UPDATE CASCADE
);
CREATE UNIQUE INDEX Order_quoteId_key ON "Order"(quoteId);
CREATE UNIQUE INDEX Order_orderNumber_key ON "Order"(orderNumber);

CREATE TABLE Invoice (
    id TEXT NOT NULL PRIMARY KEY,
    orderId TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    lineItems TEXT NOT NULL,
    total REAL NOT NULL,
    currency TEXT NOT NULL DEFAULT 'EUR',
    issuedAt DATETIME NULL,
    dueAt DATETIME NULL,
    paidAt DATETIME NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Invoice_orderId_fkey FOREIGN KEY (orderId) REFERENCES "Order"(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE TABLE Report (
    id TEXT NOT NULL PRIMARY KEY,
    projectId TEXT NULL,
    assetId TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    authorId TEXT NULL,
    createdAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    updatedAt DATETIME NOT NULL,
    CONSTRAINT Report_projectId_fkey FOREIGN KEY (projectId) REFERENCES Project(id) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT Report_assetId_fkey FOREIGN KEY (assetId) REFERENCES Asset(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT Report_authorId_fkey FOREIGN KEY (authorId) REFERENCES User(id) ON DELETE SET NULL ON UPDATE CASCADE
);
