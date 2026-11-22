package teams

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

var (
	ErrTeamNotFound            = errors.New("teams: team not found")
	ErrTeamNotSoftDeleted      = errors.New("teams: team not soft-deleted")
	ErrDuplicateTeamCode       = errors.New("teams: duplicate team number")
	ErrDivisionNotFound        = errors.New("teams: division not found")
	ErrVenueNotFound           = errors.New("teams: venue not found")
	ErrTeamHasMatches          = errors.New("teams: team has matches")
	ErrTeamNotInDivision       = errors.New("teams: team has not entered division")
	ErrTeamAlreadyInDivision   = errors.New("teams: team already entered division")
	ErrPlayerNotFound          = errors.New("teams: player not found")
	ErrRosterPlayerNotFound    = errors.New("teams: player not on active roster")
	ErrPlayerAlreadyInDivision = errors.New("teams: player already on an active roster in division")
)

// constraintErrors maps each constraint a write in this package can trip, by the name
// given in db/migrations, to its sentinel. The SQLSTATE alone cannot tell them apart:
// one INSERT INTO teams can violate three different foreign keys.
var constraintErrors = map[string]error{
	"teams_division_id_team_code_key":               ErrDuplicateTeamCode, // unique within a division
	"teams_team_code_key":                           ErrDuplicateTeamCode, // unique across the league, on databases that had it
	"teams_venue_id_fkey":                           ErrVenueNotFound,
	"teams_captain_player_id_fkey":                  ErrPlayerNotFound,
	"division_teams_pkey":                           ErrTeamAlreadyInDivision,
	"division_teams_division_id_fkey":               ErrDivisionNotFound,
	"teams_division_id_fkey":                        ErrDivisionNotFound,
	"division_teams_team_id_fkey":                   ErrTeamNotFound,
	"team_rosters_division_team_fkey":               ErrTeamNotInDivision,
	"team_rosters_player_id_fkey":                   ErrPlayerNotFound,
	"team_rosters_player_id_division_id_active_key": ErrPlayerAlreadyInDivision,
	"team_matches_home_team_id_fkey":                ErrTeamHasMatches,
	"team_matches_away_team_id_fkey":                ErrTeamHasMatches,
	"team_matches_home_division_team_fkey":          ErrTeamHasMatches,
	"team_matches_away_division_team_fkey":          ErrTeamHasMatches,
}

const teamListLimit = 200

// TODO vincent: Need write as the array format and using String.joins to easier remove or add the new field as needed
const teamColumns = `id, team_code, name, captain_player_id, venue_id, created_at, updated_at`

const divisionTeamColumns = `division_id, team_id, created_at`

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

type Repository interface {
	CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error)
	GetTeamByID(ctx context.Context, id string) (*Team, error)
	ListTeams(ctx context.Context, params ListTeamsServiceParams) ([]Team, error)
	UpdateTeam(ctx context.Context, id string, params UpdateTeamServiceParams) (*Team, error)
	SoftDeleteTeam(ctx context.Context, id string) error
	HardDeleteTeam(ctx context.Context, id string) error

	AddDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error)
	GetDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error)
	RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error

	ListRosterPlayers(ctx context.Context, divisionID, teamID string) ([]RosterPlayer, error)
	GetRosterPlayer(ctx context.Context, params RosterEntryServiceParams) (*RosterPlayer, error)
	IsActiveTeamPlayer(ctx context.Context, teamID, playerID string) (bool, error)
	GetPlayerIDByUserID(ctx context.Context, userID string) (string, error)
	IsPaidPlayer(ctx context.Context, playerID string) (bool, error)
	AddRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error
	RemoveRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error
	TransferRosterPlayer(ctx context.Context, params TransferRosterPlayerServiceParams) error
}

type repository struct {
	pgPool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) Repository {
	return &repository{pgPool: pool}
}

func (r *repository) CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error) {
	const query = `
INSERT INTO teams (division_id, team_code, name, venue_id)
VALUES ($1, $2, $3, $4)
RETURNING ` + teamColumns
	args := []any{
		params.DivisionID, // $1 division_id
		params.TeamCode,   // $2 team_code
		params.Name,       // $3 name
		params.VenueID,    // $4 venue_id
	}

	team, err := repo.QueryOne[Team](ctx, r.pgPool, query, args...)
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("teams: create team: %w", err)
	}
	return team, nil
}

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
	var (
		filters = []string{"NOT is_deleted"}
		args    []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		filters = append(filters, fmt.Sprintf(clause, len(args)))
	}

	if params.DivisionID != "" {
		add("id IN (SELECT team_id FROM division_teams WHERE division_id = $%d)", params.DivisionID)
	}
	args = append(args, teamListLimit)

	query := fmt.Sprintf("SELECT %s FROM teams WHERE %s ORDER BY team_code LIMIT $%d",
		teamColumns, strings.Join(filters, " AND "), len(args))

	teams, err := repo.QueryAll[Team](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("teams: list teams: %w", err)
	}
	return teams, nil
}

// UpdateTeam is one statement: COALESCE leaves a column alone when its argument is
// NULL, so concurrent PATCHes cannot lose an update.
func (r *repository) UpdateTeam(ctx context.Context, id string, params UpdateTeamServiceParams) (*Team, error) {
	const query = `
UPDATE teams SET
    team_code       = COALESCE($2, team_code),
    name              = COALESCE($3, name),
    captain_player_id = COALESCE($4, captain_player_id),
    venue_id          = COALESCE($5, venue_id),
    updated_at        = now()
WHERE id = $1 AND NOT is_deleted
RETURNING ` + teamColumns
	args := []any{
		id,                     // $1 id
		params.TeamCode,        // $2 team_code, NULL leaves it
		params.Name,            // $3 name, NULL leaves it
		params.CaptainPlayerID, // $4 captain_player_id, NULL leaves it
		params.VenueID,         // $5 venue_id, NULL leaves it
	}

	team, err := repo.QueryOne[Team](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTeamNotFound
	}
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("teams: update team %s: %w", id, err)
	}
	return team, nil
}

// SoftDeleteTeam hides the team and closes its active roster entries in one transaction,
// so its players are free to join another team in the division. Fixtures, division
// entries and roster history stay.
func (r *repository) SoftDeleteTeam(ctx context.Context, id string) error {
	const deleteTeamQuery = `
UPDATE teams SET
    is_deleted = true,
    updated_at = now()
WHERE id = $1 AND NOT is_deleted`
	const closeRostersQuery = `
UPDATE team_rosters SET
    left_at    = now(),
    updated_at = now()
WHERE team_id = $1 AND left_at IS NULL`
	args := []any{
		id,
	}

	err := pgx.BeginFunc(ctx, r.pgPool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, deleteTeamQuery, args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrTeamNotFound
		}
		_, err = tx.Exec(ctx, closeRostersQuery, args...)
		return err
	})
	if err != nil {
		return fmt.Errorf("teams: soft delete team %s: %w", id, err)
	}
	return nil
}

// HardDeleteTeam removes a soft-deleted team; its division entries and roster history
// cascade. A team with a fixture is protected by the team_matches foreign keys, which do
// not cascade.
func (r *repository) HardDeleteTeam(ctx context.Context, id string) error {
	const query = `DELETE FROM teams WHERE id = $1 AND is_deleted`
	args := []any{
		id,
	}

	tag, err := r.pgPool.Exec(ctx, query, args...)
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("teams: hard delete team %s: %w", id, err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	return r.missingOrLiveTeamError(ctx, id)
}

// missingOrLiveTeamError explains why a hard delete matched nothing.
func (r *repository) missingOrLiveTeamError(ctx context.Context, id string) error {
	const query = `SELECT EXISTS (SELECT 1 FROM teams WHERE id = $1)`
	args := []any{
		id,
	}

	var exists bool
	if err := r.pgPool.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return fmt.Errorf("teams: check team %s: %w", id, err)
	}
	if exists {
		return ErrTeamNotSoftDeleted
	}
	return ErrTeamNotFound
}

func (r *repository) AddDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error) {
	// No row back means the team is missing or soft-deleted.
	const query = `
INSERT INTO division_teams (division_id, team_id)
SELECT $1::uuid, id FROM teams WHERE id = $2 AND NOT is_deleted
RETURNING ` + divisionTeamColumns
	args := []any{
		divisionID, // $1 division_id
		teamID,     // $2 team_id
	}

	entry, err := repo.QueryOne[DivisionTeam](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTeamNotFound
	}
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("teams: add team %s to division %s: %w", teamID, divisionID, err)
	}
	return entry, nil
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

// RemoveDivisionTeam cascades to the team's roster in that division. A team with a
// fixture in the division is protected by the team_matches foreign keys.
func (r *repository) RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error {
	const query = `DELETE FROM division_teams WHERE division_id = $1 AND team_id = $2`
	args := []any{
		divisionID, // $1 division_id
		teamID,     // $2 team_id
	}

	tag, err := r.pgPool.Exec(ctx, query, args...)
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("teams: remove team %s from division %s: %w", teamID, divisionID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotInDivision
	}
	return nil
}

func (r *repository) ListRosterPlayers(ctx context.Context, divisionID, teamID string) ([]RosterPlayer, error) {
	const query = rosterPlayerSelect + `
  AND tr.division_id = $1
  AND tr.team_id = $2
ORDER BY p.full_name, p.id`
	args := []any{
		divisionID, // $1 division_id
		teamID,     // $2 team_id
	}

	players, err := repo.QueryAll[RosterPlayer](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("teams: list roster of team %s in division %s: %w", teamID, divisionID, err)
	}
	return players, nil
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

func (r *repository) AddRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error {
	if err := insertRosterRow(ctx, r.pgPool, params); err != nil {
		return fmt.Errorf("teams: add roster player %s` to team %s: %w", params.PlayerID, params.TeamID, err)
	}
	return nil
}

func (r *repository) RemoveRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error {
	if err := closeRosterRow(ctx, r.pgPool, params); err != nil {
		return fmt.Errorf("teams: remove roster player %s from team %s: %w", params.PlayerID, params.TeamID, err)
	}
	return nil
}

// TransferRosterPlayer closes the source row and opens the target row in one
// transaction, so the player is never on both rosters nor on neither. Closing first
// frees the player's place in the division for a transfer within it.
func (r *repository) TransferRosterPlayer(ctx context.Context, params TransferRosterPlayerServiceParams) error {
	err := pgx.BeginFunc(ctx, r.pgPool, func(tx pgx.Tx) error {
		if err := closeRosterRow(ctx, tx, params.From); err != nil {
			return err
		}
		return insertRosterRow(ctx, tx, RosterEntryServiceParams{
			DivisionID: params.ToDivisionID,
			TeamID:     params.ToTeamID,
			PlayerID:   params.From.PlayerID,
		})
	})
	if err != nil {
		return fmt.Errorf("teams: transfer player %s from team %s to %s: %w",
			params.From.PlayerID, params.From.TeamID, params.ToTeamID, err)
	}
	return nil
}

type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertRosterRow(ctx context.Context, db execer, params RosterEntryServiceParams) error {
	// Inserts nothing when the team is soft-deleted.
	const query = `
INSERT INTO team_rosters (division_id, team_id, player_id)
SELECT $1::uuid, $2::uuid, $3::uuid
WHERE EXISTS (SELECT 1 FROM teams WHERE id = $2 AND NOT is_deleted)`
	args := []any{
		params.DivisionID, // $1 division_id
		params.TeamID,     // $2 team_id
		params.PlayerID,   // $3 player_id
	}

	tag, err := db.Exec(ctx, query, args...)
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("insert roster row: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func closeRosterRow(ctx context.Context, db execer, params RosterEntryServiceParams) error {
	const query = `
UPDATE team_rosters SET
    left_at    = now(),
    updated_at = now()
WHERE division_id = $1
  AND team_id = $2
  AND player_id = $3
  AND left_at IS NULL`
	args := []any{
		params.DivisionID, // $1 division_id
		params.TeamID,     // $2 team_id
		params.PlayerID,   // $3 player_id
	}

	tag, err := db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("close roster row: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRosterPlayerNotFound
	}
	return nil
}

// constraintError returns the sentinel for the constraint err violated, or nil when err
// is not a violation this package knows.
func constraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	return constraintErrors[pgErr.ConstraintName]
}
