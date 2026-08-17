-- Tags (free-form labels) and Materials-as-a-relation are out of scope for the design artifact.
-- Neither is referenced by a FOREIGN KEY from any other table (Tag/TagAssignment use the
-- polymorphic taggableType/taggableId pattern, not a real FK; AssetMaterial isn't a FK target
-- either), so straight DROPs are safe here -- no table rebuild needed.
DROP TABLE TagAssignment;
DROP TABLE Tag;
DROP TABLE AssetMaterial;
