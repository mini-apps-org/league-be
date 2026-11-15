-- One session per user: auth.ReplaceSession upserts with ON CONFLICT (user_id),
-- which Postgres rejects unless user_id has a unique index.

BEGIN;

-- Keep each user's most recently used session; id breaks a last_seen_at tie.
DELETE FROM users_sessions older
USING users_sessions newer
WHERE older.user_id = newer.user_id
  AND (older.last_seen_at, older.id) < (newer.last_seen_at, newer.id);

CREATE UNIQUE INDEX IF NOT EXISTS users_sessions_user_id_key ON users_sessions (user_id);

-- Covered by the unique index above.
DROP INDEX IF EXISTS sessions_user_id_idx;

COMMIT;
