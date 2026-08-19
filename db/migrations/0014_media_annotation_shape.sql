ALTER TABLE MediaAnnotationRegion ADD COLUMN shape TEXT NOT NULL DEFAULT 'rect';
ALTER TABLE MediaAnnotationRegion ADD COLUMN pathData TEXT;
