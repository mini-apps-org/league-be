package teams

import (
	"time"

	"github.com/nguyentanvinh/league-be/internal/common"
)

type CreateTeamRequest struct {
	DivisionID string  `json:"division_id" example:"7b1f0a52-2c6e-4d0a-9a3e-1f0c2b9d4e11"`
	TeamCode   string  `json:"team_code" example:"41503"`
	Name       string  `json:"name" example:"Cue Masters"`
	VenueID    *string `json:"venue_id,omitempty"`
}

type UpdateTeamRequest struct {
	TeamCode        *string `json:"team_code,omitempty" example:"41503"`
	Name            *string `json:"name,omitempty" example:"Cue Masters"`
	CaptainPlayerID *string `json:"captain_player_id,omitempty"`
	VenueID         *string `json:"venue_id,omitempty"`
}

type AddDivisionTeamRequest struct {
	DivisionID string `json:"division_id" example:"7b1f0a52-2c6e-4d0a-9a3e-1f0c2b9d4e11"`
}

type AddRosterPlayerRequest struct {
	PlayerID string `json:"player_id" example:"2d9c4b1e-8f3a-4c57-b6d2-0e7a1f5c9b30"`
}

// TransferRosterPlayerRequest moves a player to TeamID in DivisionID, or in the
// player's current division when DivisionID is absent.
type TransferRosterPlayerRequest struct {
	TeamID     string `json:"team_id" example:"5e8a2c1d-3b7f-4a90-8c6e-2d1f0b9a7e44"`
	DivisionID string `json:"division_id,omitempty"`
}

type TeamResponse struct {
	ID              string    `json:"id"`
	TeamCode        string    `json:"team_code"`
	Name            string    `json:"name"`
	CaptainPlayerID *string   `json:"captain_player_id"`
	VenueID         *string   `json:"venue_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type DivisionTeamResponse struct {
	DivisionID string    `json:"division_id"`
	TeamID     string    `json:"team_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type RosterPlayerResponse struct {
	RosterID      string    `json:"roster_id"`
	DivisionID    string    `json:"division_id"`
	TeamID        string    `json:"team_id"`
	PlayerID      string    `json:"player_id"`
	MemberID      string    `json:"member_id"`
	FullName      string    `json:"full_name"`
	SkillLevel    int16     `json:"skill_level"`
	IsPaid        bool      `json:"is_paid"`
	IsCaptain     bool      `json:"is_captain"`
	MatchesPlayed int32     `json:"matches_played"`
	MatchesWon    int32     `json:"matches_won"`
	TotalPoints   int32     `json:"total_points"`
	JoinedAt      time.Time `json:"joined_at"`
}

// The newXResponse mappers copy fields by name; TestResponsesCopyEveryField fails if a
// rename leaves a response field unfilled.

func newTeamResponse(team Team) TeamResponse {
	return common.CopyFieldsByName[TeamResponse](&team)
}

func newDivisionTeamResponse(entry DivisionTeam) DivisionTeamResponse {
	return common.CopyFieldsByName[DivisionTeamResponse](&entry)
}

func newRosterPlayerResponse(player RosterPlayer) RosterPlayerResponse {
	return common.CopyFieldsByName[RosterPlayerResponse](&player)
}
