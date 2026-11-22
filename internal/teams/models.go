// Package teams owns team CRUD, a team's entries into divisions, and its roster in each.
package teams

import (
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

// Fields a team list can be sorted by: ?sort=name, or ?sort=-name for descending.
const (
	TeamSortByTeamCode  = "team_code"
	TeamSortByName      = "name"
	TeamSortByCreatedAt = "created_at"
)

var teamSortFields = []string{TeamSortByTeamCode, TeamSortByName, TeamSortByCreatedAt}

// Fields a roster can be sorted by: the columns of the roster screen.
const (
	RosterSortByFullName      = "full_name"
	RosterSortByMemberID      = "member_id"
	RosterSortBySkillLevel    = "skill_level"
	RosterSortByMatchesPlayed = "matches_played"
	RosterSortByMatchesWon    = "matches_won"
	RosterSortByTotalPoints   = "total_points"
	RosterSortByJoinedAt      = "joined_at"
)

var rosterSortFields = []string{
	RosterSortByFullName, RosterSortByMemberID, RosterSortBySkillLevel,
	RosterSortByMatchesPlayed, RosterSortByMatchesWon, RosterSortByTotalPoints, RosterSortByJoinedAt,
}

// Team is permanent: one row and one team_code for life. It plays a season by entering
// a division (DivisionTeam).
type Team struct {
	ID              string    `db:"id"`
	TeamCode        string    `db:"team_code"`
	Name            string    `db:"name"`
	CaptainPlayerID *string   `db:"captain_player_id"`
	VenueID         *string   `db:"venue_id"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type DivisionTeam struct {
	DivisionID string    `db:"division_id"`
	TeamID     string    `db:"team_id"`
	CreatedAt  time.Time `db:"created_at"`
}

// RosterPlayer is an active team_rosters row joined with its player and the player's
// stats in that division. Stats are zero before the first match.
type RosterPlayer struct {
	RosterID      string    `db:"roster_id"`
	DivisionID    string    `db:"division_id"`
	TeamID        string    `db:"team_id"`
	PlayerID      string    `db:"player_id"`
	MemberID      string    `db:"member_id"`
	FullName      string    `db:"full_name"`
	SkillLevel    int16     `db:"skill_level"`
	IsPaid        bool      `db:"is_paid"`
	IsCaptain     bool      `db:"is_captain"`
	MatchesPlayed int32     `db:"matches_played"`
	MatchesWon    int32     `db:"matches_won"`
	TotalPoints   int32     `db:"total_points"`
	JoinedAt      time.Time `db:"joined_at"`
}

// ListTeamsServiceParams filters are ANDed; an empty one is not applied.
type ListTeamsServiceParams struct {
	DivisionID string
	VenueID    string
	Query      string // matches name or team_code, case-insensitive
	Sort       model.SortParams
	Page       model.CursorParams
}

type ListRosterPlayersServiceParams struct {
	DivisionID string
	TeamID     string
	Query      string // matches full_name or member_id, case-insensitive
	Sort       model.SortParams
	Page       model.CursorParams
}

// CreateTeamServiceParams creates a team in its home division: teams.division_id is NOT
// NULL, and the team is entered into that division in the same transaction.
type CreateTeamServiceParams struct {
	DivisionID string
	TeamCode   string
	Name       string
	VenueID    *string
}

// UpdateTeamServiceParams is a partial update: nil leaves a field unchanged.
type UpdateTeamServiceParams struct {
	TeamCode        *string
	Name            *string
	CaptainPlayerID *string
	VenueID         *string
}

// RosterEntryServiceParams names one player's place on a team within a division.
type RosterEntryServiceParams struct {
	DivisionID string
	TeamID     string
	PlayerID   string
}

type TransferRosterPlayerServiceParams struct {
	From         RosterEntryServiceParams
	ToDivisionID string
	ToTeamID     string
}

// BulkDeleteTeamsResult is the outcome of a best-effort bulk delete: every requested team
// is in exactly one of DeletedIDs and Failures.
type BulkDeleteTeamsResult struct {
	DeletedIDs []string
	Failures   []BulkDeleteTeamFailure
}

// BulkDeleteTeamFailure is one team a bulk delete could not delete. Err is what the single
// delete would have returned for it.
type BulkDeleteTeamFailure struct {
	TeamID string
	Err    error
}
