package sessions

import "time"

// The validate tags only mark the required fields in swagger; no validator reads them.
// The service validates the body.
type CreateLeagueSessionRequest struct {
	Name        string  `json:"name" validate:"required" example:"Spring 2027" minLength:"3" maxLength:"60"`
	SessionType string  `json:"session_type" validate:"required" enums:"SPRING,SUMMER,FALL" example:"SPRING"`
	SeasonYear  int     `json:"season_year" validate:"required" minimum:"2000" maximum:"2100" example:"2027"`
	StartsOn    string  `json:"starts_on" validate:"required" format:"date" example:"2027-01-10"`
	EndsOn      string  `json:"ends_on" validate:"required" format:"date" example:"2027-04-30"`
	Status      *string `json:"status,omitempty" enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED" default:"DRAFT" example:"DRAFT"`
	IsActive    *bool   `json:"is_active,omitempty" default:"false" example:"false"`
}

type UpdateLeagueSessionRequest struct {
	Name        *string `json:"name,omitempty" example:"Spring 2027" minLength:"3" maxLength:"60"`
	SessionType *string `json:"session_type,omitempty" enums:"SPRING,SUMMER,FALL" example:"SPRING"`
	SeasonYear  *int    `json:"season_year,omitempty" minimum:"2000" maximum:"2100" example:"2027"`
	StartsOn    *string `json:"starts_on,omitempty" format:"date" example:"2027-01-10"`
	EndsOn      *string `json:"ends_on,omitempty" format:"date" example:"2027-04-30"`
	Status      *string `json:"status,omitempty" enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED" example:"IN_PROGRESS"`
	IsActive    *bool   `json:"is_active,omitempty" example:"true"`
}

type LeagueSessionResponse struct {
	ID          string `json:"id" format:"uuid"`
	LeagueID    string `json:"league_id" format:"uuid"`
	Name        string `json:"name" example:"Fall 2026"`
	SessionType string `json:"session_type" enums:"SPRING,SUMMER,FALL" example:"FALL"`
	SeasonYear  int    `json:"season_year" example:"2026"`
	StartsOn    string `json:"starts_on" format:"date" example:"2026-09-01"`
	EndsOn      string `json:"ends_on" format:"date" example:"2026-12-20"`
	Status      string `json:"status" enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED" example:"IN_PROGRESS"`
	IsActive    bool   `json:"is_active" example:"true"`
	// Distinct team_matches.week_no values across the session's divisions.
	WeekCount int `json:"week_count" example:"10"`
	// True when any of the session's team matches has status submitted.
	HasSubmittedResults bool      `json:"has_submitted_results" example:"true"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type LeagueSessionWeekResponse struct {
	WeekNo           int       `json:"week_no" example:"1"`
	MatchCount       int       `json:"match_count" example:"4"`
	SubmittedCount   int       `json:"submitted_count" example:"3"`
	FirstScheduledAt time.Time `json:"first_scheduled_at"`
}
