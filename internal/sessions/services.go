package sessions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

const maxSearchQueryLength = 100

type Service interface {
	ListLeagueSessions(ctx context.Context, params ListLeagueSessionsServiceParams) ([]LeagueSession, error)
	GetLeagueSessionByID(ctx context.Context, id string) (*LeagueSession, error)
	CreateLeagueSession(ctx context.Context, params CreateLeagueSessionServiceParams) (*LeagueSession, error)
	UpdateLeagueSession(ctx context.Context, id string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error)
	DeleteLeagueSession(ctx context.Context, id string) error
	ListLeagueSessionWeeks(ctx context.Context, id string) ([]LeagueSessionWeek, error)
}

type service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) Service {
	return &service{repo: repo, logger: logger.With(slog.String("component", "sessions.service"))}
}

func (s *service) ListLeagueSessions(ctx context.Context, params ListLeagueSessionsServiceParams) ([]LeagueSession, error) {
	params.Query = strings.TrimSpace(params.Query)
	if utf8.RuneCountInString(params.Query) > maxSearchQueryLength {
		return nil, newValidationError(fmt.Sprintf("q must be at most %d characters", maxSearchQueryLength))
	}
	if params.SessionType != "" {
		if err := validateLeagueSessionType(params.SessionType); err != nil {
			return nil, err
		}
	}
	if params.SeasonYear != 0 {
		if err := validateSeasonYear(params.SeasonYear); err != nil {
			return nil, err
		}
	}
	if params.Status != "" {
		if err := validateLeagueSessionStatus(params.Status); err != nil {
			return nil, err
		}
	}

	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	sessions, err := s.repo.ListLeagueSessions(ctx, leagueID, params)
	return sessions, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) GetLeagueSessionByID(ctx context.Context, id string) (*LeagueSession, error) {
	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.repo.GetLeagueSessionByID(ctx, leagueID, id)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) CreateLeagueSession(ctx context.Context, params CreateLeagueSessionServiceParams) (*LeagueSession, error) {
	params.Name = strings.TrimSpace(params.Name)
	if params.Status == "" {
		params.Status = LeagueSessionStatusDraft
	}

	if err := validateLeagueSession(leagueSessionFields{
		Name:        params.Name,
		SessionType: params.SessionType,
		SeasonYear:  params.SeasonYear,
		StartsOn:    params.StartsOn,
		EndsOn:      params.EndsOn,
		Status:      params.Status,
	}); err != nil {
		return nil, err
	}

	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.repo.CreateLeagueSession(ctx, leagueID, params)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// UpdateLeagueSession validates the merged result, not just the supplied fields: a PATCH
// that moves only ends_on can still put it before the stored starts_on.
func (s *service) UpdateLeagueSession(ctx context.Context, id string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error) {
	if params == (UpdateLeagueSessionServiceParams{}) {
		return nil, newValidationError("supply at least one of name, session_type, season_year, starts_on, ends_on, status or is_active")
	}
	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		params.Name = &trimmed
	}

	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.GetLeagueSessionByID(ctx, leagueID, id)
	if err != nil {
		return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
	}
	if err := validateLeagueSession(mergeLeagueSessionFields(*current, params)); err != nil {
		return nil, err
	}

	session, err := s.repo.UpdateLeagueSession(ctx, leagueID, id, params)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) DeleteLeagueSession(ctx context.Context, id string) error {
	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return err
	}

	err = s.repo.DeleteLeagueSession(ctx, leagueID, id)
	return handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) ListLeagueSessionWeeks(ctx context.Context, id string) ([]LeagueSessionWeek, error) {
	leagueID, err := s.defaultLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	weeks, err := s.repo.ListLeagueSessionWeeks(ctx, leagueID, id)
	return weeks, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// defaultLeagueID stands in for league scoping while the MVP runs a single league; it
// must be replaced before a second league exists.
func (s *service) defaultLeagueID(ctx context.Context) (string, error) {
	leagueID, err := s.repo.GetDefaultLeagueID(ctx)
	return leagueID, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// leagueSessionFields is every validated field of a session, whether it comes from a
// create body or from a stored row merged with a PATCH.
type leagueSessionFields struct {
	Name        string
	SessionType LeagueSessionType
	SeasonYear  int
	StartsOn    time.Time
	EndsOn      time.Time
	Status      LeagueSessionStatus
}

func mergeLeagueSessionFields(current LeagueSession, params UpdateLeagueSessionServiceParams) leagueSessionFields {
	fields := leagueSessionFields{
		Name:        current.Name,
		SessionType: current.SessionType,
		SeasonYear:  int(current.SeasonYear),
		StartsOn:    current.StartsOn,
		EndsOn:      current.EndsOn,
		Status:      current.Status,
	}
	if params.Name != nil {
		fields.Name = *params.Name
	}
	if params.SessionType != nil {
		fields.SessionType = *params.SessionType
	}
	if params.SeasonYear != nil {
		fields.SeasonYear = *params.SeasonYear
	}
	if params.StartsOn != nil {
		fields.StartsOn = *params.StartsOn
	}
	if params.EndsOn != nil {
		fields.EndsOn = *params.EndsOn
	}
	if params.Status != nil {
		fields.Status = *params.Status
	}
	return fields
}

// validateLeagueSession repeats the schema's checks so a bad request gets a clear
// message instead of a generic one from the constraint.
func validateLeagueSession(fields leagueSessionFields) error {
	if length := utf8.RuneCountInString(fields.Name); length < leagueSessionNameMinLength || length > leagueSessionNameMaxLength {
		return newValidationError(fmt.Sprintf("name must be %d to %d characters",
			leagueSessionNameMinLength, leagueSessionNameMaxLength))
	}
	if err := validateLeagueSessionType(fields.SessionType); err != nil {
		return err
	}
	if err := validateSeasonYear(fields.SeasonYear); err != nil {
		return err
	}
	if err := validateLeagueSessionStatus(fields.Status); err != nil {
		return err
	}
	if fields.EndsOn.Before(fields.StartsOn) {
		return newValidationError("ends_on must be on or after starts_on")
	}
	return nil
}

func validateLeagueSessionType(sessionType LeagueSessionType) error {
	if !slices.Contains(leagueSessionTypes, sessionType) {
		return newValidationError("session_type must be one of SPRING, SUMMER, FALL")
	}
	return nil
}

func validateLeagueSessionStatus(status LeagueSessionStatus) error {
	if !slices.Contains(leagueSessionStatuses, status) {
		return newValidationError("status must be one of DRAFT, IN_PROGRESS, COMPLETE, CANCELLED")
	}
	return nil
}

func validateSeasonYear(year int) error {
	if year < leagueSessionMinSeasonYear || year > leagueSessionMaxSeasonYear {
		return newValidationError(fmt.Sprintf("season_year must be between %d and %d",
			leagueSessionMinSeasonYear, leagueSessionMaxSeasonYear))
	}
	return nil
}

func newValidationError(message string) error {
	return handler.ErrorWithMessage(handler.CodeValidationFailed, message, nil)
}
