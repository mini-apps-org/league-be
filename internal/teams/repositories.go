package teams

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamNotFound            = errors.New("teams: team not found")
	ErrTeamNotSoftDeleted      = errors.New("teams: team not soft-deleted")
	ErrDuplicateTeamCode       = errors.New("teams: duplicate team code")
	ErrDivisionNotFound        = errors.New("teams: division not found")
	ErrVenueNotFound           = errors.New("teams: venue not found")
	ErrTeamHasMatches          = errors.New("teams: team has matches")
	ErrTeamNotInDivision       = errors.New("teams: team has not entered division")
	ErrTeamAlreadyInDivision   = errors.New("teams: team already entered division")
	ErrPlayerNotFound          = errors.New("teams: player not found")
	ErrRosterPlayerNotFound    = errors.New("teams: player not on active roster")
	ErrPlayerAlreadyInDivision = errors.New("teams: player already on an active roster in division")
)

// TODO vincent: Need write as the array format and using String.joins to easier remove or add the new field as needed
const teamColumns = `id, team_code, name, captain_player_id, venue_id, created_at, updated_at`

const divisionTeamColumns = `division_id, team_id, created_at`

type Repository interface {
	CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error)
	GetTeamByID(ctx context.Context, id string) (*Team, error)
	ListTeams(ctx context.Context, params ListTeamsServiceParams) ([]Team, error)
	CountTeams(ctx context.Context, params ListTeamsServiceParams) (int64, error)
	UpdateTeam(ctx context.Context, id string, params UpdateTeamServiceParams) (*Team, error)
	SoftDeleteTeam(ctx context.Context, id string) error
	PermanentDeleteTeam(ctx context.Context, id string) error

	AddDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error)
	GetDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error)
	RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error

	ListRosterPlayers(ctx context.Context, params ListRosterPlayersServiceParams) ([]RosterPlayer, error)
	CountRosterPlayers(ctx context.Context, params ListRosterPlayersServiceParams) (int64, error)
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
	logger *slog.Logger
}

func NewRepo(pool *pgxpool.Pool, logger *slog.Logger) Repository {
	return &repository{pgPool: pool, logger: logger.With(slog.String("component", "teams.repository"))}
}
