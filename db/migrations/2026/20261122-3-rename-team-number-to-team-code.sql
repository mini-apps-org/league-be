-- teams.team_number becomes team_code, matching the ID Generation Rules' name for it (XXyyA)
-- and the API field. Constraints named after the column are renamed with it: a column
-- rename keeps their old names, while 000001-init.sql creates the new ones, and the code
-- maps these names to a 409 — a database left with the old name would turn a duplicate
-- team code into a 500.
--
-- RENAME has no IF EXISTS, so each step checks first: a no-op on a database just built
-- from init, and safe to run twice.

BEGIN;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'teams' AND column_name = 'team_number'
	) THEN
		ALTER TABLE teams RENAME COLUMN team_number TO team_code;
	END IF;

	-- unique within a division (000001-init.sql)
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'teams_division_id_team_number_key') THEN
		ALTER TABLE teams RENAME CONSTRAINT teams_division_id_team_number_key TO teams_division_id_team_code_key;
	END IF;

	-- Postgres 18 names NOT NULL constraints after the column too
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'teams_team_number_not_null') THEN
		ALTER TABLE teams RENAME CONSTRAINT teams_team_number_not_null TO teams_team_code_not_null;
	END IF;

	-- unique across the league, and its format check: only on databases that ran the
	-- removed permanent-teams migration
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'teams_team_number_key') THEN
		ALTER TABLE teams RENAME CONSTRAINT teams_team_number_key TO teams_team_code_key;
	END IF;
	IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'teams_team_number_format_check') THEN
		ALTER TABLE teams RENAME CONSTRAINT teams_team_number_format_check TO teams_team_code_format_check;
	END IF;
END
$$;

COMMIT;
