-- Generic key/value store for admin-configurable behavior that isn't a Classifier picklist:
-- the Dashboard's per-module list caps and the Reports "reportable field" checkboxes (see
-- internal/settings' appsetting.go). One simple table rather than a bespoke one for each concern.
CREATE TABLE AppSetting (
    id TEXT NOT NULL PRIMARY KEY,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    updatedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now'))
);
CREATE UNIQUE INDEX AppSetting_key_key ON AppSetting(key);
