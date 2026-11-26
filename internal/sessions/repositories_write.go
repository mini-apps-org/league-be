package sessions

// Writes: create, update, and soft and permanent delete, singly and in bulk. A constraint a write trips is mapped to its
// sentinel by constraintError.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
WHERE league_id = $1 AND id = $2 AND NOT is_deleted`
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

// SoftDeleteSession hides the session and clears is_active, so a deleted session never
// holds the league's one active slot. Its divisions, teams and matches stay.
func (r *repository) SoftDeleteSession(ctx context.Context, leagueID, leagueSessionID string) error {
	const query = `
UPDATE league_sessions SET
    is_deleted = true,
    is_active  = false,
    updated_at = now()
WHERE league_id = $1 AND id = $2 AND NOT is_deleted`
	args := []any{
		leagueID,        // $1 league_id
		leagueSessionID, // $2 id
	}

	tag, err := r.conn(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("sessions: soft delete league session %s: %w", leagueSessionID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeagueSessionNotFound
	}
	return nil
}

// leagueSessionHomesSharedTeams is true for the session ls when it homes a team
// (teams.division_id) that has also entered a division of another session. A permanent
// delete refuses such a session: the cascade through teams.division_id would delete that
// team from the other session too, roster included.
const leagueSessionHomesSharedTeams = `EXISTS (
    SELECT 1
    FROM teams t
    JOIN divisions home    ON home.id = t.division_id
    JOIN division_teams dt ON dt.team_id = t.id
    JOIN divisions other   ON other.id = dt.division_id
    WHERE home.season_id = ls.id AND other.season_id <> ls.id
)`

// leagueSessionDeleteState is what a permanent delete reads to decide, or to explain, why a
// session stays.
type leagueSessionDeleteState struct {
	ID             string `db:"id"`
	SessionExists  bool   `db:"session_exists"`
	IsDeleted      bool   `db:"is_deleted"`
	HasSharedTeams bool   `db:"has_shared_teams"`
}

// deleteFailure is the sentinel the single permanent delete returns for a session in
// this state, or nil when it may be deleted.
func (state leagueSessionDeleteState) deleteFailure() error {
	switch {
	case !state.SessionExists:
		return ErrLeagueSessionNotFound
	case !state.IsDeleted:
		return ErrLeagueSessionNotSoftDeleted
	case state.HasSharedTeams:
		return ErrLeagueSessionHasSharedTeams
	default:
		return nil
	}
}

// PermanentDeleteSession removes a soft-deleted session that homes no shared team (see
// leagueSessionHomesSharedTeams).
func (r *repository) PermanentDeleteSession(ctx context.Context, leagueID, leagueSessionID string) error {
	// FOR UPDATE serialises two permanent deletes of the same session, so the second
	// answers 404 rather than racing the first through the checks.
	const stateQuery = `
SELECT
    ls.id                                    AS id,
    true                                     AS session_exists,
    ls.is_deleted                            AS is_deleted,
    ` + leagueSessionHomesSharedTeams + ` AS has_shared_teams
FROM league_sessions ls
WHERE ls.league_id = $1 AND ls.id = $2
FOR UPDATE`
	const deleteQuery = `DELETE FROM league_sessions WHERE league_id = $1 AND id = $2 AND is_deleted`
	args := []any{
		leagueID,        // $1 league_id
		leagueSessionID, // $2 id
	}

	err := pgx.BeginFunc(ctx, r.conn(ctx), func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, stateQuery, args...)
		if err != nil {
			return err
		}
		state, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[leagueSessionDeleteState])
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLeagueSessionNotFound
		}
		if err != nil {
			return err
		}
		if failure := state.deleteFailure(); failure != nil {
			return failure
		}

		_, err = tx.Exec(ctx, deleteQuery, args...)
		return err
	})
	if errors.Is(err, ErrLeagueSessionNotFound) ||
		errors.Is(err, ErrLeagueSessionNotSoftDeleted) ||
		errors.Is(err, ErrLeagueSessionHasSharedTeams) {
		return err
	}
	if sentinel := constraintError(err); sentinel != nil {
		return sentinel
	}
	if err != nil {
		return fmt.Errorf("sessions: permanently delete league session %s: %w", leagueSessionID, err)
	}
	return nil
}

// BulkSoftDeleteSessions soft-deletes every live session of the league in ids, in one
// statement. A requested session it did not delete was missing or already deleted:
// ErrLeagueSessionNotFound, as SoftDeleteSession reports. ids must be canonical uuids.
func (r *repository) BulkSoftDeleteSessions(ctx context.Context, leagueID string, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	const query = `
UPDATE league_sessions SET
    is_deleted = true,
    is_active  = false,
    updated_at = now()
WHERE league_id = $1 AND id = ANY($2::uuid[]) AND NOT is_deleted
RETURNING id`
	args := []any{
		leagueID, // $1 league_id
		ids,      // $2 session ids
	}

	rows, err := r.conn(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sessions: bulk soft delete league sessions: %w", err)
	}
	deletedIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("sessions: bulk soft delete league sessions: %w", err)
	}

	return newBulkDeleteLeagueSessionsResult(ids, deletedIDs, func(string) error { return ErrLeagueSessionNotFound }), nil
}

// BulkPermanentDeleteSessions removes every soft-deleted session of the league in ids that
// homes no shared team, then reads why each other one stayed, as PermanentDeleteSession
// would report it. Two statements in one transaction. ids must be canonical uuids.
func (r *repository) BulkPermanentDeleteSessions(ctx context.Context, leagueID string, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	// A session with shared teams is skipped here rather than left to the team_matches
	// foreign keys: a violation aborts the whole statement, and with it every other session.
	const deleteQuery = `
DELETE FROM league_sessions ls
WHERE ls.league_id = $1
  AND ls.id = ANY($2::uuid[])
  AND ls.is_deleted
  AND NOT ` + leagueSessionHomesSharedTeams + `
RETURNING ls.id`
	const stateQuery = `
SELECT
    requested.id                             AS id,
    ls.id IS NOT NULL                        AS session_exists,
    COALESCE(ls.is_deleted, false)           AS is_deleted,
    ` + leagueSessionHomesSharedTeams + ` AS has_shared_teams
FROM unnest($2::uuid[]) AS requested (id)
LEFT JOIN league_sessions ls ON ls.id = requested.id AND ls.league_id = $1`
	args := []any{
		leagueID, // $1 league_id
		ids,      // $2 session ids
	}

	var (
		deletedIDs []string
		states     = map[string]leagueSessionDeleteState{}
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
		found, err := pgx.CollectRows(rows, pgx.RowToStructByName[leagueSessionDeleteState])
		if err != nil {
			return err
		}
		for _, state := range found {
			states[state.ID] = state
		}
		return nil
	})
	// Only a team entering another session between the NOT EXISTS and the cascade reaches
	// the foreign keys: the whole batch then fails as ErrLeagueSessionHasSharedTeams.
	if sentinel := constraintError(err); sentinel != nil {
		return nil, sentinel
	}
	if err != nil {
		return nil, fmt.Errorf("sessions: bulk permanently delete league sessions: %w", err)
	}

	return newBulkDeleteLeagueSessionsResult(ids, deletedIDs, func(id string) error {
		if failure := states[id].deleteFailure(); failure != nil {
			return failure
		}
		// Deletable now but not when the DELETE ran: a session whose last shared team left
		// in between. Retrying deletes it.
		return ErrLeagueSessionHasSharedTeams
	}), nil
}

// newBulkDeleteLeagueSessionsResult lists each requested id exactly once, in request
// order: in DeletedIDs when the statement returned it, otherwise in Failures with
// failureOf(id).
func newBulkDeleteLeagueSessionsResult(requestedIDs, deletedIDs []string, failureOf func(id string) error) *BulkDeleteLeagueSessionsResult {
	deleted := make(map[string]bool, len(deletedIDs))
	for _, id := range deletedIDs {
		deleted[id] = true
	}

	result := &BulkDeleteLeagueSessionsResult{DeletedIDs: []string{}, Failures: []BulkDeleteLeagueSessionFailure{}}
	for _, id := range requestedIDs {
		if deleted[id] {
			result.DeletedIDs = append(result.DeletedIDs, id)
			continue
		}
		result.Failures = append(result.Failures, BulkDeleteLeagueSessionFailure{LeagueSessionID: id, Err: failureOf(id)})
	}
	return result
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
