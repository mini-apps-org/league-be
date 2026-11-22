package teams

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

const (
	testDivisionID = "00000000-0000-4000-8000-000000000000"
	testTeamID     = "11111111-1111-4111-8111-111111111111"
	testOtherTeam  = "22222222-2222-4222-8222-222222222222"
	testCaptainID  = "33333333-3333-4333-8333-333333333333"
	testPlayerID   = "44444444-4444-4444-8444-444444444444"
	testUnpaidID   = "66666666-6666-4666-8666-666666666666"
	testOtherCapID = "77777777-7777-4777-8777-777777777777"

	testAdminUser        = "user-admin"
	testCaptainUser      = "user-captain"
	testOtherCaptainUser = "user-other-captain"
	testNoPlayerUser     = "user-without-player"
)

// testLogger discards: the tests assert on results, not on log lines.
var testLogger = slog.New(slog.DiscardHandler)

// fakePermissions grants team.update to testAdminUser only, as the seed does to SUPER_ADMIN.
type fakePermissions struct{}

func (fakePermissions) GetPermissionCodesByUserID(_ context.Context, userID string) ([]string, error) {
	if userID == testAdminUser {
		return []string{"roster.create", "team.update"}, nil
	}
	return []string{"roster.create", "roster.update", "roster.delete"}, nil
}

// fakeRepository holds testTeamID captained by testCaptainID and testOtherTeam captained
// by testOtherCapID, and records which writes reached it, so a test can assert a rule
// stopped the request before the database.
type fakeRepository struct {
	Repository
	activePlayerIDs map[string]bool
	created         *CreateTeamServiceParams
	listedTeams     *ListTeamsServiceParams
	teamRows        []Team
	countCalls      int
	listedRoster    *ListRosterPlayersServiceParams
	added           bool
	removed         bool
	transferred     bool
	updated         bool
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{activePlayerIDs: map[string]bool{testCaptainID: true, testPlayerID: true}}
}

func (f *fakeRepository) GetTeamByID(_ context.Context, id string) (*Team, error) {
	if id != testTeamID && id != testOtherTeam {
		return nil, ErrTeamNotFound
	}
	captainID := testCaptainID
	if id == testOtherTeam {
		captainID = testOtherCapID
	}
	return &Team{ID: id, CaptainPlayerID: &captainID}, nil
}

func (f *fakeRepository) GetPlayerIDByUserID(_ context.Context, userID string) (string, error) {
	switch userID {
	case testCaptainUser:
		return testCaptainID, nil
	case testOtherCaptainUser:
		return testOtherCapID, nil
	default:
		return "", ErrPlayerNotFound
	}
}

func (f *fakeRepository) IsPaidPlayer(_ context.Context, playerID string) (bool, error) {
	return playerID != testUnpaidID, nil
}

func (f *fakeRepository) AddRosterPlayer(context.Context, RosterEntryServiceParams) error {
	f.added = true
	return nil
}

func (f *fakeRepository) GetRosterPlayer(_ context.Context, params RosterEntryServiceParams) (*RosterPlayer, error) {
	return &RosterPlayer{TeamID: params.TeamID, PlayerID: params.PlayerID}, nil
}

func (f *fakeRepository) CreateTeam(_ context.Context, params CreateTeamServiceParams) (*Team, error) {
	f.created = &params
	return &Team{TeamCode: params.TeamCode}, nil
}

func (f *fakeRepository) IsActiveTeamPlayer(_ context.Context, _, playerID string) (bool, error) {
	return f.activePlayerIDs[playerID], nil
}

func (f *fakeRepository) UpdateTeam(_ context.Context, id string, _ UpdateTeamServiceParams) (*Team, error) {
	f.updated = true
	return &Team{ID: id}, nil
}

func (f *fakeRepository) RemoveRosterPlayer(context.Context, RosterEntryServiceParams) error {
	f.removed = true
	return nil
}

func (f *fakeRepository) TransferRosterPlayer(context.Context, TransferRosterPlayerServiceParams) error {
	f.transferred = true
	return nil
}

func (f *fakeRepository) ListTeams(_ context.Context, params ListTeamsServiceParams) ([]Team, error) {
	f.listedTeams = &params
	return f.teamRows, nil
}

func (f *fakeRepository) CountTeams(context.Context, ListTeamsServiceParams) (int64, error) {
	f.countCalls++
	return 42, nil
}

func (f *fakeRepository) CountRosterPlayers(context.Context, ListRosterPlayersServiceParams) (int64, error) {
	f.countCalls++
	return 7, nil
}

func (f *fakeRepository) GetDivisionTeam(_ context.Context, divisionID, teamID string) (*DivisionTeam, error) {
	return &DivisionTeam{DivisionID: divisionID, TeamID: teamID}, nil
}

func (f *fakeRepository) ListRosterPlayers(_ context.Context, params ListRosterPlayersServiceParams) ([]RosterPlayer, error) {
	f.listedRoster = &params
	return []RosterPlayer{}, nil
}

func errorCode(err error) handler.ErrorCode {
	var apiErr *handler.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

// The team code is typed by an operator and printed on scoresheets. A code outside
// XXyyA would otherwise surface as a CHECK violation, served as a 500.
func TestCreateTeamValidatesTeamCode(t *testing.T) {
	tests := []struct {
		teamCode   string
		wantStored string
		wantCode   handler.ErrorCode
	}{
		{"41503", "41503", ""},
		{" 41a0b ", "41A0B", ""},
		{"4150", "", handler.CodeValidationFailed},
		{"415034", "", handler.CodeValidationFailed},
		{"41-50", "", handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.teamCode, func(t *testing.T) {
			repo := newFakeRepository()
			_, err := NewService(repo, fakePermissions{}, testLogger).CreateTeam(context.Background(), CreateTeamServiceParams{
				DivisionID: testDivisionID,
				TeamCode:   tt.teamCode,
				Name:       "Cue Masters",
			})

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			if tt.wantCode == "" && repo.created.TeamCode != tt.wantStored {
				t.Errorf("stored team_code = %q, want %q", repo.created.TeamCode, tt.wantStored)
			}
		})
	}
}

// Removing or transferring the captain would leave teams.captain_player_id pointing
// at a player who is no longer on the team, and the Start Match screen would offer a
// captain who cannot put anyone up.
func TestCaptainCannotLeaveRoster(t *testing.T) {
	captainEntry := RosterEntryServiceParams{DivisionID: testDivisionID, TeamID: testTeamID, PlayerID: testCaptainID}

	tests := []struct {
		name string
		run  func(Service) error
	}{
		{"remove", func(s Service) error {
			return s.RemoveRosterPlayer(context.Background(), testAdminUser, captainEntry)
		}},
		{"transfer", func(s Service) error {
			_, err := s.TransferRosterPlayer(context.Background(), testAdminUser, TransferRosterPlayerServiceParams{
				From: captainEntry, ToTeamID: testOtherTeam,
			})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			err := tt.run(NewService(repo, fakePermissions{}, testLogger))

			if got := errorCode(err); got != handler.CodeConflict {
				t.Fatalf("error code = %q, want %q (err: %v)", got, handler.CodeConflict, err)
			}
			if repo.removed || repo.transferred {
				t.Error("the roster write reached the repository")
			}
		})
	}
}

// A captain off the roster could not be put up in a match; the service is the only
// place this is checked, since the schema's FK accepts any player.
func TestUpdateTeamCaptainMustBeOnRoster(t *testing.T) {
	tests := []struct {
		name     string
		playerID string
		wantCode handler.ErrorCode
		wantSave bool
	}{
		{"on roster", testPlayerID, "", true},
		{"not on roster", "55555555-5555-4555-8555-555555555555", handler.CodeValidationFailed, false},
		{"not a uuid", "abc", handler.CodeValidationFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			_, err := NewService(repo, fakePermissions{}, testLogger).UpdateTeam(context.Background(), testTeamID, UpdateTeamServiceParams{
				CaptainPlayerID: &tt.playerID,
			})

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			if repo.updated != tt.wantSave {
				t.Errorf("updated = %v, want %v", repo.updated, tt.wantSave)
			}
		})
	}
}

// Transferring to the same team and division would close the roster row and reopen an
// identical one, silently resetting joined_at. An absent target division means the
// current one, so omitting it must not slip past this check.
func TestTransferToSameRosterRejected(t *testing.T) {
	repo := newFakeRepository()
	_, err := NewService(repo, fakePermissions{}, testLogger).TransferRosterPlayer(context.Background(), testAdminUser, TransferRosterPlayerServiceParams{
		From:     RosterEntryServiceParams{DivisionID: testDivisionID, TeamID: testTeamID, PlayerID: testPlayerID},
		ToTeamID: testTeamID,
	})

	if got := errorCode(err); got != handler.CodeValidationFailed {
		t.Fatalf("error code = %q, want %q (err: %v)", got, handler.CodeValidationFailed, err)
	}
	if repo.transferred {
		t.Error("the transfer reached the repository")
	}
}

// roster.create/update/delete are granted to every captain, so without this scope check
// the captain of one team could strip or poach another team's players.
func TestRosterWritesAreCaptainScoped(t *testing.T) {
	ownPlayer := RosterEntryServiceParams{DivisionID: testDivisionID, TeamID: testTeamID, PlayerID: testPlayerID}
	otherTeamPlayer := RosterEntryServiceParams{DivisionID: testDivisionID, TeamID: testOtherTeam, PlayerID: testPlayerID}
	unpaidPlayer := RosterEntryServiceParams{DivisionID: testDivisionID, TeamID: testTeamID, PlayerID: testUnpaidID}

	add := func(entry RosterEntryServiceParams) func(Service, string) error {
		return func(s Service, user string) error {
			_, err := s.AddRosterPlayer(context.Background(), user, entry)
			return err
		}
	}
	remove := func(entry RosterEntryServiceParams) func(Service, string) error {
		return func(s Service, user string) error {
			return s.RemoveRosterPlayer(context.Background(), user, entry)
		}
	}
	transfer := func(from RosterEntryServiceParams, toTeamID string) func(Service, string) error {
		return func(s Service, user string) error {
			_, err := s.TransferRosterPlayer(context.Background(), user, TransferRosterPlayerServiceParams{From: from, ToTeamID: toTeamID})
			return err
		}
	}

	tests := []struct {
		name     string
		user     string
		run      func(Service, string) error
		wantCode handler.ErrorCode
	}{
		{"captain adds to own team", testCaptainUser, add(ownPlayer), ""},
		{"captain adds to another team", testCaptainUser, add(otherTeamPlayer), handler.CodePermissionDenied},
		{"captain adds unpaid member", testCaptainUser, add(unpaidPlayer), handler.CodePermissionDenied},
		{"admin adds unpaid member", testAdminUser, add(unpaidPlayer), ""},
		{"captain removes from own team", testCaptainUser, remove(ownPlayer), ""},
		{"captain removes from another team", testCaptainUser, remove(otherTeamPlayer), handler.CodePermissionDenied},
		{"user with no player removes", testNoPlayerUser, remove(ownPlayer), handler.CodePermissionDenied},
		{"captain poaches onto own team", testCaptainUser, transfer(otherTeamPlayer, testTeamID), handler.CodePermissionDenied},
		{"captain pushes onto another team", testCaptainUser, transfer(ownPlayer, testOtherTeam), handler.CodePermissionDenied},
		{"admin transfers", testAdminUser, transfer(ownPlayer, testOtherTeam), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			err := tt.run(NewService(repo, fakePermissions{}, testLogger), tt.user)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			wrote := repo.added || repo.removed || repo.transferred
			if wrote != (tt.wantCode == "") {
				t.Errorf("write reached repository = %v, want %v", wrote, tt.wantCode == "")
			}
		})
	}
}

// A wrapped sentinel must still map to its status: the repository wraps errors from
// inside a transaction, and a lost mapping would serve 500 for a plain 409.
func TestAsAPIErrorUnwrapsSentinels(t *testing.T) {
	tests := []struct {
		sentinel error
		want     handler.ErrorCode
	}{
		{ErrTeamNotFound, handler.CodeNotFound},
		{ErrPlayerNotFound, handler.CodeNotFound},
		{ErrTeamNotInDivision, handler.CodeNotFound},
		{ErrRosterPlayerNotFound, handler.CodeNotFound},
		{ErrDivisionNotFound, handler.CodeValidationFailed},
		{ErrVenueNotFound, handler.CodeValidationFailed},
		{ErrDuplicateTeamCode, handler.CodeConflict},
		{ErrTeamAlreadyInDivision, handler.CodeConflict},
		{ErrTeamHasMatches, handler.CodeConflict},
		{ErrTeamNotSoftDeleted, handler.CodeConflict},
		{ErrPlayerAlreadyInDivision, handler.CodeConflict},
	}

	for _, tt := range tests {
		t.Run(tt.sentinel.Error(), func(t *testing.T) {
			wrapped := errors.Join(errors.New("teams: transfer player"), tt.sentinel)
			if got := errorCode(asAPIError(wrapped)); got != tt.want {
				t.Errorf("code = %q, want %q", got, tt.want)
			}
		})
	}
}

// List filters reach SQL as uuid casts: a malformed id must stop at a 400 rather than a
// Postgres cast error served as 500, and a search term is trimmed so " cue " finds "Cue".
func TestListFiltersAreValidated(t *testing.T) {
	longQuery := strings.Repeat("a", maxSearchQueryLength+1)

	tests := []struct {
		name      string
		params    ListTeamsServiceParams
		wantCode  handler.ErrorCode
		wantQuery string
	}{
		{"no filters", ListTeamsServiceParams{}, "", ""},
		{"trimmed search", ListTeamsServiceParams{Query: "  cue  "}, "", "cue"},
		{"blank search", ListTeamsServiceParams{Query: "   "}, "", ""},
		{"division not a uuid", ListTeamsServiceParams{DivisionID: "abc"}, handler.CodeValidationFailed, ""},
		{"venue not a uuid", ListTeamsServiceParams{VenueID: "abc"}, handler.CodeValidationFailed, ""},
		{"search too long", ListTeamsServiceParams{Query: longQuery}, handler.CodeValidationFailed, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			_, err := NewService(repo, fakePermissions{}, testLogger).ListTeams(context.Background(), tt.params)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			if tt.wantCode != "" {
				if repo.listedTeams != nil {
					t.Error("an invalid filter reached the repository")
				}
				return
			}
			if repo.listedTeams.Query != tt.wantQuery {
				t.Errorf("query = %q, want %q", repo.listedTeams.Query, tt.wantQuery)
			}
		})
	}

	t.Run("roster search too long", func(t *testing.T) {
		repo := newFakeRepository()
		_, err := NewService(repo, fakePermissions{}, testLogger).ListRosterPlayers(context.Background(), ListRosterPlayersServiceParams{
			DivisionID: testDivisionID, TeamID: testTeamID, Query: longQuery,
		})
		if got := errorCode(err); got != handler.CodeValidationFailed {
			t.Fatalf("error code = %q, want %q (err: %v)", got, handler.CodeValidationFailed, err)
		}
		if repo.listedRoster != nil {
			t.Error("an invalid search reached the repository")
		}
	})
}

// The cursor is client-held. An edited value must stop at a 400: in SQL it becomes
// $n::timestamptz or $n::integer, and a failed cast would be served as a 500.
func TestListCursorIsValidated(t *testing.T) {
	byCreatedAt := model.SortParams{Field: TeamSortByCreatedAt}
	cursor := func(value, id string) string {
		return model.EncodeCursor(model.Cursor{Sort: byCreatedAt.String(), LastValue: value, LastID: id})
	}

	tests := []struct {
		name     string
		cursor   string
		wantCode handler.ErrorCode
	}{
		{"valid", cursor("2026-11-22T07:00:00.123456Z", testTeamID), ""},
		{"value not a time", cursor("yesterday", testTeamID), handler.CodeValidationFailed},
		{"id not a uuid", cursor("2026-11-22T07:00:00Z", "abc"), handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			_, err := NewService(repo, fakePermissions{}, testLogger).ListTeams(context.Background(), ListTeamsServiceParams{
				Sort: byCreatedAt,
				Page: model.CursorParams{Cursor: tt.cursor},
			})

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			if (repo.listedTeams != nil) != (tt.wantCode == "") {
				t.Errorf("reached repository = %v, want %v", repo.listedTeams != nil, tt.wantCode == "")
			}
		})
	}
}

// The next cursor must carry the last served row's value in the sort column; one built
// from another column would start the next page in the wrong place.
func TestListTeamsNextCursor(t *testing.T) {
	repo := newFakeRepository()
	for i := range model.DefaultLimit + 1 {
		repo.teamRows = append(repo.teamRows, Team{ID: strings.Repeat(string(rune('a'+i)), 3), Name: string(rune('A' + i))})
	}

	byNameDesc := model.SortParams{Field: TeamSortByName, Desc: true}
	page, err := NewService(repo, fakePermissions{}, testLogger).ListTeams(context.Background(), ListTeamsServiceParams{Sort: byNameDesc})
	if err != nil {
		t.Fatal(err)
	}
	if repo.listedTeams.Page.Limit != model.DefaultLimit {
		t.Errorf("repository asked for limit %d, want %d", repo.listedTeams.Page.Limit, model.DefaultLimit)
	}
	if len(page.Items) != model.DefaultLimit || page.NextCursor == nil {
		t.Fatalf("got %d items, next %v; want %d and a cursor", len(page.Items), page.NextCursor, model.DefaultLimit)
	}

	next, err := model.DecodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	last := page.Items[len(page.Items)-1]
	want := model.Cursor{Sort: "-name", LastValue: last.Name, LastID: last.ID}
	if *next != want {
		t.Errorf("next cursor = %+v, want %+v", *next, want)
	}
}

// A client's limit must reach the query as limit+1 and size the page; dropping it would
// silently serve the default 10.
func TestListTeamsHonoursLimit(t *testing.T) {
	repo := newFakeRepository()
	for i := range 6 {
		repo.teamRows = append(repo.teamRows, Team{ID: string(rune('a' + i)), TeamCode: string(rune('A' + i))})
	}

	page, err := NewService(repo, fakePermissions{}, testLogger).ListTeams(context.Background(), ListTeamsServiceParams{
		Page: model.CursorParams{Limit: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.listedTeams.Page.Limit != 5 {
		t.Errorf("repository asked for limit %d, want 5", repo.listedTeams.Page.Limit)
	}
	if len(page.Items) != 5 || page.NextCursor == nil {
		t.Errorf("got %d items, next %v; want 5 and a cursor", len(page.Items), page.NextCursor)
	}
}

// The total costs a second query, so it is counted once per list view: on the first page,
// and never again while the client follows cursors.
func TestListTotalOnFirstPageOnly(t *testing.T) {
	sort := model.SortParams{Field: TeamSortByTeamCode}
	laterPage := model.EncodeCursor(model.Cursor{Sort: sort.String(), LastValue: "41503", LastID: testTeamID})

	tests := []struct {
		name       string
		cursor     string
		wantTotal  *int64
		wantCounts int
	}{
		{"first page", "", ptrInt64(42), 1},
		{"later page", laterPage, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			page, err := NewService(repo, fakePermissions{}, testLogger).ListTeams(context.Background(), ListTeamsServiceParams{
				Sort: sort,
				Page: model.CursorParams{Cursor: tt.cursor},
			})
			if err != nil {
				t.Fatal(err)
			}
			if repo.countCalls != tt.wantCounts {
				t.Errorf("count queries = %d, want %d", repo.countCalls, tt.wantCounts)
			}
			if (page.Total == nil) != (tt.wantTotal == nil) || (page.Total != nil && *page.Total != *tt.wantTotal) {
				t.Errorf("total = %v, want %v", page.Total, tt.wantTotal)
			}
		})
	}

	t.Run("roster first page", func(t *testing.T) {
		repo := newFakeRepository()
		page, err := NewService(repo, fakePermissions{}, testLogger).ListRosterPlayers(context.Background(), ListRosterPlayersServiceParams{
			DivisionID: testDivisionID, TeamID: testTeamID, Sort: model.SortParams{Field: RosterSortByFullName},
		})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total == nil || *page.Total != 7 {
			t.Errorf("total = %v, want 7", page.Total)
		}
	})
}

func ptrInt64(value int64) *int64 { return &value }

// teams.division_id is NOT NULL. A missing or malformed division_id must stop at a 400 in
// the service, not reach Postgres as a NOT NULL or uuid-cast error served as a 500.
func TestCreateTeamRequiresDivision(t *testing.T) {
	tests := []struct {
		name       string
		divisionID string
		wantCode   handler.ErrorCode
	}{
		{"valid", testDivisionID, ""},
		{"missing", "", handler.CodeValidationFailed},
		{"not a uuid", "abc", handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepository()
			_, err := NewService(repo, fakePermissions{}, testLogger).CreateTeam(context.Background(), CreateTeamServiceParams{
				DivisionID: tt.divisionID,
				TeamCode:   "41503",
				Name:       "Cue Masters",
			})

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", got, tt.wantCode, err)
			}
			if (repo.created != nil) != (tt.wantCode == "") {
				t.Errorf("reached repository = %v, want %v", repo.created != nil, tt.wantCode == "")
			}
			if repo.created != nil && repo.created.DivisionID != tt.divisionID {
				t.Errorf("division_id passed on = %q, want %q", repo.created.DivisionID, tt.divisionID)
			}
		})
	}
}
