-- One session per user: auth.ReplaceSession upserts with ON CONFLICT (user_id), which
-- Postgres rejects unless user_id has a unique index. A database built from
-- 000001-init.sql between 241bc8d and this migration got a plain index instead, so every
-- Google sign-in on it failed.
--
-- A no-op on a database that ran the old 00002 migration or the current 000001-init.sql,
-- and safe to run twice.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS users_sessions_user_id_key ON users_sessions (user_id);

-- Covered by the unique index above.
DROP INDEX IF EXISTS sessions_user_id_idx;

COMMIT;
