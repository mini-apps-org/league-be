-- uuid_generate_v4() is provided by uuid-ossp, unlike the built-in
-- gen_random_uuid() it replaces.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

BEGIN;


-- -----------------------------------------------------------------------------
-- League structure
-- -----------------------------------------------------------------------------

-- Identity, written only by the Google login upsert in internal/auth/repositories.go.
-- id is TEXT holding a uuid_generate_v4() value, so auth.User.ID and auth.Session.UserID
-- are Go strings. Every FK into this table is therefore TEXT too — a BIGINT one cannot be
-- created at all ("incompatible types: bigint and text").
CREATE TABLE users (
	id              TEXT        PRIMARY KEY DEFAULT uuid_generate_v4(),
	google_sub      TEXT        NOT NULL UNIQUE,  -- the ON CONFLICT target of the login upsert
	email           TEXT        NOT NULL,
	email_verified  BOOLEAN     NOT NULL DEFAULT false,
	name            TEXT        NOT NULL,
	avatar_url      TEXT        NOT NULL DEFAULT '',
	hd              TEXT,                         -- Google Workspace domain, absent for consumer accounts
	created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_login_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Server-side sessions; the id is the value of the HttpOnly `sid` cookie. The repository
-- expires rows by both absolute age (created_at) and idleness (last_seen_at), so neither
-- column is decorative.
CREATE TABLE sessions (
	id            TEXT        PRIMARY KEY DEFAULT uuid_generate_v4(),
	user_id       TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Logout-everywhere and the expiry sweep both filter by user_id.
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE seasons (
	id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	name        TEXT        NOT NULL,  -- 'Fall 2026'
	starts_on   DATE        NOT NULL,
	ends_on     DATE        NOT NULL,
	is_active   BOOLEAN     NOT NULL DEFAULT false,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE divisions (
	id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	season_id        UUID        NOT NULL REFERENCES seasons (id) ON DELETE CASCADE,
	name             TEXT        NOT NULL,             -- '8-Ball Open'
	format           TEXT        NOT NULL DEFAULT '8-ball',
	skill_level_cap  SMALLINT    NOT NULL DEFAULT 23,  -- 23-Rule
	created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (season_id, name)
);

CREATE TABLE venues (
	id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	name        TEXT        NOT NULL,
	address     TEXT,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE players (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	-- NULL until they sign in; most players never do.
	user_id      TEXT        UNIQUE REFERENCES users (id) ON DELETE SET NULL,
	member_id    TEXT        NOT NULL UNIQUE,         -- '95007620'
	full_name    TEXT        NOT NULL,
	skill_level  SMALLINT    NOT NULL DEFAULT 3 CHECK (skill_level BETWEEN 2 AND 7),
	is_paid      BOOLEAN     NOT NULL DEFAULT false,  -- dues paid; captain may add only paid members
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE teams (
	id                 UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	division_id        UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	team_number        TEXT        NOT NULL,  -- '03209'
	name               TEXT        NOT NULL,
	captain_player_id  UUID        REFERENCES players (id),
	created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

	UNIQUE (division_id, team_number)
);

-- Every skill level change, for audits and disputes.
CREATE TABLE skill_level_history (
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

-- Equalizer "Games Must Win": race_a is games needed by the skill_level_a player.
CREATE TABLE race_chart (
	skill_level_a  SMALLINT NOT NULL CHECK (skill_level_a BETWEEN 2 AND 7),
	skill_level_b  SMALLINT NOT NULL CHECK (skill_level_b BETWEEN 2 AND 7),
	race_a         SMALLINT NOT NULL,
	race_b         SMALLINT NOT NULL,

	PRIMARY KEY (skill_level_a, skill_level_b)
);

-- Rows are symmetric: (a,b) -> (x,y) implies (b,a) -> (y,x).
INSERT INTO race_chart (skill_level_a, skill_level_b, race_a, race_b) VALUES
	(2, 2, 2, 2), (2, 3, 2, 3), (2, 4, 2, 4), (2, 5, 2, 5), (2, 6, 2, 6), (2, 7, 2, 7),
	(3, 2, 3, 2), (3, 3, 2, 2), (3, 4, 2, 3), (3, 5, 2, 4), (3, 6, 2, 5), (3, 7, 2, 6),
	(4, 2, 4, 2), (4, 3, 3, 2), (4, 4, 3, 3), (4, 5, 3, 4), (4, 6, 3, 5), (4, 7, 2, 5),
	(5, 2, 5, 2), (5, 3, 4, 2), (5, 4, 4, 3), (5, 5, 4, 4), (5, 6, 4, 5), (5, 7, 3, 5),
	(6, 2, 6, 2), (6, 3, 5, 2), (6, 4, 5, 3), (6, 5, 5, 4), (6, 6, 5, 5), (6, 7, 4, 5),
	(7, 2, 7, 2), (7, 3, 6, 2), (7, 4, 5, 2), (7, 5, 5, 3), (7, 6, 5, 4), (7, 7, 5, 5);


-- -----------------------------------------------------------------------------
-- Matches
-- -----------------------------------------------------------------------------

-- One row per team night (5 individual matches).
CREATE TABLE team_matches (
	id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	division_id   UUID        NOT NULL REFERENCES divisions (id) ON DELETE CASCADE,
	week_no       SMALLINT    NOT NULL,
	home_team_id  UUID        NOT NULL REFERENCES teams (id),
	away_team_id  UUID        NOT NULL REFERENCES teams (id),
	venue_id      UUID        REFERENCES venues (id),
	scheduled_at  TIMESTAMPTZ NOT NULL,
	status        VARCHAR(20) NOT NULL DEFAULT 'scheduled',
	home_points   SMALLINT    NOT NULL DEFAULT 0,  -- kept in sync by trigger
	away_points   SMALLINT    NOT NULL DEFAULT 0,
	submitted_by  TEXT        REFERENCES users (id),
	submitted_at  TIMESTAMPTZ,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

	CHECK (home_team_id <> away_team_id)
);

CREATE INDEX team_matches_division_id_week_no_idx
	ON team_matches (division_id, week_no);

-- One row per player-vs-player match. Skill level and race are snapshots taken
-- at put-up, so a later admin skill level change never alters this match.
CREATE TABLE individual_matches (
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

CREATE INDEX individual_matches_home_player_id_idx
	ON individual_matches (home_player_id);

CREATE INDEX individual_matches_away_player_id_idx
	ON individual_matches (away_player_id);


-- -----------------------------------------------------------------------------
-- Aggregates
-- -----------------------------------------------------------------------------

-- Denormalised rollup of individual_matches: one row per player per division,
-- which is what the roster screen reads. Counters are INTEGER rather than the
-- SMALLINT used for per-match values — these are sums over a whole season, and
-- a rollup is where a silent overflow would do the most damage.
CREATE TABLE players_stats (
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
	last_played_at   TIMESTAMPTZ,  -- NULL until the first match
	updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (player_id, division_id),

	-- Cross-column invariants: these are what catch a broken recompute, since
	-- every counter above would still pass its own >= 0 check.
	CHECK (matches_won <= matches_played),
	CHECK (shutouts <= matches_won)
);

-- The primary key leads with player_id, so a roster query filtering on
-- division alone cannot use it.
CREATE INDEX players_stats_division_id_idx
	ON players_stats (division_id);


-- -----------------------------------------------------------------------------
-- Authorization (RBAC)
-- -----------------------------------------------------------------------------

-- `code` is the machine-readable identity that authorization code compares
-- against, so it is an API contract: add codes, never rename one. `name` is the
-- display label and may change freely.
CREATE TABLE roles (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	code         VARCHAR(50) NOT NULL UNIQUE,  -- 'CAPTAIN'
	name         TEXT        NOT NULL,  -- 'Captain'
	description  TEXT,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
	id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
	code         VARCHAR(100) NOT NULL UNIQUE,  -- 'match.submit'
	name         TEXT        NOT NULL,
	description  TEXT,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Both sides CASCADE: a deleted role or permission must not leave a row here
-- that a permission check would still read as a grant.
CREATE TABLE roles_permissions (
	role_id        UUID        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	permission_id  UUID        NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
	created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users_roles (
	user_id     TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	role_id     UUID        NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	granted_by  TEXT        REFERENCES users (id),  -- who assigned it, for audits
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

	PRIMARY KEY (user_id, role_id)
);

-- Each primary key leads with the left-hand column, so the reverse lookups
-- ("which roles grant X", "who holds role Y") need their own indexes.
CREATE INDEX roles_permissions_permission_id_idx
	ON roles_permissions (permission_id);

CREATE INDEX users_roles_role_id_idx
	ON users_roles (role_id);


-- -----------------------------------------------------------------------------
-- Authorization seed data
-- -----------------------------------------------------------------------------

INSERT INTO roles (code, name, description) VALUES
	('PLAYER',  'Player',  'Reads the divisions and players they play against.'),
	('CAPTAIN', 'Captain', 'Runs a team: puts players up and enters scores for their own team nights.'),
	('ADMIN',   'Admin',   'Full access, including skill-level changes.');

INSERT INTO permissions (code, name, description) VALUES
	('player.read',               'Read players',          'See player profiles and skill levels.'),
	('match.put_up',              'Put up a player',       'Nominate a player for the next individual match.'),
	('match.enter_score',         'Enter a score',         'Record games, innings and defensive shots.'),
	('match.submit',              'Submit a team night',   'Close a team match and lock its scores.'),
	('roster.manage',             'Manage a roster',       'Add or deactivate players on a team.'),
	('player.change_skill_level', 'Change a skill level',  'Adjust a skill level and write the history row.'),
	('season.manage',             'Manage seasons',        'Create seasons, divisions and venues.');

-- ADMIN holds everything, including permissions a later migration adds — but
-- only if that migration re-runs this grant, so keep them together.
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code = 'ADMIN';

-- PLAYER and CAPTAIN are granted explicitly, by code rather than by id, so the
-- seed does not depend on generated uuids.
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (r.code, p.code) IN (
	('PLAYER',  'player.read'),
	('CAPTAIN', 'player.read'),
	('CAPTAIN', 'match.put_up'),
	('CAPTAIN', 'match.enter_score'),
	('CAPTAIN', 'match.submit'),
	('CAPTAIN', 'roster.manage')
);

COMMIT;
