-- A team plays a season through a division entry (division_teams). Rosters and fixtures
-- hang off that entry rather than off the team, so a team can have neither in a division it
-- never entered, and one player is on at most one active team per division.
--
-- Brings an existing database to what 000001-init.sql defines for division_teams,
-- team_rosters and team_matches. Every step checks first: a no-op on a database just built
-- from init, and safe to run twice.

BEGIN;

CREATE TABLE IF NOT EXISTS division_teams (
	division_id  UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	team_id      UUID        NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (division_id, team_id)
);

CREATE INDEX IF NOT EXISTS division_teams_team_id_idx ON division_teams (team_id);

-- Enter every team into its home division, and every team with a fixture into that
-- fixture's division, so the foreign keys below find their rows.
INSERT INTO division_teams (division_id, team_id)
SELECT division_id, id FROM teams
UNION
SELECT division_id, home_team_id FROM team_matches
UNION
SELECT division_id, away_team_id FROM team_matches
ON CONFLICT DO NOTHING;

-- A roster row belongs to a team within a division: its team's home division.
ALTER TABLE team_rosters ADD COLUMN IF NOT EXISTS division_id UUID;

UPDATE team_rosters tr
SET division_id = t.division_id
FROM teams t
WHERE t.id = tr.team_id AND tr.division_id IS NULL;

ALTER TABLE team_rosters ALTER COLUMN division_id SET NOT NULL;

-- ADD CONSTRAINT has no IF NOT EXISTS.
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'team_rosters_division_team_fkey') THEN
		ALTER TABLE team_rosters ADD CONSTRAINT team_rosters_division_team_fkey
			FOREIGN KEY (division_id, team_id) REFERENCES division_teams (division_id, team_id) ON DELETE CASCADE;
	END IF;

	-- Both teams of a fixture must have entered its division. No ON DELETE: a team with a
	-- fixture can be neither withdrawn from the division nor permanently deleted.
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'team_matches_home_division_team_fkey') THEN
		ALTER TABLE team_matches ADD CONSTRAINT team_matches_home_division_team_fkey
			FOREIGN KEY (division_id, home_team_id) REFERENCES division_teams (division_id, team_id);
	END IF;
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'team_matches_away_division_team_fkey') THEN
		ALTER TABLE team_matches ADD CONSTRAINT team_matches_away_division_team_fkey
			FOREIGN KEY (division_id, away_team_id) REFERENCES division_teams (division_id, team_id);
	END IF;
END
$$;

-- Superseded by team_rosters_division_team_fkey: a team's deletion still reaches its roster
-- rows, through division_teams.
ALTER TABLE team_rosters DROP CONSTRAINT IF EXISTS team_rosters_team_id_fkey;

-- One active team per player per division. Fails, rolling everything back, if the data
-- already has a player active on two teams of one division: close one of the rows first.
CREATE UNIQUE INDEX IF NOT EXISTS team_rosters_player_id_division_id_active_key
	ON team_rosters (player_id, division_id) WHERE left_at IS NULL;

-- The lookups now go by division and team; the unique index above leads with player_id.
DROP INDEX IF EXISTS team_rosters_team_id_active_idx;
DROP INDEX IF EXISTS team_rosters_player_id_active_idx;
CREATE INDEX IF NOT EXISTS team_rosters_division_id_team_id_active_idx
	ON team_rosters (division_id, team_id) WHERE left_at IS NULL;

COMMIT;
