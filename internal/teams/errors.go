package teams

import (
	"errors"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

var (
	// Teams
	ErrTeamNotFound          = errors.New("teams: team not found")
	ErrTeamNotSoftDeleted    = errors.New("teams: team not soft-deleted")
	ErrDuplicateTeamCode     = errors.New("teams: duplicate team code")
	ErrDivisionNotFound      = errors.New("teams: division not found")
	ErrVenueNotFound         = errors.New("teams: venue not found")
	ErrTeamHasMatches        = errors.New("teams: team has matches")
	ErrTeamNotInDivision     = errors.New("teams: team has not entered division")
	ErrTeamAlreadyInDivision = errors.New("teams: team already entered division")

	// Rosters
	ErrPlayerNotFound          = errors.New("teams: player not found")
	ErrRosterPlayerNotFound    = errors.New("teams: player not on active roster")
	ErrPlayerAlreadyInDivision = errors.New("teams: player already on an active roster in division")
)

// teamErrorMappings is how the service serves each sentinel the repository
// returns. Team and roster writes share constraintErrors, so one table covers both.
var teamErrorMappings = []handler.ErrorMapping{
	// Teams
	{Sentinel: ErrTeamNotFound, Code: handler.CodeNotFound, Message: "team not found"},
	{Sentinel: ErrTeamNotInDivision, Code: handler.CodeNotFound, Message: "the team has not entered this division"},
	{Sentinel: ErrDivisionNotFound, Code: handler.CodeValidationFailed, Message: "division_id does not match a division"},
	{Sentinel: ErrVenueNotFound, Code: handler.CodeValidationFailed, Message: "venue_id does not match a venue"},
	{Sentinel: ErrDuplicateTeamCode, Code: handler.CodeConflict, Message: "a team with that team_code already exists"},
	{Sentinel: ErrTeamAlreadyInDivision, Code: handler.CodeConflict, Message: "the team has already entered this division"},
	{Sentinel: ErrTeamNotSoftDeleted, Code: handler.CodeConflict, Message: "delete the team before deleting it permanently"},
	{Sentinel: ErrTeamHasMatches, Code: handler.CodeConflict, Message: "the team has scheduled or played matches"},

	// Rosters
	{Sentinel: ErrPlayerNotFound, Code: handler.CodeNotFound, Message: "player not found"},
	{Sentinel: ErrRosterPlayerNotFound, Code: handler.CodeNotFound, Message: "player is not on this team's active roster in the division"},
	{Sentinel: ErrPlayerAlreadyInDivision, Code: handler.CodeConflict, Message: "the player is already on an active roster in this division"},
}
