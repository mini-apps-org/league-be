-- Development seed. Applied by `make seed`, not part of the schema.
--
-- Natural keys (league name, session name, team number, member id) are used for every lookup
-- so the file never depends on generated uuids and can be re-read top to bottom.

BEGIN;


INSERT INTO leagues (name, description, location, contact_email, timezone)
VALUES ('MVIP League', 'Development league', 'Ho Chi Minh City', 'ops@mvip-league.com', 'Asia/Ho_Chi_Minh');

-- Fall 2026 runs Sep -> Dec, so season_year and the dates agree here; a session that
-- crossed into January would still carry season_year 2026.
INSERT INTO league_sessions (league_id, name, session_type, season_year, starts_on, ends_on, is_active)
SELECT l.id, 'Fall 2026', 'FALL', 2026, '2026-09-01', '2026-12-20', true
FROM leagues l WHERE l.name = 'MVIP League';

INSERT INTO divisions (season_id, name, format, skill_level_cap)
SELECT s.id, '8-Ball Open', '8-ball', 23
FROM league_sessions s WHERE s.name = 'Fall 2026';

INSERT INTO venues (league_id, name, address)
SELECT l.id, 'Saigon Cue Club', '12 Nguyen Hue, District 1, Ho Chi Minh City'
FROM leagues l WHERE l.name = 'MVIP League';

INSERT INTO teams (division_id, venue_id, team_number, name)
SELECT d.id, v.id, t.team_number, t.name
FROM divisions d
CROSS JOIN venues v
CROSS JOIN (VALUES
	('03209', 'Corner Pocket Crew'),
	('03214', 'Eight Ball Bandits'),
	('03211', 'Rack Attack'),
	('03207', 'Side Pocket Social')
) AS t (team_number, name)
WHERE d.name = '8-Ball Open' AND v.name = 'Saigon Cue Club';


-- Skill levels are deliberately top-heavy on 03209 and 03211: their five strongest total
-- 29 and 27, so the 23-Rule actually binds and phase 6's "Would break 23" state is
-- reachable. A roster of all 4s would never trigger it and would ship untested.
CREATE TEMP TABLE seed_players (
	member_id   TEXT,
	full_name   TEXT,
	skill_level SMALLINT,
	is_paid     BOOLEAN,
	team_number TEXT,
	is_captain  BOOLEAN
) ON COMMIT DROP;

INSERT INTO seed_players VALUES
	-- 03209 Corner Pocket Crew — top five = 7+6+6+5+5 = 29
	('95007620', 'Le Minh Quan',     7, true,  '03209', true),
	('95007621', 'Tran Hoang Nam',   6, true,  '03209', false),
	('95007622', 'Nguyen Thi Mai',   6, true,  '03209', false),
	('95007623', 'Pham Van Dung',    5, true,  '03209', false),
	('95007624', 'Vo Thanh Tung',    5, true,  '03209', false),
	('95007625', 'Dang Kim Anh',     4, true,  '03209', false),
	('95007626', 'Bui Quoc Huy',     3, true,  '03209', false),
	('95007627', 'Ho Ngoc Lan',      2, false, '03209', false),

	-- 03214 Eight Ball Bandits — top five = 6+5+5+4+4 = 24, binds by one
	('95007630', 'Nguyen Van Tuan',  6, true,  '03214', true),
	('95007631', 'Le Thi Hong',      5, true,  '03214', false),
	('95007632', 'Tran Dinh Phuc',   5, true,  '03214', false),
	('95007633', 'Pham Thi Thuy',    4, true,  '03214', false),
	('95007634', 'Do Van Khanh',     4, true,  '03214', false),
	('95007635', 'Ngo Minh Chau',    3, true,  '03214', false),
	('95007636', 'Huynh Gia Bao',    3, true,  '03214', false),

	-- 03211 Rack Attack — top five = 7+7+6+4+3 = 27
	('95007640', 'Vu Dinh Long',     7, true,  '03211', true),
	('95007641', 'Truong My Linh',   7, true,  '03211', false),
	('95007642', 'Phan Van Loc',     6, true,  '03211', false),
	('95007643', 'Dinh Thi Nga',     4, true,  '03211', false),
	('95007644', 'Luong Hai Son',    3, true,  '03211', false),
	('95007645', 'Cao Thi Yen',      3, true,  '03211', false),
	('95007646', 'Ly Thanh Dat',     2, true,  '03211', false),
	('95007647', 'Mai Van Thinh',    2, false, '03211', false),

	-- 03207 Side Pocket Social — top five = 5+5+4+4+4 = 22, never binds
	('95007650', 'Trinh Quoc Vinh',  5, true,  '03207', true),
	('95007651', 'Nguyen Thu Ha',    5, true,  '03207', false),
	('95007652', 'Hoang Van Tam',    4, true,  '03207', false),
	('95007653', 'Chu Thi Diem',     4, true,  '03207', false),
	('95007654', 'Ta Minh Duc',      4, true,  '03207', false),
	('95007655', 'Kieu Van Sang',    3, true,  '03207', false),
	('95007656', 'Lam Thi Oanh',     2, true,  '03207', false);

INSERT INTO players (member_id, full_name, skill_level, is_paid)
SELECT member_id, full_name, skill_level, is_paid FROM seed_players;

INSERT INTO team_rosters (team_id, player_id)
SELECT t.id, p.id
FROM seed_players sp
JOIN teams t   ON t.team_number = sp.team_number
JOIN players p ON p.member_id = sp.member_id;

UPDATE teams t
SET captain_player_id = p.id
FROM seed_players sp
JOIN players p ON p.member_id = sp.member_id
WHERE sp.is_captain AND t.team_number = sp.team_number;


-- Week 10, both fixtures on the same night at the one venue.
INSERT INTO team_matches (division_id, week_no, home_team_id, away_team_id, venue_id, scheduled_at)
SELECT d.id, 10, h.id, a.id, v.id, f.scheduled_at
FROM divisions d
CROSS JOIN (VALUES
	('03209', '03214', TIMESTAMPTZ '2026-11-10 19:30+07'),
	('03211', '03207', TIMESTAMPTZ '2026-11-10 19:30+07')
) AS f (home, away, scheduled_at)
JOIN teams h ON h.team_number = f.home
JOIN teams a ON a.team_number = f.away
CROSS JOIN venues v
WHERE d.name = '8-Ball Open' AND v.name = 'Saigon Cue Club';


-- Dev identities. These can never actually sign in: nobody holds these google_sub values and
-- there is no password anywhere in this system, so they exist only to give the role checks and
-- the captain-owns-this-night rule something to resolve against before a real Google login.
INSERT INTO users (google_sub, email, email_verified, name)
VALUES
	('dev-admin-google-sub',   'admin@example.test',   true, 'Dev Admin'),
	('dev-captain-google-sub', 'captain@example.test', true, 'Dev Captain');

INSERT INTO users_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON (u.google_sub, r.code) IN (
	('dev-admin-google-sub',   'SUPER_ADMIN'),
	('dev-captain-google-sub', 'CAPTAIN')
);

-- The dev captain is the captain of 03209, so their night is 03209 v 03214.
UPDATE players p
SET user_id = u.id
FROM users u
WHERE u.google_sub = 'dev-captain-google-sub' AND p.member_id = '95007620';


COMMIT;
