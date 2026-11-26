// Package sessions owns league sessions: the seasons of play stored in league_sessions.
// They are unrelated to the login sessions in auth.
package sessions

import (
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

// LeagueSessionType and LeagueSessionStatus mirror the CHECKs on league_sessions in
// 000001-init.sql; a value added there must be added here.
type LeagueSessionType string

const (
	LeagueSessionTypeSpring LeagueSessionType = "SPRING"
	LeagueSessionTypeSummer LeagueSessionType = "SUMMER"
	LeagueSessionTypeFall   LeagueSessionType = "FALL"
)

var leagueSessionTypes = []LeagueSessionType{
	LeagueSessionTypeSpring, LeagueSessionTypeSummer, LeagueSessionTypeFall,
}

type LeagueSessionStatus string

const (
	LeagueSessionStatusDraft      LeagueSessionStatus = "DRAFT"
	LeagueSessionStatusInProgress LeagueSessionStatus = "IN_PROGRESS"
	LeagueSessionStatusComplete   LeagueSessionStatus = "COMPLETE"
	LeagueSessionStatusCancelled  LeagueSessionStatus = "CANCELLED"
)

var leagueSessionStatuses = []LeagueSessionStatus{
	LeagueSessionStatusDraft, LeagueSessionStatusInProgress,
	LeagueSessionStatusComplete, LeagueSessionStatusCancelled,
}

// Fields a league session list can be sorted by: ?sort=name, or ?sort=-name for descending.
const (
	LeagueSessionSortByName       = "name"
	LeagueSessionSortByStartsOn   = "starts_on"
	LeagueSessionSortBySeasonYear = "season_year"
)

var leagueSessionSortFields = []string{
	LeagueSessionSortByName, LeagueSessionSortByStartsOn, LeagueSessionSortBySeasonYear,
}

// Newest first. model.ParseSort defaults to ascending, so an empty ?sort= is replaced
// with this rather than passed a default field.
var defaultLeagueSessionSort = model.SortParams{Field: LeagueSessionSortByStartsOn, Desc: true}

const (
	leagueSessionNameMinLength = 3
	leagueSessionNameMaxLength = 60
	leagueSessionMinSeasonYear = 2000
	leagueSessionMaxSeasonYear = 2100
	leagueSessionListLimit     = 200
)

// LeagueSession is a league_sessions row plus two values derived from its team matches.
// StartsOn and EndsOn are DATEs: midnight UTC, with no time-of-day meaning.
type LeagueSession struct {
	ID          string              `db:"id"`
	LeagueID    string              `db:"league_id"`
	Name        string              `db:"name"`
	SessionType LeagueSessionType   `db:"session_type"`
	SeasonYear  int16               `db:"season_year"`
	StartsOn    time.Time           `db:"starts_on"`
	EndsOn      time.Time           `db:"ends_on"`
	Status      LeagueSessionStatus `db:"status"`
	IsActive    bool                `db:"is_active"`
	// Distinct team_matches.week_no values across the session's divisions.
	WeekCount int64 `db:"week_count"`
	// True when any of the session's team matches has status submitted.
	HasSubmittedResults bool      `db:"has_submitted_results"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

// LeagueSessionWeek summarises one week_no of a session's team matches.
type LeagueSessionWeek struct {
	WeekNo           int16     `db:"week_no"`
	MatchCount       int64     `db:"match_count"`
	SubmittedCount   int64     `db:"submitted_count"`
	FirstScheduledAt time.Time `db:"first_scheduled_at"`
}

// ListLeagueSessionsServiceParams filters are ANDed; an empty one is not applied.
type ListLeagueSessionsServiceParams struct {
	Query       string
	SessionType LeagueSessionType
	SeasonYear  int
	Status      LeagueSessionStatus
	Sort        model.SortParams
}

type CreateLeagueSessionServiceParams struct {
	Name        string
	SessionType LeagueSessionType
	SeasonYear  int
	StartsOn    time.Time
	EndsOn      time.Time
	Status      LeagueSessionStatus
	IsActive    bool
}

// UpdateLeagueSessionServiceParams is a partial update: nil leaves a field unchanged.
type UpdateLeagueSessionServiceParams struct {
	Name        *string
	SessionType *LeagueSessionType
	SeasonYear  *int
	StartsOn    *time.Time
	EndsOn      *time.Time
	Status      *LeagueSessionStatus
	IsActive    *bool
}
