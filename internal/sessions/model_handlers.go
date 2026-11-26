package sessions

import (
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// The validate tags only mark the required fields in swagger; no validator reads them.
// The service validates the body.
type CreateLeagueSessionRequest struct {
	Name        string               `json:"name" validate:"required" example:"Spring 2027" minLength:"3" maxLength:"60"`
	SessionType LeagueSessionType    `json:"session_type" swaggertype:"string" validate:"required" enums:"SPRING,SUMMER,FALL" example:"SPRING"`
	SeasonYear  int                  `json:"season_year" validate:"required" minimum:"2000" maximum:"2100" example:"2027"`
	StartsOn    string               `json:"starts_on" validate:"required" format:"date" example:"2027-01-10"`
	EndsOn      string               `json:"ends_on" validate:"required" format:"date" example:"2027-04-30"`
	Status      *LeagueSessionStatus `json:"status,omitempty" swaggertype:"string" enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED" default:"DRAFT" example:"DRAFT"`
	IsActive    *bool                `json:"is_active,omitempty" default:"false" example:"false"`
}

type UpdateLeagueSessionRequest struct {
	Name        *string              `json:"name,omitempty" example:"Spring 2027" minLength:"3" maxLength:"60"`
	SessionType *LeagueSessionType   `json:"session_type,omitempty" swaggertype:"string" enums:"SPRING,SUMMER,FALL" example:"SPRING"`
	SeasonYear  *int                 `json:"season_year,omitempty" minimum:"2000" maximum:"2100" example:"2027"`
	StartsOn    *string              `json:"starts_on,omitempty" format:"date" example:"2027-01-10"`
	EndsOn      *string              `json:"ends_on,omitempty" format:"date" example:"2027-04-30"`
	Status      *LeagueSessionStatus `json:"status,omitempty" swaggertype:"string" enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED" example:"IN_PROGRESS"`
	IsActive    *bool                `json:"is_active,omitempty" example:"true"`
}

type BulkDeleteLeagueSessionsRequest struct {
	IDs []string `json:"ids" example:"5e8a2c1d-3b7f-4a90-8c6e-2d1f0b9a7e44"`
}

// BulkDeleteLeagueSessionsResponse lists every requested session once: deleted, or failed
// with the error the single delete would have returned.
type BulkDeleteLeagueSessionsResponse struct {
	DeletedIDs []string                                 `json:"deleted_ids"`
	Failed     []BulkDeleteLeagueSessionFailureResponse `json:"failed"`
}

type BulkDeleteLeagueSessionFailureResponse struct {
	ID      string            `json:"id"`
	Code    handler.ErrorCode `json:"code" example:"404001"`
	Message string            `json:"message" example:"league session not found"`
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

func newLeagueSessionResponse(session LeagueSession) LeagueSessionResponse {
	return LeagueSessionResponse{
		ID:                  session.ID,
		LeagueID:            session.LeagueID,
		Name:                session.Name,
		SessionType:         string(session.SessionType),
		SeasonYear:          int(session.SeasonYear),
		StartsOn:            session.StartsOn.Format(time.DateOnly),
		EndsOn:              session.EndsOn.Format(time.DateOnly),
		Status:              string(session.Status),
		IsActive:            session.IsActive,
		WeekCount:           int(session.WeekCount),
		HasSubmittedResults: session.HasSubmittedResults,
		CreatedAt:           session.CreatedAt,
		UpdatedAt:           session.UpdatedAt,
	}
}

func newLeagueSessionWeekResponse(week LeagueSessionWeek) LeagueSessionWeekResponse {
	return LeagueSessionWeekResponse{
		WeekNo:           int(week.WeekNo),
		MatchCount:       int(week.MatchCount),
		SubmittedCount:   int(week.SubmittedCount),
		FirstScheduledAt: week.FirstScheduledAt,
	}
}

// newBulkDeleteLeagueSessionsResponse serves each failure as its public code and message;
// handler.AsAPIError turns an unclassified error into 500001 without its cause.
func newBulkDeleteLeagueSessionsResponse(result BulkDeleteLeagueSessionsResult) BulkDeleteLeagueSessionsResponse {
	failed := make([]BulkDeleteLeagueSessionFailureResponse, 0, len(result.Failures))
	for _, failure := range result.Failures {
		apiErr := handler.AsAPIError(failure.Err)
		failed = append(failed, BulkDeleteLeagueSessionFailureResponse{
			ID:      failure.LeagueSessionID,
			Code:    apiErr.Code,
			Message: apiErr.Message,
		})
	}
	return BulkDeleteLeagueSessionsResponse{DeletedIDs: result.DeletedIDs, Failed: failed}
}
