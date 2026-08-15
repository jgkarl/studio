-- Defensive backfill: the app is dropping its use of Project.stage (the 7-stage workflow state
-- machine) in favor of the already-existing Project.completedAt as the sole open/closed signal.
-- AdvanceStage always set completedAt when a project first reached the 'handover_done' stage, so
-- this should be a no-op in practice - it just guarantees no already-finished project is left
-- looking "open" once the app stops reading stage at all.
UPDATE Project SET completedAt = updatedAt WHERE stage = 'handover_done' AND completedAt IS NULL;
