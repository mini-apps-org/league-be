package sessions

import (
	"errors"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

var (
	ErrLeagueSessionNotFound       = errors.New("sessions: league session not found")
	ErrDuplicateLeagueSessionName  = errors.New("sessions: duplicate league session name")
	ErrInvalidLeagueSession        = errors.New("sessions: league session breaks a schema check")
	ErrLeagueSessionActiveConflict = errors.New("sessions: another league session became active concurrently")
	ErrLeagueSessionHasSharedTeams = errors.New("sessions: league session homes teams that play in another session")
	ErrNoDefaultLeague             = errors.New("sessions: not exactly one active league")
)

// leagueSessionErrorMappings is how the service serves each sentinel the repository returns.
var leagueSessionErrorMappings = []handler.ErrorMapping{
	{Sentinel: ErrLeagueSessionNotFound, Code: handler.CodeNotFound, Message: "league session not found"},
	{Sentinel: ErrDuplicateLeagueSessionName, Code: handler.CodeConflict, Message: "a league session with that name already exists"},
	{Sentinel: ErrInvalidLeagueSession, Code: handler.CodeValidationFailed},
	{Sentinel: ErrLeagueSessionActiveConflict, Code: handler.CodeConflict, Message: "another league session was made active at the same time; try again"},
	{Sentinel: ErrLeagueSessionHasSharedTeams, Code: handler.CodeConflict, Message: "teams created in this session also play in another session; move or remove them first"},
	{Sentinel: ErrNoDefaultLeague, Code: handler.CodeInternal},
}
