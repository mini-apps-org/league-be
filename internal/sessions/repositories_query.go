package sessions

// Reads: the default league, gets, lists and counts of sessions, and a session's weeks.
// ListSessions and CountSessions share leagueSessionListFilters, so a page and its total
// apply the same filters. Every read hides soft-deleted sessions.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

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
	filters, args := leagueSessionListFilters(leagueID, params)

	column, columnType := leagueSessionSortColumn(params.Sort.Field)
	if after := params.Page.After; after != nil {
		args = append(args, after.LastValue, after.LastID)
		var (
			operator             = params.Sort.KeysetOperator() // ">" ascending, "<" descending
			lastValuePlaceholder = len(args) - 1                // $n of after.LastValue
			lastIDPlaceholder    = len(args)                    // $n of after.LastID
		)
		// Rows after the cursor's row in this sort, e.g. (ls.starts_on, ls.id) < ($1::date, $2::uuid).
		keysetFilter := fmt.Sprintf("(%s, ls.id) %s ($%d::%s, $%d::uuid)",
			column, operator, lastValuePlaceholder, columnType, lastIDPlaceholder)
		filters = append(filters, keysetFilter)
	}
	// Limit+1 is a lookahead: its presence tells the service a next page exists.
	args = append(args, params.Page.Limit+1)

	query := fmt.Sprintf("SELECT %s FROM league_sessions ls WHERE %s ORDER BY %s %s, ls.id %s LIMIT $%d",
		leagueSessionColumns, strings.Join(filters, " AND "),
		column, params.Sort.Direction(), params.Sort.Direction(), len(args))

	sessions, err := repo.QueryAll[LeagueSession](ctx, r.conn(ctx), query, args...)
	if err != nil {
		return nil, fmt.Errorf("sessions: list league sessions: %w", err)
	}
	return sessions, nil
}

// CountSessions counts the sessions ListSessions pages through: the same filters, without
// the cursor's keyset predicate or the limit.
func (r *repository) CountSessions(ctx context.Context, leagueID string, params ListLeagueSessionsServiceParams) (int64, error) {
	filters, args := leagueSessionListFilters(leagueID, params)
	query := fmt.Sprintf("SELECT count(*) FROM league_sessions ls WHERE %s", strings.Join(filters, " AND "))

	total, err := repo.QueryCount(ctx, r.conn(ctx), query, args...)
	if err != nil {
		return 0, fmt.Errorf("sessions: count league sessions: %w", err)
	}
	return total, nil
}

func leagueSessionListFilters(leagueID string, params ListLeagueSessionsServiceParams) (filters []string, args []any) {
	filters = []string{"NOT ls.is_deleted"}
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
	return filters, args
}

func (r *repository) GetSessionByID(ctx context.Context, leagueID, leagueSessionID string) (*LeagueSession, error) {
	return getLeagueSession(ctx, r.conn(ctx), leagueID, leagueSessionID)
}

func getLeagueSession(ctx context.Context, conn db.Querier, leagueID, leagueSessionID string) (*LeagueSession, error) {
	const query = `SELECT ` + leagueSessionColumns + `
FROM league_sessions ls
WHERE ls.league_id = $1 AND ls.id = $2 AND NOT ls.is_deleted`
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

// ListSessionWeeks checks the session first, so a missing session is a 404 rather
// than an empty list.
func (r *repository) ListSessionWeeks(ctx context.Context, leagueID, leagueSessionID string) ([]LeagueSessionWeek, error) {
	const existsQuery = `
SELECT EXISTS (SELECT 1 FROM league_sessions WHERE league_id = $1 AND id = $2 AND NOT is_deleted)`
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

// leagueSessionSortColumn maps an allowed sort field to its column and the type a cursor's
// text value is cast to. ls.id always follows it, so rows with the same value keep one
// order and a page boundary never skips or repeats one. Keep in step with
// leagueSessionSortValue.
func leagueSessionSortColumn(field string) (column, columnType string) {
	switch field {
	case LeagueSessionSortByName:
		return "ls.name", "text"
	case LeagueSessionSortBySeasonYear:
		return "ls.season_year", "smallint"
	default:
		return "ls.starts_on", "date"
	}
}

// escapeLike makes a search term literal inside a LIKE pattern, so a search for "100%"
// does not match everything.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
