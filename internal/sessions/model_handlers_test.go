package sessions

import (
	"reflect"
	"testing"
	"time"
)

// A date served with a time part ("2026-09-01T00:00:00Z") breaks every client that
// compares or displays it as YYYY-MM-DD, and a field missed by the mapper is served as
// its zero value: a session that silently reads as inactive or with no weeks.
func TestNewLeagueSessionResponse(t *testing.T) {
	createdAt := time.Date(2026, 8, 15, 10, 30, 0, 0, time.UTC)
	session := LeagueSession{
		ID:                  "6f1c2a9e-0b3d-4e5f-8a7b-1c2d3e4f5a6b",
		LeagueID:            "9a8b7c6d-5e4f-4a3b-2c1d-0e9f8a7b6c5d",
		Name:                "Fall 2026",
		SessionType:         LeagueSessionTypeFall,
		SeasonYear:          2026,
		StartsOn:            time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:              time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		Status:              LeagueSessionStatusInProgress,
		IsActive:            true,
		WeekCount:           10,
		HasSubmittedResults: true,
		CreatedAt:           createdAt,
		UpdatedAt:           createdAt.Add(time.Hour),
	}

	response := newLeagueSessionResponse(session)

	if response.StartsOn != "2026-09-01" || response.EndsOn != "2026-12-20" {
		t.Errorf("dates = %q, %q; want 2026-09-01, 2026-12-20", response.StartsOn, response.EndsOn)
	}

	value := reflect.ValueOf(response)
	for i := range value.NumField() {
		if value.Field(i).IsZero() {
			t.Errorf("LeagueSessionResponse.%s was not filled", value.Type().Field(i).Name)
		}
	}
}

func TestNewLeagueSessionWeekResponse(t *testing.T) {
	week := LeagueSessionWeek{
		WeekNo:           3,
		MatchCount:       4,
		SubmittedCount:   2,
		FirstScheduledAt: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
	}

	response := newLeagueSessionWeekResponse(week)

	value := reflect.ValueOf(response)
	for i := range value.NumField() {
		if value.Field(i).IsZero() {
			t.Errorf("LeagueSessionWeekResponse.%s was not filled", value.Type().Field(i).Name)
		}
	}
}
