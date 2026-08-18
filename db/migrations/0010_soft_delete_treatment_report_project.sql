-- "Unlink" on the asset detail view's Treatments/Reports/Projects sections can't reassign these
-- rows to "no asset" (assetId is NOT NULL on all three, unlike Media's many-to-many
-- MediaReference join), so unlinking soft-deletes instead: the row is hidden from every query but
-- stays in the database. Plain ADD COLUMN — no table rebuild needed, unlike the FK-removal
-- rebuilds in 0004/0006, since this is just a new nullable column.
ALTER TABLE Treatment ADD COLUMN deletedAt DATETIME NULL;
ALTER TABLE Report ADD COLUMN deletedAt DATETIME NULL;
ALTER TABLE Project ADD COLUMN deletedAt DATETIME NULL;
