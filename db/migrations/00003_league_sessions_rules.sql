-- League session rules enforced by the sessions API, backed by the schema.
--
-- The repository maps these constraint names to API errors, so renaming one is a breaking change:
--   league_sessions_league_id_name_key   (23505) duplicate name
--   league_sessions_one_active_per_league (23505) second active session
--   league_sessions_check                (23514) ends_on before starts_on
--   league_sessions_name_length_check    (23514)
--   league_sessions_season_year_check    (23514)

BEGIN;

-- Existing rows keep their status; only new inserts that omit it start as drafts.
ALTER TABLE league_sessions ALTER COLUMN status SET DEFAULT 'DRAFT';

-- The API deactivates the league's other sessions in the same transaction first,
-- so this only fires on a bug or a manual write.
CREATE UNIQUE INDEX IF NOT EXISTS league_sessions_one_active_per_league
	ON league_sessions (league_id) WHERE is_active;

-- Dropped and re-added so the file can be re-run.
ALTER TABLE league_sessions DROP CONSTRAINT IF EXISTS league_sessions_name_length_check;
ALTER TABLE league_sessions ADD CONSTRAINT league_sessions_name_length_check
	CHECK (char_length(btrim(name)) BETWEEN 3 AND 60);

ALTER TABLE league_sessions DROP CONSTRAINT IF EXISTS league_sessions_season_year_check;
ALTER TABLE league_sessions ADD CONSTRAINT league_sessions_season_year_check
	CHECK (season_year BETWEEN 2000 AND 2100);

COMMIT;
