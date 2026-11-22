// Package teams owns team CRUD, a team's entries into divisions, and its roster in each.
package teams

import "time"

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

type ListTeamsServiceParams struct {
	DivisionID string
}

type CreateTeamServiceParams struct {
	DivisionID string // required: teams.division_id is NOT NULL
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
