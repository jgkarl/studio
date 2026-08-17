-- Reports gain structured sections (replacing the freeform TipTap doc going forward) and a
-- customize-layout sidebar. `content` (the old TipTap JSON) is kept, unused by new reports, same
-- "don't lose old data" reasoning used elsewhere in this app.
ALTER TABLE Report ADD COLUMN summary TEXT NULL;
ALTER TABLE Report ADD COLUMN conditionFindings TEXT NULL;
ALTER TABLE Report ADD COLUMN treatmentPerformed TEXT NULL;
ALTER TABLE Report ADD COLUMN materialsUsed TEXT NULL;
ALTER TABLE Report ADD COLUMN recommendations TEXT NULL;
ALTER TABLE Report ADD COLUMN coverMediaId TEXT NULL REFERENCES Media(id) ON DELETE SET NULL;
ALTER TABLE Report ADD COLUMN layoutStyle TEXT NOT NULL DEFAULT 'standard';
ALTER TABLE Report ADD COLUMN showCover INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN showSummary INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN showCondition INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN showTreatment INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN showMaterials INTEGER NOT NULL DEFAULT 1;
ALTER TABLE Report ADD COLUMN showRecommendations INTEGER NOT NULL DEFAULT 1;
