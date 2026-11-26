package sessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

// leagueSessionColumns reads FROM league_sessions ls. The two derived values are
// correlated subqueries: cheap at a handful of sessions per league, and covered by
// team_matches_division_id_week_no_idx.
const leagueSessionColumns = `
    ls.id, ls.league_id, ls.name, ls.session_type, ls.season_year, ls.starts_on, ls.ends_on,
    ls.status, ls.is_active,
    (SELECT count(DISTINCT tm.week_no)
       FROM team_matches tm JOIN divisions d ON d.id = tm.division_id
      WHERE d.season_id = ls.id) AS week_count,
    EXISTS (SELECT 1
              FROM team_matches tm JOIN divisions d ON d.id = tm.division_id
             WHERE d.season_id = ls.id AND tm.status = 'submitted') AS has_submitted_results,
    ls.created_at, ls.updated_at`

// leagueSessionSortColumns whitelists the ORDER BY column for each sort field, so a
// column name never comes from the request.
var leagueSessionSortColumns = map[string]string{
	LeagueSessionSortByName:       "ls.name",
	LeagueSessionSortByStartsOn:   "ls.starts_on",
	LeagueSessionSortBySeasonYear: "ls.season_year",
}

// constraintErrors maps each constraint a write can trip, by its name in db/migrations,
// to its sentinel.
var constraintErrors = map[string]error{
	"league_sessions_league_id_name_key":    ErrDuplicateLeagueSessionName,
	"league_sessions_one_active_per_league": ErrLeagueSessionActiveConflict,
	"league_sessions_check":                 ErrInvalidLeagueSession, // ends_on >= starts_on
	"league_sessions_session_type_check":    ErrInvalidLeagueSession,
	"league_sessions_status_check":          ErrInvalidLeagueSession,
	// A delete that races a team entering another session's division: the cascade
	// removes the team but not that division's fixtures, which still reference it.
	"team_matches_home_division_team_fkey": ErrLeagueSessionHasSharedTeams,
	"team_matches_away_division_team_fkey": ErrLeagueSessionHasSharedTeams,
}

type Repository interface {
	GetDefaultLeagueID(ctx context.Context) (string, error)
	ListSessions(ctx context.Context, leagueID string, params ListLeagueSessionsServiceParams) ([]LeagueSession, error)
	GetSessionByID(ctx context.Context, leagueID, leagueSessionID string) (*LeagueSession, error)
	CreateSession(ctx context.Context, leagueID string, params CreateLeagueSessionServiceParams) (*LeagueSession, error)
	UpdateSession(ctx context.Context, leagueID, leagueSessionID string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error)
	DeleteSession(ctx context.Context, leagueID, leagueSessionID string) error
	ListSessionWeeks(ctx context.Context, leagueID, leagueSessionID string) ([]LeagueSessionWeek, error)
}

type repository struct {
	pgPool *pgxpool.Pool
	logger *slog.Logger
}

func NewRepo(pool *pgxpool.Pool, logger *slog.Logger) Repository {
	return &repository{pgPool: pool, logger: logger.With(slog.String("component", "sessions.repository"))}
}

// conn is the transaction carried by ctx, if a service opened one, else the pool.
func (r *repository) conn(ctx context.Context) db.Querier {
	return db.ConnFromContext(ctx, r.pgPool)
}

// GetDefaultLeagueID resolves the one ACTIVE league the MVP runs. LIMIT 2 is enough to
// tell "exactly one" from "more than one".
func (r *repository) GetDefaultLeagueID(ctx context.Context) (string, error) {
	const query = `SELECT id FROM leagues WHERE status = $1 ORDER BY id LIMIT 2`
	args := []any{
		"ACTIVE",
	}

	ids, err := repo.QueryStrings(ctx, r.conn(ctx), query, args...)
	if err != nil {
		return "", fmt.Errorf("sessions: get default league: %w", err)
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("%w: found %d", ErrNoDefaultLeague, len(ids))
	}
	return ids[0], nil
}

func (r *repository) ListSessions(ctx context.Context, leagueID string, params ListLeagueSessionsServiceParams) ([]LeagueSession, error) {
	var (
		filters []string
		args    []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		filters = append(filters, fmt.Sprintf(clause, len(args)))
	}

	add("ls.league_id = $%d", leagueID)
	if params.Query != "" {
		add(`ls.name ILIKE '%%' || $%d || '%%'`, escapeLike(params.Query))
	}
	if params.SessionType != "" {
		add("ls.session_type = $%d", string(params.SessionType))
	}
	if params.SeasonYear != 0 {
		add("ls.season_year = $%d", params.SeasonYear)
	}
	if params.Status != "" {
		add("ls.status = $%d", string(params.Status))
	}
	args = append(args, leagueSessionListLimit)

	column, ok := leagueSessionSortColumns[params.Sort.Field]
	if !ok {
		return nil, fmt.Errorf("sessions: list league sessions: unknown sort field %q", params.Sort.Field)
	}
	direction := params.Sort.Direction()

	query := fmt.Sprintf("SELECT %s FROM league_sessions ls WHERE %s ORDER BY %s %s, ls.id %s LIMIT $%d",
		leagueSessionColumns, strings.Join(filters, " AND "), column, direction, direction, len(args))

	sessions, err := repo.QueryAll[LeagueSession](ctx, r.conn(ctx), query, args...)
	if err != nil {
		return nil, fmt.Errorf("sessions: list league sessions: %w", err)
	}
	return sessions, nil
}

func (r *repository) GetSessionByID(ctx context.Context, leagueID, leagueSessionID string) (*LeagueSession, error) {
	return getLeagueSession(ctx, r.conn(ctx), leagueID, leagueSessionID)
}

func getLeagueSession(ctx context.Context, conn db.Querier, leagueID, leagueSessionID string) (*LeagueSession, error) {
	const query = `SELECT ` + leagueSessionColumns + ` FROM league_sessions ls WHERE ls.league_id = $1 AND ls.id = $2`
	args := []any{
		leagueID,        // $1 league_id
		leagueSessionID, // $2 id
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sessions: get league session %s: %w", leagueSessionID, err)
	}
	session, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[LeagueSession])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLeagueSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sessions: get league session %s: %w", leagueSessionID, err)
	}
	return &session, nil
}

// CreateSession deactivates the league's other sessions first when the new one is
// active, in the same transaction: league_sessions_one_active_per_league would reject
// the insert otherwise.
func (r *repository) CreateSession(ctx context.Context, leagueID string, params CreateLeagueSessionServiceParams) (*LeagueSession, error) {
	const query = `
INSERT INTO league_sessions (league_id, name, session_type, season_year, starts_on, ends_on, status, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id`
	args := []any{
		leagueID,                   // $1 league_id
		params.Name,                // $2 name
		string(params.SessionType), // $3 session_type
		params.SeasonYear,          // $4 season_year
		params.StartsOn,            // $5 starts_on
		params.EndsOn,              // $6 ends_on
		string(params.Status),      // $7 status
		params.IsActive,            // $8 is_active
	}

	var session *LeagueSession
	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		if params.IsActive {
			if err := deactivateOtherLeagueSessions(ctx, tx, leagueID, nil); err != nil {
				return err
			}
		}

		var leagueSessionID string
		if err := tx.QueryRow(ctx, query, args...).Scan(&leagueSessionID); err != nil {
			return err
		}

		var err error
		session, err = getLeagueSession(ctx, tx, leagueID, leagueSessionID)
		return err
	})
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("sessions: create league session: %w", err)
	}
	return session, nil
}

// UpdateSession applies a partial update: COALESCE leaves a column alone when its
// argument is NULL. The service has already validated the merged result; the schema
// checks catch a concurrent update that breaks it anyway.
func (r *repository) UpdateSession(ctx context.Context, leagueID, leagueSessionID string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error) {
	const query = `
UPDATE league_sessions SET
    name         = COALESCE($3, name),
    session_type = COALESCE($4, session_type),
    season_year  = COALESCE($5, season_year),
    starts_on    = COALESCE($6, starts_on),
    ends_on      = COALESCE($7, ends_on),
    status       = COALESCE($8, status),
    is_active    = COALESCE($9, is_active),
    updated_at   = now()
WHERE league_id = $1 AND id = $2`
	args := []any{
		leagueID,           // $1 league_id
		leagueSessionID,    // $2 id
		params.Name,        // $3 name, NULL leaves it
		params.SessionType, // $4 session_type, NULL leaves it
		params.SeasonYear,  // $5 season_year, NULL leaves it
		params.StartsOn,    // $6 starts_on, NULL leaves it
		params.EndsOn,      // $7 ends_on, NULL leaves it
		params.Status,      // $8 status, NULL leaves it
		params.IsActive,    // $9 is_active, NULL leaves it
	}

	var session *LeagueSession
	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		if params.IsActive != nil && *params.IsActive {
			if err := deactivateOtherLeagueSessions(ctx, tx, leagueID, &leagueSessionID); err != nil {
				return err
			}
		}

		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			// Returned inside the transaction so the deactivation above rolls back.
			return ErrLeagueSessionNotFound
		}

		session, err = getLeagueSession(ctx, tx, leagueID, leagueSessionID)
		return err
	})
	if errors.Is(err, ErrLeagueSessionNotFound) {
		return nil, ErrLeagueSessionNotFound
	}
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("sessions: update league session %s: %w", leagueSessionID, err)
	}
	return session, nil
}

// deactivateOtherLeagueSessions clears is_active on every session of the league except
// keepID; a nil keepID clears them all, since nothing IS DISTINCT FROM NULL fails.
func deactivateOtherLeagueSessions(ctx context.Context, tx pgx.Tx, leagueID string, keepID *string) error {
	const query = `
UPDATE league_sessions SET
    is_active  = false,
    updated_at = now()
WHERE league_id = $1 AND is_active AND id IS DISTINCT FROM $2`
	args := []any{
		leagueID, // $1 league_id
		keepID,   // $2 id to keep, NULL keeps none
	}

	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("sessions: deactivate other league sessions: %w", err)
	}
	return nil
}

// DeleteSession refuses a session that homes a team (teams.division_id) which has
// also entered a division of another session. The cascade through teams.division_id
// would delete that team from the other session too, roster included.
func (r *repository) DeleteSession(ctx context.Context, leagueID, leagueSessionID string) error {
	const sharedTeamsQuery = `
SELECT EXISTS (
    SELECT 1
    FROM teams t
    JOIN divisions home    ON home.id = t.division_id
    JOIN division_teams dt ON dt.team_id = t.id
    JOIN divisions other   ON other.id = dt.division_id
    WHERE home.season_id = $1 AND other.season_id <> $1
)`
	sharedTeamsArgs := []any{
		leagueSessionID,
	}
	const deleteQuery = `DELETE FROM league_sessions WHERE league_id = $1 AND id = $2`
	deleteArgs := []any{
		leagueID,        // $1 league_id
		leagueSessionID, // $2 id
	}

	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		var hasSharedTeams bool
		if err := tx.QueryRow(ctx, sharedTeamsQuery, sharedTeamsArgs...).Scan(&hasSharedTeams); err != nil {
			return err
		}
		if hasSharedTeams {
			return ErrLeagueSessionHasSharedTeams
		}

		tag, err := tx.Exec(ctx, deleteQuery, deleteArgs...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrLeagueSessionNotFound
		}
		return nil
	})
	if errors.Is(err, ErrLeagueSessionHasSharedTeams) || errors.Is(err, ErrLeagueSessionNotFound) {
		return err
	}
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("sessions: delete league session %s: %w", leagueSessionID, err)
	}
	return nil
}

// ListSessionWeeks checks the session first, so a missing session is a 404 rather
// than an empty list.
func (r *repository) ListSessionWeeks(ctx context.Context, leagueID, leagueSessionID string) ([]LeagueSessionWeek, error) {
	const existsQuery = `SELECT EXISTS (SELECT 1 FROM league_sessions WHERE league_id = $1 AND id = $2)`
	existsArgs := []any{
		leagueID,        // $1 league_id
		leagueSessionID, // $2 id
	}

	var exists bool
	if err := r.conn(ctx).QueryRow(ctx, existsQuery, existsArgs...).Scan(&exists); err != nil {
		return nil, fmt.Errorf("sessions: check league session %s: %w", leagueSessionID, err)
	}
	if !exists {
		return nil, ErrLeagueSessionNotFound
	}

	// Bounded by the session's week numbers, a few dozen at most.
	const weeksQuery = `
SELECT
    tm.week_no                                    AS week_no,
    count(*)                                      AS match_count,
    count(*) FILTER (WHERE tm.status = 'submitted') AS submitted_count,
    min(tm.scheduled_at)                          AS first_scheduled_at
FROM team_matches tm
JOIN divisions d ON d.id = tm.division_id
WHERE d.season_id = $1
GROUP BY tm.week_no
ORDER BY tm.week_no`
	weeksArgs := []any{
		leagueSessionID,
	}

	weeks, err := repo.QueryAll[LeagueSessionWeek](ctx, r.conn(ctx), weeksQuery, weeksArgs...)
	if err != nil {
		return nil, fmt.Errorf("sessions: list league session %s weeks: %w", leagueSessionID, err)
	}
	return weeks, nil
}

func constraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	return constraintErrors[pgErr.ConstraintName]
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
