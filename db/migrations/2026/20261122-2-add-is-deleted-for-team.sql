-- Soft delete for teams: DELETE /teams/{id} sets is_deleted instead of removing the row,
-- and a permanent delete removes only a team already marked. A soft-deleted team keeps
-- its team code, so the number never names two teams while the first still exists.
--
-- 000001-init.sql has the column for a fresh database; this brings an existing one up to
-- date, and changes nothing on a database just built from init.

BEGIN;

ALTER TABLE teams ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false;

COMMIT;
