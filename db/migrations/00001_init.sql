-- Required for uuid_generate_v4().
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

BEGIN;


-- -----------------------------------------------------------------------------
-- League structure
-- -----------------------------------------------------------------------------

-- User identity is managed by the Google login upsert.
-- id is TEXT holding a uuid_generate_v4() value, so auth.User.ID and auth.Session.UserID
-- are Go strings. Every FK into this table is therefore TEXT too — a BIGINT one cannot be
-- created at all ("incompatible types: bigint and text").
CREATE TABLE IF NOT EXISTS users (
	id              TEXT        PRIMARY KEY DEFAULT uuid_generate_v4(),
	google_sub      TEXT        NOT NULL UNIQUE,  -- Google login upsert key
	email           TEXT        NOT NULL,
	email_verified  BOOLEAN     NOT NULL DEFAULT false,
	name            TEXT        NOT NULL,
	avatar_url      TEXT        NOT NULL DEFAULT '',
	hd              TEXT,                         -- Google Workspace domain
	created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_login_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Server-side sessions expire by absolute age and inactivity.
CREATE TABLE IF NOT EXISTS users_sessions (
	id            TEXT        PRIMARY KEY DEFAULT uuid_generate_v4(),
	user_id       TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports logout-everywhere and session-expiry lookups.
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON users_sessions (user_id);

-- Hierarchy: League -> Season -> Division -> Team -> Roster.
-- Timezone controls league-local scheduling and season boundaries.
CREATE TABLE IF NOT EXISTS leagues (
	id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	name           TEXT        NOT NULL,
	description    TEXT,
	location       TEXT,
	contact_email  TEXT,
	contact_phone  TEXT,
	timezone       TEXT        NOT NULL DEFAULT 'UTC',
	status         TEXT        NOT NULL DEFAULT 'ACTIVE'
		CHECK (status IN ('ACTIVE', 'INACTIVE', 'ARCHIVED')),
	created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Maps users to the leagues they operate; RBAC defines permissions,
-- while this table defines their league scope.
CREATE TABLE IF NOT EXISTS league_operators (
	league_id   UUID        NOT NULL REFERENCES leagues (id) ON DELETE CASCADE,
	user_id     TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	granted_by  TEXT        REFERENCES users (id),  -- audit trail
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (league_id, user_id)
);

-- Supports reverse lookups by user.
CREATE INDEX IF NOT EXISTS league_operators_user_id_idx ON league_operators (user_id);

CREATE TABLE IF NOT EXISTS league_sessions (
	id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	league_id   UUID        NOT NULL REFERENCES leagues (id) ON DELETE CASCADE,
	name        TEXT        NOT NULL,  -- e.g. 'Fall 2026'
	session_type  TEXT      NOT NULL
		CHECK (session_type IN ('SPRING', 'SUMMER', 'FALL')),
	season_year   SMALLINT  NOT NULL,  -- Explicit because seasons may cross calendar years
	starts_on   DATE        NOT NULL,
	ends_on     DATE        NOT NULL,
	status      TEXT        NOT NULL DEFAULT 'IN_PROGRESS'
		CHECK (status IN ('DRAFT', 'IN_PROGRESS', 'COMPLETE', 'CANCELLED')),
	is_active   BOOLEAN     NOT NULL DEFAULT false,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (league_id, name),
	CHECK (ends_on >= starts_on)
);

CREATE TABLE IF NOT EXISTS divisions (
	id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	season_id        UUID        NOT NULL REFERENCES league_sessions (id) ON DELETE CASCADE,
	name             TEXT        NOT NULL,             -- e.g. '8-Ball Open'
	format           TEXT        NOT NULL DEFAULT '8-ball',
	skill_level_cap  SMALLINT    NOT NULL DEFAULT 23,  -- 23-Rule
	created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (season_id, name)
);

CREATE TABLE IF NOT EXISTS venues (
	id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	league_id   UUID        REFERENCES leagues (id) ON DELETE SET NULL,
	name        TEXT        NOT NULL,
	address     TEXT,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS players (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	-- Optional account link; most players never sign in.
	user_id      TEXT        UNIQUE REFERENCES users (id) ON DELETE SET NULL,
	member_id    TEXT        NOT NULL UNIQUE,         -- External membership ID
	full_name    TEXT        NOT NULL,
	skill_level  SMALLINT    NOT NULL DEFAULT 3 CHECK (skill_level BETWEEN 2 AND 7),
	is_paid      BOOLEAN     NOT NULL DEFAULT false,  -- Captains may add only paid members
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teams (
	id                 UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	division_id        UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	team_number        TEXT        NOT NULL,  -- External team number
	name               TEXT        NOT NULL,
	captain_player_id  UUID        REFERENCES players (id),
	venue_id           UUID        REFERENCES venues (id) ON DELETE SET NULL,
	created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (division_id, team_number)
);

-- Historical roster membership. Transfers close the old row and create a new
-- one, preserving match history. Eligibility rules are enforced by the service layer.
CREATE TABLE IF NOT EXISTS team_rosters (
	id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	team_id     UUID        NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
	player_id   UUID        NOT NULL REFERENCES players (id) ON DELETE CASCADE,
	joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	left_at     TIMESTAMPTZ,  -- NULL means currently active
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (team_id, player_id, joined_at),
	CHECK (left_at IS NULL OR left_at > joined_at)
);

-- Supports lookups of a team's current players and a player's current team.
CREATE INDEX IF NOT EXISTS team_rosters_team_id_active_idx
	ON team_rosters (team_id) WHERE left_at IS NULL;

CREATE INDEX IF NOT EXISTS team_rosters_player_id_active_idx
	ON team_rosters (player_id) WHERE left_at IS NULL;

-- Audit trail for skill-level changes.
CREATE TABLE IF NOT EXISTS skill_level_history (
	id               BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	player_id        UUID        NOT NULL REFERENCES players (id) ON DELETE CASCADE,
	old_skill_level  SMALLINT    NOT NULL,
	new_skill_level  SMALLINT    NOT NULL CHECK (new_skill_level BETWEEN 2 AND 7),
	reason           TEXT,
	changed_by       TEXT        REFERENCES users (id),
	changed_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- -----------------------------------------------------------------------------
-- Rules data
-- -----------------------------------------------------------------------------

-- APA Equalizer lookup: race_a and race_b are games required by each player.
CREATE TABLE IF NOT EXISTS race_chart (
	skill_level_a  SMALLINT NOT NULL CHECK (skill_level_a BETWEEN 2 AND 7),
	skill_level_b  SMALLINT NOT NULL CHECK (skill_level_b BETWEEN 2 AND 7),
	race_a         SMALLINT NOT NULL,
	race_b         SMALLINT NOT NULL,

	PRIMARY KEY (skill_level_a, skill_level_b)
);

-- Race values are symmetric: (a,b) -> (x,y) implies (b,a) -> (y,x).
INSERT INTO race_chart (skill_level_a, skill_level_b, race_a, race_b) VALUES
	(2, 2, 2, 2), (2, 3, 2, 3), (2, 4, 2, 4), (2, 5, 2, 5), (2, 6, 2, 6), (2, 7, 2, 7),
	(3, 2, 3, 2), (3, 3, 2, 2), (3, 4, 2, 3), (3, 5, 2, 4), (3, 6, 2, 5), (3, 7, 2, 6),
	(4, 2, 4, 2), (4, 3, 3, 2), (4, 4, 3, 3), (4, 5, 3, 4), (4, 6, 3, 5), (4, 7, 2, 5),
	(5, 2, 5, 2), (5, 3, 4, 2), (5, 4, 4, 3), (5, 5, 4, 4), (5, 6, 4, 5), (5, 7, 3, 5),
	(6, 2, 6, 2), (6, 3, 5, 2), (6, 4, 5, 3), (6, 5, 5, 4), (6, 6, 5, 5), (6, 7, 4, 5),
	(7, 2, 7, 2), (7, 3, 6, 2), (7, 4, 5, 2), (7, 5, 5, 3), (7, 6, 5, 4), (7, 7, 5, 5)
ON CONFLICT (skill_level_a, skill_level_b) DO NOTHING;


-- -----------------------------------------------------------------------------
-- Matches
-- -----------------------------------------------------------------------------

-- One row per team night, consisting of five individual matches.
CREATE TABLE IF NOT EXISTS team_matches (
	id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	division_id   UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	week_no       SMALLINT    NOT NULL,
	home_team_id  UUID        NOT NULL REFERENCES teams (id),
	away_team_id  UUID        NOT NULL REFERENCES teams (id),
	venue_id      UUID        REFERENCES venues (id),
	scheduled_at  TIMESTAMPTZ NOT NULL,
	status        VARCHAR(20) NOT NULL DEFAULT 'scheduled',
	home_points   SMALLINT    NOT NULL DEFAULT 0,  -- Maintained by trigger
	away_points   SMALLINT    NOT NULL DEFAULT 0,
	submitted_by  TEXT        REFERENCES users (id),
	submitted_at  TIMESTAMPTZ,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

	CHECK (home_team_id <> away_team_id)
);

CREATE INDEX IF NOT EXISTS team_matches_division_id_week_no_idx
	ON team_matches (division_id, week_no);

-- Snapshot skill levels and race values at put-up to preserve match history.
CREATE TABLE IF NOT EXISTS individual_matches (
	id                UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	team_match_id     UUID        NOT NULL REFERENCES team_matches (id) ON DELETE CASCADE,
	match_order       SMALLINT    NOT NULL CHECK (match_order BETWEEN 1 AND 5),
	first_put_up      TEXT        NOT NULL,  -- home in match 1, then alternate
	home_player_id    UUID        REFERENCES players (id),
	away_player_id    UUID        REFERENCES players (id),
	home_skill_level  SMALLINT,
	away_skill_level  SMALLINT,
	home_race         SMALLINT,
	away_race         SMALLINT,
	home_games        SMALLINT    NOT NULL DEFAULT 0 CHECK (home_games >= 0),
	away_games        SMALLINT    NOT NULL DEFAULT 0 CHECK (away_games >= 0),
	innings           SMALLINT    CHECK (innings >= 0),
	home_def_shots    SMALLINT    NOT NULL DEFAULT 0 CHECK (home_def_shots >= 0),
	away_def_shots    SMALLINT    NOT NULL DEFAULT 0 CHECK (away_def_shots >= 0),
	home_points       SMALLINT    NOT NULL DEFAULT 0,
	away_points       SMALLINT    NOT NULL DEFAULT 0,
	status            TEXT        NOT NULL DEFAULT 'pending'
		CHECK (status IN ('pending', 'complete')),
	entered_by        TEXT        REFERENCES users (id),
	updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (team_match_id, match_order)
);

CREATE INDEX IF NOT EXISTS individual_matches_home_player_id_idx
	ON individual_matches (home_player_id);

CREATE INDEX IF NOT EXISTS individual_matches_away_player_id_idx
	ON individual_matches (away_player_id);


-- -----------------------------------------------------------------------------
-- Aggregates
-- -----------------------------------------------------------------------------

-- Denormalised rollup of individual_matches, one row per player per division, which
-- is what the roster screen reads. Counters are INTEGER rather than the SMALLINT used
-- for per-match values: these are season sums, and a rollup is where a silent overflow
-- would do the most damage.
CREATE TABLE IF NOT EXISTS players_stats (
	player_id        UUID        NOT NULL REFERENCES players (id) ON DELETE CASCADE,
	division_id      UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	matches_played   INTEGER     NOT NULL DEFAULT 0 CHECK (matches_played >= 0),
	matches_won      INTEGER     NOT NULL DEFAULT 0 CHECK (matches_won >= 0),
	total_points     INTEGER     NOT NULL DEFAULT 0 CHECK (total_points >= 0),
	games_won        INTEGER     NOT NULL DEFAULT 0 CHECK (games_won >= 0),
	games_lost       INTEGER     NOT NULL DEFAULT 0 CHECK (games_lost >= 0),
	innings          INTEGER     NOT NULL DEFAULT 0 CHECK (innings >= 0),
	defensive_shots  INTEGER     NOT NULL DEFAULT 0 CHECK (defensive_shots >= 0),
	shutouts         INTEGER     NOT NULL DEFAULT 0 CHECK (shutouts >= 0),
	last_played_at   TIMESTAMPTZ,  -- NULL before first match
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (player_id, division_id),

	-- Cross-column invariants: these are what catch a broken recompute, since every
	-- counter above would still pass its own >= 0 check.
	CHECK (matches_won <= matches_played),
	CHECK (shutouts <= matches_won)
);

-- Supports statistics lookups by division.
CREATE INDEX IF NOT EXISTS players_stats_division_id_idx
	ON players_stats (division_id);


-- -----------------------------------------------------------------------------
-- Authorization (RBAC)
-- -----------------------------------------------------------------------------

-- Role and permission codes are authorization contracts: never rename them.
-- Display names can change freely.
CREATE TABLE IF NOT EXISTS roles (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	code         VARCHAR(50) NOT NULL UNIQUE,
	name         TEXT        NOT NULL,
	description  TEXT,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS permissions (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	code         VARCHAR(100) NOT NULL UNIQUE,
	name         TEXT        NOT NULL,
	description  TEXT,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Both sides CASCADE: a deleted role or permission must not leave a row here that a
-- permission check would still read as a grant.
CREATE TABLE IF NOT EXISTS roles_permissions (
	role_id        UUID        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	permission_id  UUID        NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
	created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS users_roles (
	user_id     TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	role_id     UUID        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	granted_by  TEXT        REFERENCES users (id),  -- audit trail
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (user_id, role_id)
);

-- Supports reverse lookups by permission and role.
CREATE INDEX IF NOT EXISTS roles_permissions_permission_id_idx
	ON roles_permissions (permission_id);

CREATE INDEX IF NOT EXISTS users_roles_role_id_idx
	ON users_roles (role_id);


-- -----------------------------------------------------------------------------
-- Authorization seed data
-- -----------------------------------------------------------------------------

INSERT INTO roles (code, name, description) VALUES
	('PLAYER',           'Player',                  'Reads the divisions and players they play against.'),
	('CAPTAIN',          'Team Captain',            'Runs a team: puts players up and enters scores for their own team nights.'),
	('SUPER_ADMIN',      'Super Admin',             'Manages the whole platform, across every league.')
ON CONFLICT (code) DO NOTHING;

-- Permission codes follow [entity].[action] and are authorization contracts.
-- Keep the full catalog here; use dedicated codes for non-CRUD operations.
INSERT INTO permissions (code, name, description) VALUES
	('league.read',               'View leagues',          'See a league and its settings.'),
	('league.create',             'Create leagues',        'Create a new league.'),
	('league.update',             'Update leagues',        'Edit league settings, contact details and timezone.'),
	('league.delete',             'Delete leagues',        'Archive or remove a league.'),

	('session.read',              'View sessions',         'See the sessions in a league and their dates.'),
	('session.create',            'Create sessions',       'Open a new session in a league.'),
	('session.update',            'Update sessions',       'Edit session dates, type and status.'),
	('session.delete',            'Delete sessions',       'Remove a session.'),

	('division.read',             'View divisions',        'See the divisions in a session and their formats.'),
	('division.create',           'Create divisions',      'Add a division to a session.'),
	('division.update',           'Update divisions',      'Edit division format, match day and skill-level cap.'),
	('division.delete',           'Delete divisions',      'Remove a division from a session.'),

	('venue.read',                'View venues',           'See the host locations a league plays at.'),
	('venue.create',              'Create venues',         'Add a host location.'),
	('venue.update',              'Update venues',         'Edit the name and address of a host location.'),
	('venue.delete',              'Delete venues',         'Remove a host location.'),

	('team.read',                 'View teams',            'See the teams in a division.'),
	('team.create',               'Create teams',          'Register a team into a division.'),
	('team.update',               'Update teams',          'Edit team name, number, captain and home room.'),
	('team.delete',               'Delete teams',          'Remove a team from a division.'),

	('player.read',               'View players',          'See player profiles and skill levels.'),
	('player.create',             'Create players',        'Register a new player.'),
	('player.update',             'Update players',        'Edit player profiles and membership details.'),
	('player.delete',             'Delete players',        'Remove a player record.'),
	('player.change_skill_level', 'Change a skill level',  'Adjust a skill level and write the history row.'),

	('roster.read',               'View rosters',          'See which players are on a team.'),
	('roster.create',             'Create rosters',        'Add a player to a team roster.'),
	('roster.update',             'Update rosters',        'Edit a roster entry.'),
	('roster.delete',             'Delete rosters',        'Remove a player from a roster.'),

	('match.read',                'View matches',          'See scheduled and completed matches and their scores.'),
	('match.create',              'Create matches',        'Schedule a team match.'),
	('match.update',              'Update matches',        'Put players up and record games, innings and defensive shots.'),
	('match.delete',              'Delete matches',        'Remove a scheduled match.'),

	('user.read',                 'View users',            'See the accounts that can sign in.'),
	('user.create',               'Create users',          'Create an account outside the Google sign-in flow.'),
	('user.update',               'Update users',          'Edit an account.'),
	('user.delete',               'Delete users',          'Remove an account.'),

	('role.read',                 'View roles',            'See roles and the permissions they grant.'),
	('role.create',               'Create roles',          'Define a new role.'),
	('role.update',               'Update roles',          'Change which permissions a role grants.'),
	('role.delete',               'Delete roles',          'Remove a role.')
ON CONFLICT (code) DO NOTHING;


-- SUPER_ADMIN receives all permissions; rerun this grant after adding permissions.
-- Every seed INSERT is ON CONFLICT DO NOTHING, so this whole file is re-runnable.
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- All signed-in player roles can read the league structure.
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
	'league.read', 'session.read', 'division.read', 'venue.read',
	'team.read', 'player.read', 'roster.read', 'match.read')
WHERE r.code IN ('PLAYER', 'CAPTAIN')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Explicit code-based grants keep the role-permission matrix reviewable.
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (r.code, p.code) IN (
	('CAPTAIN', 'roster.create'),
	('CAPTAIN', 'roster.update'),
	('CAPTAIN', 'roster.delete'),
	('CAPTAIN', 'match.create'),
	('CAPTAIN', 'match.update')
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
