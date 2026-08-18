-- Estonian variant of a classifier's title/description. NULL falls back to the (English) title/
-- description everywhere a classifier label is rendered — see internal/settings's ClassifierLabel().
ALTER TABLE Classifier ADD COLUMN titleEt TEXT NULL;
ALTER TABLE Classifier ADD COLUMN descriptionEt TEXT NULL;
