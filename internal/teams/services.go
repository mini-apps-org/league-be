package teams

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nguyentanvinh/league-be/internal/common"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

const maxTeamNameLength = 100

var (
	// teamCodePattern is the ID Generation Rules' XXyyA: region, host location, team
	// sequence. The schema does not check the format, so this is the only check.
	teamCodePattern = regexp.MustCompile(`^[0-9A-Z]{5}$`)

	// teamUpdatePermissionCode ("team.update") lifts the captain-only scope on roster writes: a
	// user who may update any team may change any team's roster. Captains are not granted it.
	teamUpdatePermissionCode = config.GetPermissionByFeatureAndAction(config.PermissionFeatureTeams, config.PermissionActionUpdate)
)

// PermissionReader is the slice of permissions.Service the roster rules need.
type PermissionReader interface {
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type Service interface {
	CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error)
	GetTeam(ctx context.Context, id string) (*Team, error)
	ListTeams(ctx context.Context, params ListTeamsServiceParams) ([]Team, error)
	UpdateTeam(ctx context.Context, id string, params UpdateTeamServiceParams) (*Team, error)
	DeleteTeam(ctx context.Context, id string) error
	HardDeleteTeam(ctx context.Context, id string) error

	AddDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error)
	RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error

	ListRosterPlayers(ctx context.Context, divisionID, teamID string) ([]RosterPlayer, error)
	AddRosterPlayer(ctx context.Context, actorUserID string, params RosterEntryServiceParams) (*RosterPlayer, error)
	RemoveRosterPlayer(ctx context.Context, actorUserID string, params RosterEntryServiceParams) error
	TransferRosterPlayer(ctx context.Context, actorUserID string, params TransferRosterPlayerServiceParams) (*RosterPlayer, error)
}

type service struct {
	repo             Repository
	permissionReader PermissionReader
}

func NewService(repo Repository, permissionReader PermissionReader) Service {
	return &service{repo: repo, permissionReader: permissionReader}
}

func (s *service) CreateTeam(ctx context.Context, params CreateTeamServiceParams) (*Team, error) {
	params.TeamCode = normaliseTeamCode(params.TeamCode)
	params.Name = strings.TrimSpace(params.Name)

	if err := validateUUIDByFieldAndValue("division_id", params.DivisionID); err != nil {
		return nil, err
	}
	if err := validateTeamFields(&params.TeamCode, &params.Name); err != nil {
		return nil, err
	}
	if err := validateOptionalUUIDByFieldAndValue("venue_id", params.VenueID); err != nil {
		return nil, err
	}

	team, err := s.repo.CreateTeam(ctx, params)
	return team, asAPIError(err)
}

func (s *service) GetTeam(ctx context.Context, id string) (*Team, error) {
	team, err := s.repo.GetTeamByID(ctx, id)
	return team, asAPIError(err)
}

func (s *service) ListTeams(ctx context.Context, params ListTeamsServiceParams) ([]Team, error) {
	if params.DivisionID != "" {
		if err := validateUUIDByFieldAndValue("division_id", params.DivisionID); err != nil {
			return nil, err
		}
	}

	return s.repo.ListTeams(ctx, params)
}

func (s *service) UpdateTeam(ctx context.Context, id string, params UpdateTeamServiceParams) (*Team, error) {
	if params.TeamCode == nil && params.Name == nil && params.CaptainPlayerID == nil && params.VenueID == nil {
		return nil, newValidationError("supply at least one of team_code, name, captain_player_id or venue_id")
	}
	if params.TeamCode != nil {
		normalised := normaliseTeamCode(*params.TeamCode)
		params.TeamCode = &normalised
	}
	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		params.Name = &trimmed
	}

	if err := validateTeamFields(params.TeamCode, params.Name); err != nil {
		return nil, err
	}
	if err := validateOptionalUUIDByFieldAndValue("venue_id", params.VenueID); err != nil {
		return nil, err
	}
	if err := validateOptionalUUIDByFieldAndValue("captain_player_id", params.CaptainPlayerID); err != nil {
		return nil, err
	}

	if params.CaptainPlayerID != nil {
		isActive, err := s.repo.IsActiveTeamPlayer(ctx, id, *params.CaptainPlayerID)
		if err != nil {
			return nil, err
		}
		if !isActive {
			return nil, newValidationError("captain_player_id must be a player on the team's active roster")
		}
	}

	team, err := s.repo.UpdateTeam(ctx, id, params)
	return team, asAPIError(err)
}

func (s *service) DeleteTeam(ctx context.Context, id string) error {
	return asAPIError(s.repo.SoftDeleteTeam(ctx, id))
}

func (s *service) HardDeleteTeam(ctx context.Context, id string) error {
	return asAPIError(s.repo.HardDeleteTeam(ctx, id))
}

func (s *service) AddDivisionTeam(ctx context.Context, divisionID, teamID string) (*DivisionTeam, error) {
	if err := validateUUIDByFieldAndValue("division_id", divisionID); err != nil {
		return nil, err
	}

	entry, err := s.repo.AddDivisionTeam(ctx, divisionID, teamID)
	return entry, asAPIError(err)
}

func (s *service) RemoveDivisionTeam(ctx context.Context, divisionID, teamID string) error {
	return asAPIError(s.repo.RemoveDivisionTeam(ctx, divisionID, teamID))
}

func (s *service) ListRosterPlayers(ctx context.Context, divisionID, teamID string) ([]RosterPlayer, error) {
	if err := validateUUIDByFieldAndValue("division_id", divisionID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetDivisionTeam(ctx, divisionID, teamID); err != nil {
		return nil, asAPIError(err)
	}

	return s.repo.ListRosterPlayers(ctx, divisionID, teamID)
}

func (s *service) AddRosterPlayer(ctx context.Context, actorUserID string, params RosterEntryServiceParams) (*RosterPlayer, error) {
	if err := validateUUIDByFieldAndValue("division_id", params.DivisionID); err != nil {
		return nil, err
	}
	if err := validateUUIDByFieldAndValue("player_id", params.PlayerID); err != nil {
		return nil, err
	}

	canManageAllRosters, err := s.authorizeRosterWrite(ctx, actorUserID, params.TeamID)
	if err != nil {
		return nil, err
	}
	if !canManageAllRosters {
		isPaid, err := s.repo.IsPaidPlayer(ctx, params.PlayerID)
		if err != nil {
			return nil, asAPIError(err)
		}
		if !isPaid {
			return nil, handler.ErrorWithMessage(handler.CodePermissionDenied,
				"a captain can add only paid members; ask a league operator to add this player", nil)
		}
	}

	if err := s.repo.AddRosterPlayer(ctx, params); err != nil {
		return nil, asAPIError(err)
	}

	player, err := s.repo.GetRosterPlayer(ctx, params)
	return player, asAPIError(err)
}

func (s *service) RemoveRosterPlayer(ctx context.Context, actorUserID string, params RosterEntryServiceParams) error {
	if err := validateUUIDByFieldAndValue("division_id", params.DivisionID); err != nil {
		return err
	}
	if _, err := s.authorizeRosterWrite(ctx, actorUserID, params.TeamID); err != nil {
		return err
	}
	if err := s.rejectCaptain(ctx, params.TeamID, params.PlayerID); err != nil {
		return err
	}

	return asAPIError(s.repo.RemoveRosterPlayer(ctx, params))
}

func (s *service) TransferRosterPlayer(ctx context.Context, actorUserID string, params TransferRosterPlayerServiceParams) (*RosterPlayer, error) {
	if params.ToDivisionID == "" {
		params.ToDivisionID = params.From.DivisionID
	}

	if err := validateUUIDByFieldAndValue("division_id", params.From.DivisionID); err != nil {
		return nil, err
	}
	if err := validateUUIDByFieldAndValue("team_id", params.ToTeamID); err != nil {
		return nil, err
	}
	if err := validateUUIDByFieldAndValue("target division_id", params.ToDivisionID); err != nil {
		return nil, err
	}
	if params.ToTeamID == params.From.TeamID && params.ToDivisionID == params.From.DivisionID {
		return nil, newValidationError("the target team and division must differ from the player's current ones")
	}
	// Both sides: otherwise a captain could pull a player off another team onto their own.
	if _, err := s.authorizeRosterWrite(ctx, actorUserID, params.From.TeamID, params.ToTeamID); err != nil {
		return nil, err
	}
	if err := s.rejectCaptain(ctx, params.From.TeamID, params.From.PlayerID); err != nil {
		return nil, err
	}

	if err := s.repo.TransferRosterPlayer(ctx, params); err != nil {
		return nil, asAPIError(err)
	}

	player, err := s.repo.GetRosterPlayer(ctx, RosterEntryServiceParams{
		DivisionID: params.ToDivisionID,
		TeamID:     params.ToTeamID,
		PlayerID:   params.From.PlayerID,
	})
	return player, asAPIError(err)
}

// authorizeRosterWrite allows a roster change on teamIDs when the user holds
// team.update, or else captains every one of them. It reports which applied, since
// a captain is held to further rules.
func (s *service) authorizeRosterWrite(ctx context.Context, actorUserID string, teamIDs ...string) (canManageAllRosters bool, err error) {
	permissionCodes, err := s.permissionReader.GetPermissionCodesByUserID(ctx, actorUserID)
	if err != nil {
		return false, err
	}
	if slices.Contains(permissionCodes, teamUpdatePermissionCode) {
		return true, nil
	}

	actorPlayerID, err := s.repo.GetPlayerIDByUserID(ctx, actorUserID)
	if errors.Is(err, ErrPlayerNotFound) {
		return false, notTeamCaptainError()
	}
	if err != nil {
		return false, err
	}

	for _, teamID := range teamIDs {
		team, err := s.repo.GetTeamByID(ctx, teamID)
		if err != nil {
			return false, asAPIError(err)
		}
		if team.CaptainPlayerID == nil || *team.CaptainPlayerID != actorPlayerID {
			return false, notTeamCaptainError()
		}
	}
	return false, nil
}

func notTeamCaptainError() error {
	return handler.ErrorWithMessage(handler.CodePermissionDenied,
		"only the team captain or a league operator can change this roster", nil)
}

// rejectCaptain stops the captain leaving a roster: teams.captain_player_id would then
// name a player who cannot be put up.
func (s *service) rejectCaptain(ctx context.Context, teamID, playerID string) error {
	team, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return asAPIError(err)
	}
	if team.CaptainPlayerID != nil && *team.CaptainPlayerID == playerID {
		return handler.ErrorWithMessage(handler.CodeConflict,
			"the player is the team captain; assign another captain first", nil)
	}
	return nil
}

// normaliseTeamCode accepts a code typed in lower case; the stored form is upper case.
func normaliseTeamCode(teamCode string) string {
	return strings.ToUpper(strings.TrimSpace(teamCode))
}

// validateTeamFields checks the supplied (non-nil) fields. Lengths count runes, not
// bytes, so Vietnamese names are not cut short.
func validateTeamFields(teamCode, name *string) error {
	if teamCode != nil && !teamCodePattern.MatchString(*teamCode) {
		return newValidationError("team_code must be 5 characters of 0-9 and A-Z: region, host location, team sequence")
	}
	if name != nil {
		if *name == "" {
			return newValidationError("name is required")
		}
		if utf8.RuneCountInString(*name) > maxTeamNameLength {
			return newValidationError(fmt.Sprintf("name must be at most %d characters", maxTeamNameLength))
		}
	}
	return nil
}

// validateUUIDByFieldAndValue stops a malformed id before Postgres rejects the cast with a 500.
func validateUUIDByFieldAndValue(field, value string) error {
	if err := common.ValidateUUID(value); err != nil {
		return handler.ErrorWithMessage(handler.CodeValidationFailed, field+" must be a uuid", err)
	}
	return nil
}

func validateOptionalUUIDByFieldAndValue(field string, value *string) error {
	if value == nil {
		return nil
	}
	return validateUUIDByFieldAndValue(field, *value)
}

func newValidationError(message string) error {
	return handler.ErrorWithMessage(handler.CodeValidationFailed, message, nil)
}

// TODO vincent: will refactor later
func asAPIError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrTeamNotFound):
		return handler.ErrorWithMessage(handler.CodeNotFound, "team not found", err)
	case errors.Is(err, ErrPlayerNotFound):
		return handler.ErrorWithMessage(handler.CodeNotFound, "player not found", err)
	case errors.Is(err, ErrTeamNotInDivision):
		return handler.ErrorWithMessage(handler.CodeNotFound, "the team has not entered this division", err)
	case errors.Is(err, ErrRosterPlayerNotFound):
		return handler.ErrorWithMessage(handler.CodeNotFound, "player is not on this team's active roster in the division", err)
	case errors.Is(err, ErrDivisionNotFound):
		return handler.ErrorWithMessage(handler.CodeValidationFailed, "division_id does not match a division", err)
	case errors.Is(err, ErrVenueNotFound):
		return handler.ErrorWithMessage(handler.CodeValidationFailed, "venue_id does not match a venue", err)
	case errors.Is(err, ErrDuplicateTeamCode):
		return handler.ErrorWithMessage(handler.CodeConflict, "a team with that team_code already exists", err)
	case errors.Is(err, ErrTeamAlreadyInDivision):
		return handler.ErrorWithMessage(handler.CodeConflict, "the team has already entered this division", err)
	case errors.Is(err, ErrTeamNotSoftDeleted):
		return handler.ErrorWithMessage(handler.CodeConflict, "delete the team before deleting it permanently", err)
	case errors.Is(err, ErrTeamHasMatches):
		return handler.ErrorWithMessage(handler.CodeConflict, "the team has scheduled or played matches", err)
	case errors.Is(err, ErrPlayerAlreadyInDivision):
		return handler.ErrorWithMessage(handler.CodeConflict, "the player is already on an active roster in this division", err)
	default:
		return err
	}
}
