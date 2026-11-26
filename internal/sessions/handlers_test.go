package sessions

import (
	"strings"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// The contract promises -starts_on when ?sort= is empty. model.ParseSort alone would sort
// oldest first, so the list would open on a session from years ago.
func TestParseLeagueSessionSort(t *testing.T) {
	tests := []struct {
		raw       string
		wantField string
		wantDesc  bool
		wantCode  handler.ErrorCode
	}{
		{"", LeagueSessionSortByStartsOn, true, 0},
		{"  ", LeagueSessionSortByStartsOn, true, 0},
		{"name", LeagueSessionSortByName, false, 0},
		{"-name", LeagueSessionSortByName, true, 0},
		{"starts_on", LeagueSessionSortByStartsOn, false, 0},
		{"-starts_on", LeagueSessionSortByStartsOn, true, 0},
		{"season_year", LeagueSessionSortBySeasonYear, false, 0},
		{"-season_year", LeagueSessionSortBySeasonYear, true, 0},
		{"created_at", "", false, handler.CodeValidationFailed},
		{"name;DROP TABLE league_sessions", "", false, handler.CodeValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			sort, err := parseLeagueSessionSort(tt.raw)

			if got := errorCode(err); got != tt.wantCode {
				t.Fatalf("code = %d, want %d", got, tt.wantCode)
			}
			if err == nil && (sort.Field != tt.wantField || sort.Desc != tt.wantDesc) {
				t.Errorf("sort = %+v, want %s desc=%v", sort, tt.wantField, tt.wantDesc)
			}
			if err == nil {
				if column, _ := leagueSessionSortColumn(sort.Field); !strings.HasSuffix(column, "."+sort.Field) {
					t.Errorf("sort field %q orders by %s: the default branch swallowed it", sort.Field, column)
				}
			}
		})
	}
}

// A date that is not YYYY-MM-DD must be a 400 naming the field. Accepting 01/10/2027 would
// store January or October depending on who typed it.
func TestParseLeagueSessionDate(t *testing.T) {
	for _, raw := range []string{"2027-01-10", " 2027-01-10 "} {
		if _, err := parseLeagueSessionDate("starts_on", raw); err != nil {
			t.Errorf("%q rejected: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "01/10/2027", "2027-1-10", "2027-02-30", "2027-01-10T00:00:00Z"} {
		if _, err := parseLeagueSessionDate("starts_on", raw); errorCode(err) != handler.CodeValidationFailed {
			t.Errorf("%q accepted or not a 400: %v", raw, err)
		}
	}
}

func TestParseOptionalSeasonYear(t *testing.T) {
	if year, err := parseOptionalSeasonYear(""); err != nil || year != 0 {
		t.Errorf("empty = %d, %v; want 0, nil (no filter)", year, err)
	}
	if year, err := parseOptionalSeasonYear("2027"); err != nil || year != 2027 {
		t.Errorf("2027 = %d, %v", year, err)
	}
	if _, err := parseOptionalSeasonYear("twenty"); errorCode(err) != handler.CodeValidationFailed {
		t.Errorf("non-number not a 400: %v", err)
	}
}
