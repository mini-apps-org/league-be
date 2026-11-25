package teams

// Writes: create, update, soft and permanent delete, division entries and roster changes.
// A constraint a write trips is mapped to its sentinel by constraintError.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nguyentanvinh/league-be/internal/platform/db"
	"github.com/nguyentanvinh/league-be/internal/platform/handler/repo"
)

// constraintErrors maps each constraint a write in this package can trip, by the name
// given in db/migrations, to its sentinel. The SQLSTATE alone cannot tell them apart:
// one INSERT INTO teams can violate three different foreign keys.
var constraintErrors = map[string]error{
	"teams_division_id_team_code_key":               ErrDuplicateTeamCode, // unique within a division
	"teams_team_code_key":                           ErrDuplicateTeamCode, // unique across the league, on databases that had it
	"teams_division_id_fkey":                        ErrDivisionNotFound,
	"teams_venue_id_fkey":                           ErrVenueNotFound,
	"teams_captain_player_id_fkey":                  ErrPlayerNotFound,
	"division_teams_pkey":                           ErrTeamAlreadyInDivision,
	"division_teams_division_id_fkey":               ErrDivisionNotFound,
	"division_teams_team_id_fkey":                   ErrTeamNotFound,
	"team_rosters_division_team_fkey":               ErrTeamNotInDivision,
	"team_rosters_player_id_fkey":                   ErrPlayerNotFound,
	"team_rosters_player_id_division_id_active_key": ErrPlayerAlreadyInDivision,
	"team_matches_home_team_id_fkey":                ErrTeamHasMatches,
	"team_matches_away_team_id_fkey":                ErrTeamHasMatches,
	"team_matches_home_division_team_fkey":          ErrTeamHasMatches,
	"team_matches_away_division_team_fkey":          ErrTeamHasMatches,
}

// CreateTeam inserts the team and its entry into its home division in one transaction, so
// a team never exists without the division_teams row its roster and fixtures hang off.
func (r *repository) CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error) {
	const createTeamQuery = `
INSERT INTO teams (division_id, team_code, name, venue_id)
VALUES ($1, $2, $3, $4)
RETURNING ` + teamColumns
	const enterDivisionQuery = `INSERT INTO division_teams (division_id, team_id) VALUES ($1, $2)`

	var team Team
	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		createTeamArgs := []any{
			params.DivisionID, // $1 division_id
			params.TeamCode,   // $2 team_code
			params.Name,       // $3 name
			params.VenueID,    // $4 venue_id
		}
		rows, err := tx.Query(ctx, createTeamQuery, createTeamArgs...)
		if err != nil {
			return err
		}
		if team, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[Team]); err != nil {
			return err
		}

		enterDivisionArgs := []any{
			params.DivisionID, // $1 division_id
			team.ID,           // $2 team_id
		}
		_, err = tx.Exec(ctx, enterDivisionQuery, enterDivisionArgs...)
		return err
	})
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("teams: create team: %w", err)
	}
	return &team, nil
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

	team, err := repo.QueryOne[Team](ctx, r.conn(ctx), query, args...)
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

	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
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

// PermanentDeleteTeam removes a soft-deleted team; its division entries and roster history
// cascade. A team with a fixture is protected by the team_matches foreign keys, which do
// not cascade.
func (r *repository) PermanentDeleteTeam(ctx context.Context, id string) error {
	const query = `DELETE FROM teams WHERE id = $1 AND is_deleted`
	args := []any{
		id,
	}

	tag, err := r.conn(ctx).Exec(ctx, query, args...)
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("teams: permanently delete team %s: %w", id, err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	return r.missingOrLiveTeamError(ctx, id)
}

// missingOrLiveTeamError explains why a permanent delete matched nothing.
func (r *repository) missingOrLiveTeamError(ctx context.Context, id string) error {
	const query = `SELECT EXISTS (SELECT 1 FROM teams WHERE id = $1)`
	args := []any{
		id,
	}

	var exists bool
	if err := r.conn(ctx).QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return fmt.Errorf("teams: check team %s: %w", id, err)
	}
	if exists {
		return ErrTeamNotSoftDeleted
	}
	return ErrTeamNotFound
}

// BulkSoftDeleteTeams soft-deletes every live team in ids and closes their active roster
// entries, in one statement. A requested team it did not delete was missing or already
// deleted: ErrTeamNotFound, as SoftDeleteTeam reports. ids must be canonical uuids.
func (r *repository) BulkSoftDeleteTeams(ctx context.Context, ids []string) (*BulkDeleteTeamsResult, error) {
	// A data-modifying WITH runs whether or not the outer query reads it, so the rosters
	// close in the same statement, and so the same transaction, as the teams.
	const query = `
WITH deleted AS (
    UPDATE teams SET
        is_deleted = true,
        updated_at = now()
    WHERE id = ANY($1::uuid[]) AND NOT is_deleted
    RETURNING id
), closed_rosters AS (
    UPDATE team_rosters SET
        left_at    = now(),
        updated_at = now()
    WHERE team_id IN (SELECT id FROM deleted) AND left_at IS NULL
)
SELECT id FROM deleted`
	args := []any{
		ids, // $1 team ids
	}

	rows, err := r.conn(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("teams: bulk soft delete teams: %w", err)
	}
	deletedIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("teams: bulk soft delete teams: %w", err)
	}

	return newBulkDeleteTeamsResult(ids, deletedIDs, func(string) error { return ErrTeamNotFound }), nil
}

// teamDeleteState is what a bulk permanent delete reads to explain a team it left in place.
type teamDeleteState struct {
	ID         string `db:"id"`
	TeamExists bool   `db:"team_exists"`
	IsDeleted  bool   `db:"is_deleted"`
}

// BulkPermanentDeleteTeams removes every soft-deleted team in ids that has no fixture, then
// reads why each other one stayed: missing (ErrTeamNotFound), not soft-deleted
// (ErrTeamNotSoftDeleted) or with a fixture (ErrTeamHasMatches) — what PermanentDeleteTeam
// reports. Two statements in one transaction. ids must be canonical uuids.
func (r *repository) BulkPermanentDeleteTeams(ctx context.Context, ids []string) (*BulkDeleteTeamsResult, error) {
	// A team with a fixture is skipped here rather than left to the team_matches foreign
	// keys: a violation aborts the whole statement, and with it every other team.
	const deleteQuery = `
DELETE FROM teams t
WHERE t.id = ANY($1::uuid[])
  AND t.is_deleted
  AND NOT EXISTS (
      SELECT 1 FROM team_matches m
      WHERE m.home_team_id = t.id OR m.away_team_id = t.id
  )
RETURNING t.id`
	const stateQuery = `
SELECT
    requested.id                    AS id,
    t.id IS NOT NULL                AS team_exists,
    COALESCE(t.is_deleted, false)   AS is_deleted
FROM unnest($1::uuid[]) AS requested (id)
LEFT JOIN teams t ON t.id = requested.id`
	args := []any{
		ids, // $1 team ids
	}

	var (
		deletedIDs []string
		states     = map[string]teamDeleteState{}
	)
	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, deleteQuery, args...)
		if err != nil {
			return err
		}
		if deletedIDs, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}

		rows, err = tx.Query(ctx, stateQuery, args...)
		if err != nil {
			return err
		}
		found, err := pgx.CollectRows(rows, pgx.RowToStructByName[teamDeleteState])
		if err != nil {
			return err
		}
		for _, state := range found {
			states[state.ID] = state
		}
		return nil
	})
	// Only a fixture written between the NOT EXISTS and the DELETE reaches the foreign keys:
	// the whole batch then fails as ErrTeamHasMatches.
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("teams: bulk permanently delete teams: %w", err)
	}

	return newBulkDeleteTeamsResult(ids, deletedIDs, func(id string) error {
		switch state := states[id]; {
		case !state.TeamExists:
			return ErrTeamNotFound
		case !state.IsDeleted:
			return ErrTeamNotSoftDeleted
		default:
			return ErrTeamHasMatches
		}
	}), nil
}

// newBulkDeleteTeamsResult lists each requested id exactly once, in request order: in
// DeletedIDs when the statement returned it, otherwise in Failures with failureOf(id).
func newBulkDeleteTeamsResult(requestedIDs, deletedIDs []string, failureOf func(id string) error) *BulkDeleteTeamsResult {
	deleted := make(map[string]bool, len(deletedIDs))
	for _, id := range deletedIDs {
		deleted[id] = true
	}

	result := &BulkDeleteTeamsResult{DeletedIDs: []string{}, Failures: []BulkDeleteTeamFailure{}}
	for _, id := range requestedIDs {
		if deleted[id] {
			result.DeletedIDs = append(result.DeletedIDs, id)
			continue
		}
		result.Failures = append(result.Failures, BulkDeleteTeamFailure{TeamID: id, Err: failureOf(id)})
	}
	return result
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

	entry, err := repo.QueryOne[DivisionTeam](ctx, r.conn(ctx), query, args...)
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

// RemoveDivisionTeam cascades to the team's roster in that division. A team with a
// fixture in the division is protected by the team_matches foreign keys.
func (r *repository) RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error {
	const query = `DELETE FROM division_teams WHERE division_id = $1 AND team_id = $2`
	args := []any{
		divisionID, // $1 division_id
		teamID,     // $2 team_id
	}

	tag, err := r.conn(ctx).Exec(ctx, query, args...)
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

func (r *repository) AddRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error {
	if err := insertRosterRow(ctx, r.conn(ctx), params); err != nil {
		return fmt.Errorf("teams: add roster player %s to team %s: %w", params.PlayerID, params.TeamID, err)
	}
	return nil
}

func (r *repository) RemoveRosterPlayer(ctx context.Context, params RosterEntryServiceParams) error {
	if err := closeRosterRow(ctx, r.conn(ctx), params); err != nil {
		return fmt.Errorf("teams: remove roster player %s from team %s: %w", params.PlayerID, params.TeamID, err)
	}
	return nil
}

// TransferRosterPlayer closes the source row and opens the target row in one
// transaction, so the player is never on both rosters nor on neither. Closing first
// frees the player's place in the division for a transfer within it.
func (r *repository) TransferRosterPlayer(ctx context.Context, params TransferRosterPlayerServiceParams) error {
	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
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

func insertRosterRow(ctx context.Context, conn db.Execer, params RosterEntryServiceParams) error {
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

	tag, err := conn.Exec(ctx, query, args...)
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

func closeRosterRow(ctx context.Context, conn db.Execer, params RosterEntryServiceParams) error {
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

	tag, err := conn.Exec(ctx, query, args...)
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
