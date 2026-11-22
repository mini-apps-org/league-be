-- Soft delete for roles: DELETE /roles/{id} sets is_deleted instead of removing the row,
-- and a permanent delete removes only a role already marked. Every permission lookup
-- must join roles on NOT is_deleted, so a soft-deleted role stops granting at once.
--
-- 000001-init.sql has the column for a fresh database; this brings an existing one up to
-- date, and changes nothing on a database just built from init.

BEGIN;

ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false;

COMMIT;
