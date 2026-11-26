package sessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

const (
	testLeagueID        = "00000000-0000-4000-8000-000000000001"
	testLeagueSessionID = "00000000-0000-4000-8000-000000000002"
)

var testLogger = slog.New(slog.DiscardHandler)

// fakeRepository stores one session, Fall 2026, and records which writes reached it, so a
// test can assert a rule stopped the request before the database.
type fakeRepository struct {
	Repository
	defaultLeagueErr error
	writeErr         error
	created          *CreateLeagueSessionServiceParams
	updated          *UpdateLeagueSessionServiceParams
	softDeleted      bool
	permanentDeleted bool
	listed           []LeagueSession
	listedParams     *ListLeagueSessionsServiceParams
	counted          bool
	bulkCalledIDs    []string
	bulkFailures     map[string]error
}

func (f *fakeRepository) bulkDelete(ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	f.bulkCalledIDs = ids
	var deletedIDs []string
	for _, id := range ids {
		if f.bulkFailures[id] == nil {
			deletedIDs = append(deletedIDs, id)
		}
	}
	return newBulkDeleteLeagueSessionsResult(ids, deletedIDs, func(id string) error { return f.bulkFailures[id] }), nil
}

func (f *fakeRepository) BulkSoftDeleteSessions(_ context.Context, _ string, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	return f.bulkDelete(ids)
}

func (f *fakeRepository) BulkPermanentDeleteSessions(_ context.Context, _ string, ids []string) (*BulkDeleteLeagueSessionsResult, error) {
	return f.bulkDelete(ids)
}

func (f *fakeRepository) ListSessions(_ context.Context, _ string, params ListLeagueSessionsServiceParams) ([]LeagueSession, error) {
	f.listedParams = &params
	return f.listed, nil
}

func (f *fakeRepository) CountSessions(context.Context, string, ListLeagueSessionsServiceParams) (int64, error) {
	f.counted = true
	return int64(len(f.listed)), nil
}

func (f *fakeRepository) GetDefaultLeagueID(context.Context) (string, error) {
	if f.defaultLeagueErr != nil {
		return "", f.defaultLeagueErr
	}
	return testLeagueID, nil
}

func (f *fakeRepository) GetSessionByID(_ context.Context, leagueID, id string) (*LeagueSession, error) {
	if leagueID != testLeagueID || id != testLeagueSessionID {
		return nil, ErrLeagueSessionNotFound
	}
	return &LeagueSession{
		ID:          testLeagueSessionID,
		LeagueID:    testLeagueID,
		Name:        "Fall 2026",
		SessionType: LeagueSessionTypeFall,
		SeasonYear:  2026,
		StartsOn:    testDate("2026-09-01"),
		EndsOn:      testDate("2026-12-20"),
		Status:      LeagueSessionStatusInProgress,
	}, nil
}

func (f *fakeRepository) CreateSession(_ context.Context, _ string, params CreateLeagueSessionServiceParams) (*LeagueSession, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	f.created = &params
	return &LeagueSession{Name: params.Name, Status: params.Status}, nil
}

func (f *fakeRepository) UpdateSession(_ context.Context, _, id string, params UpdateLeagueSessionServiceParams) (*LeagueSession, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	f.updated = &params
	return &LeagueSession{ID: id}, nil
}

func (f *fakeRepository) SoftDeleteSession(context.Context, string, string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.softDeleted = true
	return nil
}

func (f *fakeRepository) PermanentDeleteSession(context.Context, string, string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.permanentDeleted = true
	return nil
}

func testDate(value string) time.Time {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return date
}

func validCreateParams() CreateLeagueSessionServiceParams {
	return CreateLeagueSessionServiceParams{
		Name:        "Spring 2027",
		SessionType: LeagueSessionTypeSpring,
		SeasonYear:  2027,
		StartsOn:    testDate("2027-01-10"),
		EndsOn:      testDate("2027-04-30"),
	}
}

func errorCode(err error) handler.ErrorCode {
	if err == nil {
		return 0
	}
	return handler.AsAPIError(err).Code
}

// The service is the only check on name length and season year, so a bad value it lets
// through is stored as is; and a valid one it rejects means an operator cannot create the
// session at all.
func TestCreateLeagueSessionValidation(t *testing.T) {
	tests := []struct {
		name     string
		change   func(*CreateLeagueSessionServiceParams)
		wantCode handler.ErrorCode
	}{
		{"name of 2 characters", func(p *CreateLeagueSessionServiceParams) { p.Name = "ab" }, handler.CodeValidationFailed},
		{"name of 3 characters", func(p *CreateLeagueSessionServiceParams) { p.Name = "abc" }, 0},
		{"name of 60 characters", func(p *CreateLeagueSessionServiceParams) { p.Name = strings.Repeat("a", 60) }, 0},
		{"name of 61 characters", func(p *CreateLeagueSessionServiceParams) { p.Name = strings.Repeat("a", 61) }, handler.CodeValidationFailed},
		{"60 multibyte characters", func(p *CreateLeagueSessionServiceParams) { p.Name = strings.Repeat("ư", 60) }, 0},
		{"whitespace-only name", func(p *CreateLeagueSessionServiceParams) { p.Name = "     " }, handler.CodeValidationFailed},
		{"name padded to 3 characters", func(p *CreateLeagueSessionServiceParams) { p.Name = "  ab  " }, handler.CodeValidationFailed},
		{"unknown session type", func(p *CreateLeagueSessionServiceParams) { p.SessionType = "WINTER" }, handler.CodeValidationFailed},
		{"lower-case session type", func(p *CreateLeagueSessionServiceParams) { p.SessionType = "spring" }, handler.CodeValidationFailed},
		{"unknown status", func(p *CreateLeagueSessionServiceParams) { p.Status = "ARCHIVED" }, handler.CodeValidationFailed},
		{"year 1999", func(p *CreateLeagueSessionServiceParams) { p.SeasonYear = 1999 }, handler.CodeValidationFailed},
		{"year 2000", func(p *CreateLeagueSessionServiceParams) { p.SeasonYear = 2000 }, 0},
		{"year 2100", func(p *CreateLeagueSessionServiceParams) { p.SeasonYear = 2100 }, 0},
		{"year 2101", func(p *CreateLeagueSessionServiceParams) { p.SeasonYear = 2101 }, handler.CodeValidationFailed},
		{"ends before it starts", func(p *CreateLeagueSessionServiceParams) { p.EndsOn = p.StartsOn.AddDate(0, 0, -1) }, handler.CodeValidationFailed},
		{"one-day session", func(p *CreateLeagueSessionServiceParams) { p.EndsOn = p.StartsOn }, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			params := validCreateParams()
			tt.change(&params)

			_, err := NewService(repo, testLogger).CreateSession(context.Background(), params)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("code = %d, want %d (err: %v)", got, tt.wantCode, err)
			}
			if reached := repo.created != nil; reached != (tt.wantCode == 0) {
				t.Errorf("repository reached = %v, want %v", reached, tt.wantCode == 0)
			}
		})
	}
}

// The schema default is DRAFT too, but the service must not rely on it: a session created
// as IN_PROGRESS by accident shows up as live league play before it is set up.
func TestCreateLeagueSessionDefaultsStatusAndTrimsName(t *testing.T) {
	repo := &fakeRepository{}
	params := validCreateParams()
	params.Name = "  Spring 2027  "

	if _, err := NewService(repo, testLogger).CreateSession(context.Background(), params); err != nil {
		t.Fatal(err)
	}

	if repo.created.Status != LeagueSessionStatusDraft {
		t.Errorf("status = %q, want DRAFT", repo.created.Status)
	}
	if repo.created.Name != "Spring 2027" {
		t.Errorf("name = %q, want it trimmed", repo.created.Name)
	}
}

// Validating only the supplied fields would let a PATCH of ends_on alone move it before the
// stored starts_on: the database rejects it, but as a generic message rather than a named rule.
func TestUpdateLeagueSessionValidatesMergedResult(t *testing.T) {
	earlyEnd := testDate("2026-08-31")
	lateStart := testDate("2026-12-21")
	sameDayEnd := testDate("2026-09-01")
	shortName := " ab "
	newName := "Fall 2026 League"

	tests := []struct {
		name     string
		id       string
		params   UpdateLeagueSessionServiceParams
		wantCode handler.ErrorCode
	}{
		{"empty patch", testLeagueSessionID, UpdateLeagueSessionServiceParams{}, handler.CodeValidationFailed},
		{"ends_on before stored starts_on", testLeagueSessionID, UpdateLeagueSessionServiceParams{EndsOn: &earlyEnd}, handler.CodeValidationFailed},
		{"starts_on after stored ends_on", testLeagueSessionID, UpdateLeagueSessionServiceParams{StartsOn: &lateStart}, handler.CodeValidationFailed},
		{"ends_on on stored starts_on", testLeagueSessionID, UpdateLeagueSessionServiceParams{EndsOn: &sameDayEnd}, 0},
		{"name too short after trim", testLeagueSessionID, UpdateLeagueSessionServiceParams{Name: &shortName}, handler.CodeValidationFailed},
		{"rename", testLeagueSessionID, UpdateLeagueSessionServiceParams{Name: &newName}, 0},
		{"missing session", "00000000-0000-4000-8000-0000000000ff", UpdateLeagueSessionServiceParams{Name: &newName}, handler.CodeNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}

			_, err := NewService(repo, testLogger).UpdateSessionByID(context.Background(), tt.id, tt.params)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("code = %d, want %d (err: %v)", got, tt.wantCode, err)
			}
			if reached := repo.updated != nil; reached != (tt.wantCode == 0) {
				t.Errorf("repository update reached = %v, want %v", reached, tt.wantCode == 0)
			}
		})
	}
}

// List filters are validated before the query: an unknown status would otherwise match
// nothing and look like an empty league, rather than telling the client its filter is wrong.
func TestListLeagueSessionsRejectsBadFilters(t *testing.T) {
	tests := []struct {
		name   string
		params ListLeagueSessionsServiceParams
	}{
		{"unknown session type", ListLeagueSessionsServiceParams{SessionType: "WINTER"}},
		{"unknown status", ListLeagueSessionsServiceParams{Status: "done"}},
		{"year out of range", ListLeagueSessionsServiceParams{SeasonYear: 1999}},
		{"query too long", ListLeagueSessionsServiceParams{Query: strings.Repeat("a", maxSearchQueryLength+1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(&fakeRepository{}, testLogger).ListSessions(context.Background(), tt.params)
			if got := errorCode(err); got != handler.CodeValidationFailed {
				t.Errorf("code = %d, want %d (err: %v)", got, handler.CodeValidationFailed, err)
			}
		})
	}
}

// A sentinel mapped to the wrong code reaches the client as the wrong status: a duplicate
// name served as 500 reads as an outage, and a missing league served as 404 hides one.
func TestLeagueSessionErrorMapping(t *testing.T) {
	tests := []struct {
		sentinel   error
		wantCode   handler.ErrorCode
		wantStatus int
	}{
		{ErrLeagueSessionNotFound, handler.CodeNotFound, 404},
		{ErrLeagueSessionNotSoftDeleted, handler.CodeConflict, 409},
		{ErrDuplicateLeagueSessionName, handler.CodeConflict, 409},
		{ErrInvalidLeagueSession, handler.CodeValidationFailed, 400},
		{ErrLeagueSessionActiveConflict, handler.CodeConflict, 409},
		{ErrLeagueSessionHasSharedTeams, handler.CodeConflict, 409},
		{ErrNoDefaultLeague, handler.CodeInternal, 500},
	}

	for _, tt := range tests {
		t.Run(tt.sentinel.Error(), func(t *testing.T) {
			repo := &fakeRepository{writeErr: fmt.Errorf("wrapped: %w", tt.sentinel)}

			err := NewService(repo, testLogger).PermanentDeleteSession(context.Background(), testLeagueSessionID)

			apiErr := handler.AsAPIError(err)
			if apiErr.Code != tt.wantCode || apiErr.Status() != tt.wantStatus {
				t.Errorf("served as %d/%d, want %d/%d", apiErr.Code, apiErr.Status(), tt.wantCode, tt.wantStatus)
			}
		})
	}
}

// Without a single active league every route must fail loudly as a 500, not silently act
// on no league or on the wrong one.
func TestNoDefaultLeagueFailsEveryCall(t *testing.T) {
	repo := &fakeRepository{defaultLeagueErr: fmt.Errorf("%w: found 2", ErrNoDefaultLeague)}
	service := NewService(repo, testLogger)

	_, err := service.CreateSession(context.Background(), validCreateParams())

	if got := errorCode(err); got != handler.CodeInternal {
		t.Errorf("code = %d, want %d", got, handler.CodeInternal)
	}
	if repo.created != nil {
		t.Error("create reached the repository without a league")
	}
}

// The repository tells constraints apart by name, because one SQLSTATE covers several:
// 23505 is both a duplicate name (409) and a concurrent activation (also 409, but a
// different message), and an unmapped constraint must stay a 500 rather than be guessed.
func TestConstraintErrorMapsByName(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantTarget error
	}{
		{"duplicate name", &pgconn.PgError{Code: "23505", ConstraintName: "league_sessions_league_id_name_key"}, ErrDuplicateLeagueSessionName},
		{"second active session", &pgconn.PgError{Code: "23505", ConstraintName: "league_sessions_one_active_per_league"}, ErrLeagueSessionActiveConflict},
		{"date order check", &pgconn.PgError{Code: "23514", ConstraintName: "league_sessions_check"}, ErrInvalidLeagueSession},
		{"wrapped", fmt.Errorf("tx: %w", &pgconn.PgError{Code: "23514", ConstraintName: "league_sessions_status_check"}), ErrInvalidLeagueSession},
		{"fixture still references a cascaded team", &pgconn.PgError{Code: "23503", ConstraintName: "team_matches_home_division_team_fkey"}, ErrLeagueSessionHasSharedTeams},
		{"unmapped constraint", &pgconn.PgError{Code: "23505", ConstraintName: "something_else_key"}, nil},
		{"not a postgres error", errors.New("connection reset"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := constraintError(tt.err); !errors.Is(got, tt.wantTarget) || (tt.wantTarget == nil && got != nil) {
				t.Errorf("constraintError = %v, want %v", got, tt.wantTarget)
			}
		})
	}
}

// The two deletes must reach different repository calls: DELETE /sessions/{id} wired to the
// permanent delete would cascade a session's divisions, teams and results on the first click.
func TestDeleteLeagueSessionRoutesToTheRightDelete(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, testLogger)

	if err := service.DeleteSessionByID(context.Background(), testLeagueSessionID); err != nil {
		t.Fatal(err)
	}
	if !repo.softDeleted || repo.permanentDeleted {
		t.Fatalf("soft delete: soft = %v, permanent = %v", repo.softDeleted, repo.permanentDeleted)
	}

	repo = &fakeRepository{}
	if err := NewService(repo, testLogger).PermanentDeleteSession(context.Background(), testLeagueSessionID); err != nil {
		t.Fatal(err)
	}
	if repo.softDeleted || !repo.permanentDeleted {
		t.Errorf("permanent delete: soft = %v, permanent = %v", repo.softDeleted, repo.permanentDeleted)
	}
}

func testLeagueSessions(count int) []LeagueSession {
	sessions := make([]LeagueSession, 0, count)
	for i := range count {
		sessions = append(sessions, LeagueSession{
			ID:         fmt.Sprintf("00000000-0000-4000-8000-%012d", i+10),
			Name:       fmt.Sprintf("Session %02d", i),
			SeasonYear: int16(2020 + i),
			StartsOn:   testDate("2026-01-01").AddDate(0, 0, -i),
		})
	}
	return sessions
}

// The repository returns limit+1 rows as a lookahead. The page must drop it and point the
// cursor at the last row kept, or the client skips a session between pages.
func TestListLeagueSessionsPages(t *testing.T) {
	sort := defaultLeagueSessionSort
	repo := &fakeRepository{listed: testLeagueSessions(3)}

	page, err := NewService(repo, testLogger).ListSessions(context.Background(), ListLeagueSessionsServiceParams{
		Sort: sort,
		Page: model.CursorParams{Limit: 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("items = %d, next = %v; want 2 items and a next cursor", len(page.Items), page.NextCursor)
	}
	if page.Total == nil || !repo.counted {
		t.Error("first page has no total")
	}
	next, err := model.DecodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if next.LastID != page.Items[1].ID || next.LastValue != "2025-12-31" || next.Sort != "-starts_on" {
		t.Errorf("next cursor = %+v, want after %s at 2025-12-31 under -starts_on", next, page.Items[1].ID)
	}

	repo = &fakeRepository{listed: testLeagueSessions(1)}
	page, err = NewService(repo, testLogger).ListSessions(context.Background(), ListLeagueSessionsServiceParams{
		Sort: sort,
		Page: model.CursorParams{Limit: 2, Cursor: model.EncodeCursor(*next)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.counted || page.Total != nil {
		t.Error("a later page counted again: total is first-page only")
	}
	if page.NextCursor != nil {
		t.Errorf("last page has a next cursor %q", *page.NextCursor)
	}
}

// The cursor is client-held. An edited value must be a 400 here, not a failed ::date or
// ::uuid cast in Postgres served as a 500.
func TestListLeagueSessionsRejectsBadCursor(t *testing.T) {
	validID := "00000000-0000-4000-8000-000000000010"
	byStartsOn := defaultLeagueSessionSort
	byYear := model.SortParams{Field: LeagueSessionSortBySeasonYear}
	byName := model.SortParams{Field: LeagueSessionSortByName}

	tests := []struct {
		name     string
		sort     model.SortParams
		cursor   model.Cursor
		wantCode handler.ErrorCode
	}{
		{"valid date", byStartsOn, model.Cursor{Sort: byStartsOn.String(), LastValue: "2026-09-01", LastID: validID}, 0},
		{"timestamp for a date column", byStartsOn, model.Cursor{Sort: byStartsOn.String(), LastValue: "2026-09-01T00:00:00Z", LastID: validID}, handler.CodeValidationFailed},
		{"valid year", byYear, model.Cursor{Sort: byYear.String(), LastValue: "2026", LastID: validID}, 0},
		{"year past smallint", byYear, model.Cursor{Sort: byYear.String(), LastValue: "70000", LastID: validID}, handler.CodeValidationFailed},
		{"any name", byName, model.Cursor{Sort: byName.String(), LastValue: "x' OR 1=1 --", LastID: validID}, 0},
		{"id not a uuid", byName, model.Cursor{Sort: byName.String(), LastValue: "Fall", LastID: "42"}, handler.CodeValidationFailed},
		{"cursor from another sort", byName, model.Cursor{Sort: byStartsOn.String(), LastValue: "2026-09-01", LastID: validID}, handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, err := NewService(repo, testLogger).ListSessions(context.Background(), ListLeagueSessionsServiceParams{
				Sort: tt.sort,
				Page: model.CursorParams{Cursor: model.EncodeCursor(tt.cursor)},
			})

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("code = %d, want %d (err: %v)", got, tt.wantCode, err)
			}
			if reached := repo.listedParams != nil; reached != (tt.wantCode == 0) {
				t.Errorf("repository reached = %v, want %v", reached, tt.wantCode == 0)
			}
		})
	}
}

// A malformed request is rejected whole, before the database: an id that is not a uuid
// would otherwise fail the ::uuid[] cast and 500 the batch, and an unbounded list lets one
// request rewrite every session.
func TestBulkDeleteLeagueSessionsRequestIsValidated(t *testing.T) {
	uniqueIDs := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		}
		return ids
	}

	tests := []struct {
		name        string
		ids         []string
		wantCode    handler.ErrorCode
		wantReached int
	}{
		{"no ids", nil, handler.CodeValidationFailed, 0},
		{"one not a uuid", []string{testLeagueSessionID, "abc"}, handler.CodeValidationFailed, 0},
		{"over the limit", uniqueIDs(maxBulkLeagueSessionIDs + 1), handler.CodeValidationFailed, 0},
		{"at the limit", uniqueIDs(maxBulkLeagueSessionIDs), 0, maxBulkLeagueSessionIDs},
		{"repeats count once", append(uniqueIDs(maxBulkLeagueSessionIDs), uniqueIDs(maxBulkLeagueSessionIDs)...), 0, maxBulkLeagueSessionIDs},
		{"a repeat in upper case counts once", []string{testLeagueSessionID, strings.ToUpper(testLeagueSessionID)}, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, err := NewService(repo, testLogger).BulkSoftDeleteSessions(context.Background(), tt.ids)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %d, want %d (err: %v)", got, tt.wantCode, err)
			}
			if len(repo.bulkCalledIDs) != tt.wantReached {
				t.Errorf("sessions reaching the repository = %d, want %d", len(repo.bulkCalledIDs), tt.wantReached)
			}
		})
	}

	t.Run("ids reach the repository canonical", func(t *testing.T) {
		repo := &fakeRepository{}
		_, err := NewService(repo, testLogger).
			BulkPermanentDeleteSessions(context.Background(), []string{strings.ToUpper(testLeagueSessionID), "{" + testLeagueID + "}"})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.Join(repo.bulkCalledIDs, ","), testLeagueSessionID+","+testLeagueID; got != want {
			t.Errorf("ids passed = %s, want %s", got, want)
		}
	})
}

// Best effort: a session that cannot be deleted is reported with the code the single
// delete returns, and the others are still deleted. A failure of the statement itself
// fails the whole request, as a 500 without its cause.
func TestBulkDeleteLeagueSessionsIsBestEffort(t *testing.T) {
	const (
		missingID = "55555555-5555-4555-8555-555555555555"
		liveID    = "66666666-6666-4666-8666-666666666666"
		sharedID  = "77777777-7777-4777-8777-777777777777"
	)

	repo := &fakeRepository{bulkFailures: map[string]error{
		missingID: ErrLeagueSessionNotFound,
		liveID:    ErrLeagueSessionNotSoftDeleted,
		sharedID:  ErrLeagueSessionHasSharedTeams,
	}}
	result, err := NewService(repo, testLogger).
		BulkPermanentDeleteSessions(context.Background(), []string{missingID, testLeagueSessionID, liveID, sharedID})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.DeletedIDs) != 1 || result.DeletedIDs[0] != testLeagueSessionID {
		t.Errorf("deleted = %v, want only %s", result.DeletedIDs, testLeagueSessionID)
	}
	wantCodes := map[string]handler.ErrorCode{
		missingID: handler.CodeNotFound,
		liveID:    handler.CodeConflict,
		sharedID:  handler.CodeConflict,
	}
	response := newBulkDeleteLeagueSessionsResponse(*result)
	if len(response.Failed) != len(wantCodes) {
		t.Fatalf("failed = %+v, want %d entries", response.Failed, len(wantCodes))
	}
	for _, failure := range response.Failed {
		if failure.Code != wantCodes[failure.ID] || failure.Message == "" {
			t.Errorf("%s served as %d %q, want %d with a message", failure.ID, failure.Code, failure.Message, wantCodes[failure.ID])
		}
	}

	t.Run("statement failure fails the request", func(t *testing.T) {
		repo := &fakeRepository{writeErr: errors.New("connection reset")}
		_, err := NewService(repo, testLogger).BulkSoftDeleteSessions(context.Background(), []string{testLeagueSessionID})
		if got := errorCode(err); got != handler.CodeInternal {
			t.Errorf("code = %d, want %d", got, handler.CodeInternal)
		}
	})
}

// Every requested id appears exactly once, in request order, so the admin page can match
// each row it sent to an outcome.
func TestNewBulkDeleteLeagueSessionsResult(t *testing.T) {
	result := newBulkDeleteLeagueSessionsResult([]string{"a", "b", "c"}, []string{"c", "a"},
		func(string) error { return ErrLeagueSessionNotFound })

	if got := strings.Join(result.DeletedIDs, ","); got != "a,c" {
		t.Errorf("deleted = %s, want a,c", got)
	}
	if len(result.Failures) != 1 || result.Failures[0].LeagueSessionID != "b" {
		t.Errorf("failures = %+v, want only b", result.Failures)
	}
}

// The single permanent delete and the bulk one decide from the same state, so a session
// one refuses the other refuses with the same error.
func TestLeagueSessionDeleteStateFailure(t *testing.T) {
	tests := []struct {
		name  string
		state leagueSessionDeleteState
		want  error
	}{
		{"missing", leagueSessionDeleteState{}, ErrLeagueSessionNotFound},
		{"live", leagueSessionDeleteState{SessionExists: true}, ErrLeagueSessionNotSoftDeleted},
		{"live with shared teams", leagueSessionDeleteState{SessionExists: true, HasSharedTeams: true}, ErrLeagueSessionNotSoftDeleted},
		{"deleted with shared teams", leagueSessionDeleteState{SessionExists: true, IsDeleted: true, HasSharedTeams: true}, ErrLeagueSessionHasSharedTeams},
		{"deletable", leagueSessionDeleteState{SessionExists: true, IsDeleted: true}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.deleteFailure(); !errors.Is(got, tt.want) || (tt.want == nil && got != nil) {
				t.Errorf("deleteFailure = %v, want %v", got, tt.want)
			}
		})
	}
}
