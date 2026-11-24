package sessions

import (
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// Handler is the swagger contract for /sessions. The method bodies are filled in with the
// service; until then the handlers are not registered in the router.
type Handler struct {
	handler.Handler
}

// @Summary		List league sessions
// @Description	Sessions of the league, capped at 200 rows. Not paged: metadata carries count only.
// @Tags			sessions
// @Produce		json
// @Param			q				query		string	false	"case-insensitive match on name"
// @Param			session_type	query		string	false	"session type"								Enums(SPRING, SUMMER, FALL)
// @Param			season_year		query		int		false	"season year"								minimum(2000)	maximum(2100)
// @Param			status			query		string	false	"status"									Enums(DRAFT, IN_PROGRESS, COMPLETE, CANCELLED)
// @Param			sort			query		string	false	"sort order; a leading - sorts descending"	Enums(name, -name, starts_on, -starts_on, season_year, -season_year)	default(-starts_on)
// @Success		200				{object}	handler.SuccessResponse[[]sessions.LeagueSessionResponse]
// @Failure		400				{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401				{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403				{object}	handler.ErrorResponse	"403005 PERMISSION_DENIED"
// @Router			/sessions [get]
func (h *Handler) ListLeagueSessions(w http.ResponseWriter, r *http.Request) {}

// @Summary	Get a league session
// @Tags		sessions
// @Produce	json
// @Param		id	path		string	true	"league session id (uuid)"
// @Success	200	{object}	handler.SuccessResponse[sessions.LeagueSessionResponse]
// @Failure	400	{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure	401	{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure	403	{object}	handler.ErrorResponse	"403005 PERMISSION_DENIED"
// @Failure	404	{object}	handler.ErrorResponse	"404001 NOT_FOUND"
// @Router		/sessions/{id} [get]
func (h *Handler) GetLeagueSession(w http.ResponseWriter, r *http.Request) {}

// @Summary		Create a league session
// @Description	`name` is trimmed and must be 3–60 characters, unique within the league. `season_year` is 2000–2100. `ends_on` must be on or after `starts_on`. A missing `status` becomes DRAFT. `is_active: true` deactivates the league's other sessions.
// @Tags			sessions
// @Accept			json
// @Produce		json
// @Param			body	body		sessions.CreateLeagueSessionRequest	true	"league session to create"
// @Success		201		{object}	handler.SuccessResponse[sessions.LeagueSessionResponse]
// @Failure		400		{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401		{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403		{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Failure		409		{object}	handler.ErrorResponse	"409001 CONFLICT (duplicate name)"
// @Security		CSRFToken
// @Router			/sessions [post]
func (h *Handler) CreateLeagueSession(w http.ResponseWriter, r *http.Request) {}

// @Summary		Update a league session
// @Description	Partial update: an absent field is left unchanged, and an empty body is rejected. The create rules apply to the merged result, so moving only `ends_on` before the existing `starts_on` fails. `is_active: true` deactivates the league's other sessions.
// @Tags			sessions
// @Accept			json
// @Produce		json
// @Param			id		path		string								true	"league session id (uuid)"
// @Param			body	body		sessions.UpdateLeagueSessionRequest	true	"fields to change"
// @Success		200		{object}	handler.SuccessResponse[sessions.LeagueSessionResponse]
// @Failure		400		{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401		{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403		{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Failure		404		{object}	handler.ErrorResponse	"404001 NOT_FOUND"
// @Failure		409		{object}	handler.ErrorResponse	"409001 CONFLICT (duplicate name)"
// @Security		CSRFToken
// @Router			/sessions/{id} [patch]
func (h *Handler) UpdateLeagueSession(w http.ResponseWriter, r *http.Request) {}

// @Summary		Delete a league session
// @Description	Hard delete. Cascades to the session's divisions, teams and matches, including submitted results. Cannot be undone.
// @Tags			sessions
// @Produce		json
// @Param			id	path	string	true	"league session id (uuid)"
// @Success		204	"deleted"
// @Failure		400	{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401	{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403	{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Failure		404	{object}	handler.ErrorResponse	"404001 NOT_FOUND"
// @Security		CSRFToken
// @Router			/sessions/{id} [delete]
func (h *Handler) DeleteLeagueSession(w http.ResponseWriter, r *http.Request) {}

// @Summary		List a league session's weeks
// @Description	One row per week that has team matches, ordered by week number. An existing session with no matches returns an empty list.
// @Tags			sessions
// @Produce		json
// @Param			id	path		string	true	"league session id (uuid)"
// @Success		200	{object}	handler.SuccessResponse[[]sessions.LeagueSessionWeekResponse]
// @Failure		400	{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401	{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403	{object}	handler.ErrorResponse	"403005 PERMISSION_DENIED"
// @Failure		404	{object}	handler.ErrorResponse	"404001 NOT_FOUND"
// @Router			/sessions/{id}/weeks [get]
func (h *Handler) ListLeagueSessionWeeks(w http.ResponseWriter, r *http.Request) {}
