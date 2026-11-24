-- A player's member id is assigned by hand, so a player may exist before it has one:
-- member_id defaults to '' until then. Brings an existing database to what 000001-init.sql
-- has defined since 241bc8d, which changed the column without a migration.
--
-- The old UNIQUE (member_id) constraint allows only one ''. It is replaced by a partial
-- unique index of the same name, so an assigned id is still never reused.
--
-- A no-op on a database built from the current 000001-init.sql, and safe to run twice.

BEGIN;

ALTER TABLE players ALTER COLUMN member_id SET DEFAULT '';

-- Drops the old constraint and its index. On a migrated database the name belongs only to
-- the partial index below, which is not a constraint, so this skips it.
ALTER TABLE players DROP CONSTRAINT IF EXISTS players_member_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS players_member_id_key
	ON players (member_id) WHERE member_id <> '';

COMMIT;
