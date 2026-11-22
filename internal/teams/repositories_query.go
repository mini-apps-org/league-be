package teams

// Reads: gets, lists and counts. Lists and their counts share one filter builder each
// (teamListFilters, rosterListFilters), so a page and its total apply the same filters.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

// rosterPlayerSelect is completed by a WHERE on tr.division_id and tr.team_id; it only
// ever reads active rows.
const rosterPlayerSelect = `
SELECT
    tr.id                                           AS roster_id,
    tr.division_id                                  AS division_id,
    tr.team_id                                      AS team_id,
    p.id                                            AS player_id,
    p.member_id                                     AS member_id,
    p.full_name                                     AS full_name,
    p.skill_level                                   AS skill_level,
    p.is_paid                                       AS is_paid,
    (t.captain_player_id IS NOT DISTINCT FROM p.id) AS is_captain,
    COALESCE(ps.matches_played, 0)                  AS matches_played,
    COALESCE(ps.matches_won, 0)                     AS matches_won,
    COALESCE(ps.total_points, 0)                    AS total_points,
    tr.joined_at                                    AS joined_at
FROM team_rosters tr
JOIN teams t   ON t.id = tr.team_id
JOIN players p ON p.id = tr.player_id
LEFT JOIN players_stats ps ON ps.player_id = p.id AND ps.division_id = tr.division_id
WHERE tr.left_at IS NULL`

func (r *repository) GetTeamByID(ctx context.Context, id string) (*Team, error) {
	const query = `SELECT ` + teamColumns + ` FROM teams WHERE id = $1 AND NOT is_deleted`
	args := []any{
		id,
	}

	team, err := repo.QueryOne[Team](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("teams: get team %s: %w", id, err)
	}
	return team, nil
}

func (r *repository) ListTeams(ctx context.Context, params ListTeamsServiceParams) ([]Team, error) {
	filters, args := teamListFilters(params)

	column, columnType := teamSortColumn(params.Sort.Field)
	if after := params.Page.After; after != nil {
		args = append(args, after.LastValue, after.LastID)
		var (
			operator             = params.Sort.KeysetOperator() // ">" ascending, "<" descending
			lastValuePlaceholder = len(args) - 1                // $n of after.LastValue
			lastIDPlaceholder    = len(args)                    // $n of after.LastID
		)
		// Rows after the cursor's row in this sort, e.g. (name, id) < ($1::text, $2::uuid).
		keysetFilter := fmt.Sprintf("(%s, id) %s ($%d::%s, $%d::uuid)",
			column, operator, lastValuePlaceholder, columnType, lastIDPlaceholder)
		filters = append(filters, keysetFilter)
	}
	// Limit+1 is a lookahead: its presence tells the service a next page exists.
	args = append(args, params.Page.Limit+1)

	query := fmt.Sprintf("SELECT %s FROM teams WHERE %s ORDER BY %s %s, id %s LIMIT $%d",
		teamColumns, strings.Join(filters, " AND "),
		column, params.Sort.Direction(), params.Sort.Direction(), len(args))

	teams, err := repo.QueryAll[Team](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("teams: list teams: %w", err)
	}
	return teams, nil
}

// CountTeams counts the teams ListTeams pages through: the same filters, without the
// cursor's keyset predicate or the limit.
func (r *repository) CountTeams(ctx context.Context, params ListTeamsServiceParams) (int64, error) {
	filters, args := teamListFilters(params)
	query := fmt.Sprintf("SELECT count(*) FROM teams WHERE %s", strings.Join(filters, " AND "))

	total, err := repo.QueryCount(ctx, r.pgPool, query, args...)
	if err != nil {
		return 0, fmt.Errorf("teams: count teams: %w", err)
	}
	return total, nil
}

// teamListFilters builds a team list's WHERE clauses and their args. ListTeams and
// CountTeams both start from it, so a page and its total apply the same filters.
func teamListFilters(params ListTeamsServiceParams) (filters []string, args []any) {
	filters = []string{"NOT is_deleted"}
	add := func(clause string, value any) {
		args = append(args, value)
		filters = append(filters, fmt.Sprintf(clause, len(args)))
	}

	if params.DivisionID != "" {
		add("id IN (SELECT team_id FROM division_teams WHERE division_id = $%d)", params.DivisionID)
	}
	if params.VenueID != "" {
		add("venue_id = $%d", params.VenueID)
	}
	if params.Query != "" {
		add(`(name ILIKE '%%' || $%[1]d || '%%' OR team_code ILIKE '%%' || $%[1]d || '%%')`, escapeLike(params.Query))
	}
	return filters, args
}

func (r *repository) GetDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error) {
	const query = `
SELECT dt.division_id, dt.team_id, dt.created_at
FROM division_teams dt
JOIN teams t ON t.id = dt.team_id AND NOT t.is_deleted
WHERE dt.division_id = $1 AND dt.team_id = $2`
	args := []any{
		divisionID, // $1 division_id
		teamID,     // $2 team_id
	}

	entry, err := repo.QueryOne[DivisionTeam](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTeamNotInDivision
	}
	if err != nil {
		return nil, fmt.Errorf("teams: get team %s in division %s: %w", teamID, divisionID, err)
	}
	return entry, nil
}

func (r *repository) ListRosterPlayers(ctx context.Context, params ListRosterPlayersServiceParams) ([]RosterPlayer, error) {
	filters, args := rosterListFilters(params)

	column, columnType := rosterSortColumn(params.Sort.Field)
	if after := params.Page.After; after != nil {
		args = append(args, after.LastValue, after.LastID)
		var (
			operator             = params.Sort.KeysetOperator() // ">" ascending, "<" descending
			lastValuePlaceholder = len(args) - 1                // $n of after.LastValue
			lastIDPlaceholder    = len(args)                    // $n of after.LastID
		)
		// Rows after the cursor's row in this sort, e.g. (p.full_name, p.id) > ($3::text, $4::uuid).
		keysetFilter := fmt.Sprintf("(%s, p.id) %s ($%d::%s, $%d::uuid)",
			column, operator, lastValuePlaceholder, columnType, lastIDPlaceholder)
		filters = append(filters, keysetFilter)
	}
	args = append(args, params.Page.Limit+1)

	query := fmt.Sprintf("%s\n  AND %s\nORDER BY %s %s, p.id %s\nLIMIT $%d",
		rosterPlayerSelect, strings.Join(filters, " AND "),
		column, params.Sort.Direction(), params.Sort.Direction(), len(args))

	players, err := repo.QueryAll[RosterPlayer](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("teams: list roster of team %s in division %s: %w", params.TeamID, params.DivisionID, err)
	}
	return players, nil
}

// rosterPlayerCount joins only the tables rosterListFilters reads (tr, p); the joins
// rosterPlayerSelect adds for output columns do not change the count.
const rosterPlayerCount = `
SELECT count(*)
FROM team_rosters tr
JOIN players p ON p.id = tr.player_id
WHERE tr.left_at IS NULL`

// CountRosterPlayers counts the roster ListRosterPlayers pages through: the same filters,
// without the cursor's keyset predicate or the limit.
func (r *repository) CountRosterPlayers(ctx context.Context, params ListRosterPlayersServiceParams) (int64, error) {
	filters, args := rosterListFilters(params)
	query := fmt.Sprintf("%s\n  AND %s", rosterPlayerCount, strings.Join(filters, " AND "))

	total, err := repo.QueryCount(ctx, r.pgPool, query, args...)
	if err != nil {
		return 0, fmt.Errorf("teams: count roster of team %s in division %s: %w", params.TeamID, params.DivisionID, err)
	}
	return total, nil
}

// rosterListFilters builds a roster list's WHERE clauses and their args, against the tr
// and p aliases. ListRosterPlayers and CountRosterPlayers both start from it.
func rosterListFilters(params ListRosterPlayersServiceParams) (filters []string, args []any) {
	add := func(clause string, value any) {
		args = append(args, value)
		filters = append(filters, fmt.Sprintf(clause, len(args)))
	}

	add("tr.division_id = $%d", params.DivisionID)
	add("tr.team_id = $%d", params.TeamID)
	if params.Query != "" {
		add(`(p.full_name ILIKE '%%' || $%[1]d || '%%' OR p.member_id LIKE '%%' || $%[1]d || '%%')`, escapeLike(params.Query))
	}
	return filters, args
}

func (r *repository) GetRosterPlayer(ctx context.Context, params RosterEntryServiceParams) (*RosterPlayer, error) {
	const query = rosterPlayerSelect + `
  AND tr.division_id = $1
  AND tr.team_id = $2
  AND tr.player_id = $3`
	args := []any{
		params.DivisionID, // $1 division_id
		params.TeamID,     // $2 team_id
		params.PlayerID,   // $3 player_id
	}

	player, err := repo.QueryOne[RosterPlayer](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRosterPlayerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("teams: get roster player %s on team %s: %w", params.PlayerID, params.TeamID, err)
	}
	return player, nil
}

// IsActiveTeamPlayer reports whether the player is on the team's active roster in any
// division.
func (r *repository) IsActiveTeamPlayer(ctx context.Context, teamID, playerID string) (bool, error) {
	const query = `
SELECT EXISTS (
    SELECT 1 FROM team_rosters
    WHERE team_id = $1 AND player_id = $2 AND left_at IS NULL
)`
	args := []any{
		teamID,   // $1 team_id
		playerID, // $2 player_id
	}

	var isActive bool
	if err := r.pgPool.QueryRow(ctx, query, args...).Scan(&isActive); err != nil {
		return false, fmt.Errorf("teams: check player %s on team %s: %w", playerID, teamID, err)
	}
	return isActive, nil
}

// GetPlayerIDByUserID returns the player linked to a sign-in account; most players
// have none, which is ErrPlayerNotFound.
func (r *repository) GetPlayerIDByUserID(ctx context.Context, userID string) (string, error) {
	const query = `SELECT id FROM players WHERE user_id = $1`
	args := []any{
		userID,
	}

	var playerID string
	err := r.pgPool.QueryRow(ctx, query, args...).Scan(&playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPlayerNotFound
	}
	if err != nil {
		return "", fmt.Errorf("teams: get player of user %s: %w", userID, err)
	}
	return playerID, nil
}

func (r *repository) IsPaidPlayer(ctx context.Context, playerID string) (bool, error) {
	const query = `SELECT is_paid FROM players WHERE id = $1`
	args := []any{
		playerID,
	}

	var isPaid bool
	err := r.pgPool.QueryRow(ctx, query, args...).Scan(&isPaid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrPlayerNotFound
	}
	if err != nil {
		return false, fmt.Errorf("teams: check player %s paid: %w", playerID, err)
	}
	return isPaid, nil
}

// teamSortColumn maps an allowed sort field to its column and the type a cursor's text
// value is cast to. id always follows it, so rows with the same value keep one order and
// a page boundary never skips or repeats one. Keep in step with teamSortValue.
func teamSortColumn(field string) (column, columnType string) {
	switch field {
	case TeamSortByName:
		return "name", "text"
	case TeamSortByCreatedAt:
		return "created_at", "timestamptz"
	default:
		return "team_code", "text"
	}
}

// rosterSortColumn maps an allowed sort field to its column in rosterPlayerSelect. Stats
// sort as the response shows them: zero before the first match, not NULL. Keep in step
// with rosterSortValue.
func rosterSortColumn(field string) (column, columnType string) {
	switch field {
	case RosterSortByMemberID:
		return "p.member_id", "text"
	case RosterSortBySkillLevel:
		return "p.skill_level", "integer"
	case RosterSortByMatchesPlayed:
		return "COALESCE(ps.matches_played, 0)", "integer"
	case RosterSortByMatchesWon:
		return "COALESCE(ps.matches_won, 0)", "integer"
	case RosterSortByTotalPoints:
		return "COALESCE(ps.total_points, 0)", "integer"
	case RosterSortByJoinedAt:
		return "tr.joined_at", "timestamptz"
	default:
		return "p.full_name", "text"
	}
}

// escapeLike makes a search term literal inside a LIKE pattern, so a search for "100%"
// does not match everything.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
