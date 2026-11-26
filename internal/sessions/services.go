package sessions

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nguyentanvinh/league-be/internal/common"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

const (
	maxSearchQueryLength    = 100
	maxBulkLeagueSessionIDs = 100
)

type Service interface {
	ListSessions(ctx context.Context, params ListLeagueSessionsServiceParams) (*model.CursorResponse[LeagueSession], error)
	GetSessionByID(ctx context.Context, leagueSessionID string) (*LeagueSession, error)
	CreateSession(ctx context.Context, params CreateLeagueSessionServiceParams) (*LeagueSession, error)
	UpdateSessionByID(ctx context.Context, leagueSessionID string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error)
	DeleteSessionByID(ctx context.Context, leagueSessionID string) error
	PermanentDeleteSession(ctx context.Context, leagueSessionID string) error
	BulkSoftDeleteSessions(ctx context.Context, ids []string) (*BulkDeleteLeagueSessionsResult, error)
	BulkPermanentDeleteSessions(ctx context.Context, ids []string) (*BulkDeleteLeagueSessionsResult, error)
	ListSessionWeeks(ctx context.Context, leagueSessionID string) ([]LeagueSessionWeek, error)
}

type service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) Service {
	return &service{repo: repo, logger: logger.With(slog.String("component", "sessions.service"))}
}

func (s *service) ListSessions(ctx context.Context, params ListLeagueSessionsServiceParams) (*model.CursorResponse[LeagueSession], error) {
	params.Query = strings.TrimSpace(params.Query)
	if utf8.RuneCountInString(params.Query) > maxSearchQueryLength {
		return nil, handler.ErrorWithMessage(handler.CodeValidationFailed, fmt.Sprintf("q must be at most %d characters", maxSearchQueryLength), nil)
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
	if err := normaliseLeagueSessionPage(&params.Page, params.Sort); err != nil {
		return nil, err
	}

	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	var total *int64
	if params.Page.After == nil {
		count, err := s.repo.CountSessions(ctx, leagueID, params)
		if err != nil {
			return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
		}
		total = &count
	}

	sessions, err := s.repo.ListSessions(ctx, leagueID, params)
	if err != nil {
		return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
	}
	page := model.NewCursorResponse(sessions, params.Page.Limit, func(session LeagueSession) model.Cursor {
		return model.Cursor{Sort: params.Sort.String(), LastValue: leagueSessionSortValue(session, params.Sort.Field), LastID: session.ID}
	})
	page.Total = total
	return page, nil
}

func (s *service) GetSessionByID(ctx context.Context, leagueSessionID string) (*LeagueSession, error) {
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.repo.GetSessionByID(ctx, leagueID, leagueSessionID)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) CreateSession(ctx context.Context, params CreateLeagueSessionServiceParams) (*LeagueSession, error) {
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

	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.repo.CreateSession(ctx, leagueID, params)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// UpdateSessionByID validates the merged result, not just the supplied fields: a PATCH
// that moves only ends_on can still put it before the stored starts_on.
func (s *service) UpdateSessionByID(ctx context.Context, leagueSessionID string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error) {
	if params == (UpdateLeagueSessionServiceParams{}) {
		return nil, handler.ErrorWithMessage(handler.CodeValidationFailed, "supply at least one of name, session_type, season_year, starts_on, ends_on, status or is_active", nil)
	}
	if params.Name != nil {
		trimmed := strings.TrimSpace(*params.Name)
		params.Name = &trimmed
	}

	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.GetSessionByID(ctx, leagueID, leagueSessionID)
	if err != nil {
		return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
	}
	if err := validateLeagueSession(mergeLeagueSessionFields(*current, params)); err != nil {
		return nil, err
	}

	session, err := s.repo.UpdateSession(ctx, leagueID, leagueSessionID, params)
	return session, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) DeleteSessionByID(ctx context.Context, leagueSessionID string) error {
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return err
	}

	err = s.repo.SoftDeleteSession(ctx, leagueID, leagueSessionID)
	return handler.MapFeatureError(err, leagueSessionErrorMappings)
}

func (s *service) PermanentDeleteSession(ctx context.Context, leagueSessionID string) error {
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return err
	}

	err = s.repo.PermanentDeleteSession(ctx, leagueID, leagueSessionID)
	return handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// BulkSoftDeleteSessions soft-deletes the sessions in one statement, best effort: a session
// that cannot be deleted is reported with the error DeleteSessionByID would return, and
// the rest still are.
func (s *service) BulkSoftDeleteSessions(ctx context.Context, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	ids, err := normaliseBulkLeagueSessionIDs(ids)
	if err != nil {
		return nil, err
	}
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.repo.BulkSoftDeleteSessions(ctx, leagueID, ids)
	if err != nil {
		return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
	}
	return withLeagueSessionAPIErrors(result), nil
}

// BulkPermanentDeleteSessions permanently deletes the soft-deleted sessions that home no
// shared team, best effort, each failure being the error PermanentDeleteSession would
// return.
func (s *service) BulkPermanentDeleteSessions(ctx context.Context, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	ids, err := normaliseBulkLeagueSessionIDs(ids)
	if err != nil {
		return nil, err
	}
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	result, err := s.repo.BulkPermanentDeleteSessions(ctx, leagueID, ids)
	if err != nil {
		return nil, handler.MapFeatureError(err, leagueSessionErrorMappings)
	}
	return withLeagueSessionAPIErrors(result), nil
}

// withLeagueSessionAPIErrors turns each failure's repository sentinel into the API error
// the single delete serves for it.
func withLeagueSessionAPIErrors(result *BulkDeleteLeagueSessionsResult) *BulkDeleteLeagueSessionsResult {
	for i := range result.Failures {
		result.Failures[i].Err = handler.MapFeatureError(result.Failures[i].Err, leagueSessionErrorMappings)
	}
	return result
}

// normaliseBulkLeagueSessionIDs rejects a malformed request as a whole — that is a client
// bug, not one session's outcome — puts each id in canonical form, and drops repeated ids,
// keeping their first position.
func normaliseBulkLeagueSessionIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, handler.ErrorWithMessage(handler.CodeValidationFailed, "ids must hold at least one league session id", nil)
	}

	unique := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for i, raw := range ids {
		// Canonical, so a repeat in another case is dropped and each id matches the form the
		// delete statements return.
		id, err := common.NormaliseUUID(raw)
		if err != nil {
			return nil, handler.ErrorWithMessage(handler.CodeValidationFailed, fmt.Sprintf("ids[%d] must be a uuid", i), nil)
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	if len(unique) > maxBulkLeagueSessionIDs {
		return nil, handler.ErrorWithMessage(handler.CodeValidationFailed,
			fmt.Sprintf("ids must hold at most %d league sessions", maxBulkLeagueSessionIDs), nil)
	}
	return unique, nil
}

func (s *service) ListSessionWeeks(ctx context.Context, leagueSessionID string) ([]LeagueSessionWeek, error) {
	leagueID, err := s.resolveSingleLeagueID(ctx)
	if err != nil {
		return nil, err
	}

	weeks, err := s.repo.ListSessionWeeks(ctx, leagueID, leagueSessionID)
	return weeks, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// resolveSingleLeagueID stands in for league scoping while the MVP runs a single league; it
// must be replaced before a second league exists.
func (s *service) resolveSingleLeagueID(ctx context.Context) (string, error) {
	leagueID, err := s.repo.GetDefaultLeagueID(ctx)
	return leagueID, handler.MapFeatureError(err, leagueSessionErrorMappings)
}

// normaliseLeagueSessionPage bounds the page and decodes its cursor, then checks the
// cursor's values parse as the sort column's type. The cursor is client-held, so an edited
// one must stop at a 400 rather than reach Postgres as a failed cast and a 500.
func normaliseLeagueSessionPage(page *model.CursorParams, sort model.SortParams) error {
	if err := page.Normalise(sort); err != nil {
		return err
	}
	if page.After == nil {
		return nil
	}

	if err := common.ValidateUUID(page.After.LastID); err != nil {
		return model.InvalidCursorError()
	}

	var err error
	switch sort.Field {
	case LeagueSessionSortByStartsOn:
		_, err = time.Parse(time.DateOnly, page.After.LastValue)
	case LeagueSessionSortBySeasonYear:
		_, err = strconv.ParseInt(page.After.LastValue, 10, 16)
	}
	if err != nil {
		return model.InvalidCursorError()
	}
	return nil
}

// leagueSessionSortValue is the row's value in the sort column, as a cursor carries it.
// Keep in step with leagueSessionSortColumn.
func leagueSessionSortValue(session LeagueSession, field string) string {
	switch field {
	case LeagueSessionSortByName:
		return session.Name
	case LeagueSessionSortBySeasonYear:
		return strconv.Itoa(int(session.SeasonYear))
	default:
		return session.StartsOn.Format(time.DateOnly)
	}
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
		return handler.ErrorWithMessage(handler.CodeValidationFailed, fmt.Sprintf("name must be %d to %d characters",
			leagueSessionNameMinLength, leagueSessionNameMaxLength), nil)
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
		return handler.ErrorWithMessage(handler.CodeValidationFailed, "ends_on must be on or after starts_on", nil)
	}
	return nil
}

func validateLeagueSessionType(sessionType LeagueSessionType) error {
	if !slices.Contains(leagueSessionTypes, sessionType) {
		return handler.ErrorWithMessage(handler.CodeValidationFailed, "session_type must be one of SPRING, SUMMER, FALL", nil)
	}
	return nil
}

func validateLeagueSessionStatus(status LeagueSessionStatus) error {
	if !slices.Contains(leagueSessionStatuses, status) {
		return handler.ErrorWithMessage(handler.CodeValidationFailed, "status must be one of DRAFT, IN_PROGRESS, COMPLETE, CANCELLED", nil)
	}
	return nil
}

func validateSeasonYear(year int) error {
	if year < leagueSessionMinSeasonYear || year > leagueSessionMaxSeasonYear {
		return handler.ErrorWithMessage(handler.CodeValidationFailed, fmt.Sprintf("season_year must be between %d and %d",
			leagueSessionMinSeasonYear, leagueSessionMaxSeasonYear), nil)
	}
	return nil
}
