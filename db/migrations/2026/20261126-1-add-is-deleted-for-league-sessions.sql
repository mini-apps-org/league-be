-- Soft delete for league sessions: DELETE /sessions/{id} sets is_deleted and clears
-- is_active instead of removing the row, and DELETE /sessions/{id}/permanent removes only
-- a session already marked. A soft-deleted session keeps its name, so the name never
-- names two sessions while the first still exists.
--
-- 000001-init.sql has the column for a fresh database; this brings an existing one up to
-- date, and changes nothing on a database just built from init.

BEGIN;

ALTER TABLE league_sessions ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false;

COMMIT;
