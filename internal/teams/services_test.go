package teams

import (
	"context"
	"errors"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
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
			_, err := NewService(repo, fakePermissions{}).CreateTeam(context.Background(), CreateTeamServiceParams{
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
			err := tt.run(NewService(repo, fakePermissions{}))

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
			_, err := NewService(repo, fakePermissions{}).UpdateTeam(context.Background(), testTeamID, UpdateTeamServiceParams{
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
	_, err := NewService(repo, fakePermissions{}).TransferRosterPlayer(context.Background(), testAdminUser, TransferRosterPlayerServiceParams{
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
			err := tt.run(NewService(repo, fakePermissions{}), tt.user)

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
			_, err := NewService(repo, fakePermissions{}).CreateTeam(context.Background(), CreateTeamServiceParams{
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
