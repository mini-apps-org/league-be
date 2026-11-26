package sessions

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
	"github.com/nguyentanvinh/league-be/internal/platform/model"
)

type Handler struct {
	handler.Handler
	service Service
}

func NewHandler(cfg *config.Config, service Service) Handler {
	return Handler{Handler: handler.Handler{Config: cfg}, service: service}
}

// @Summary		List league sessions
// @Description	Paged: `limit` rows per page (default 10). Pass `metadata.next_cursor` back as `cursor` for the next page, with the same filters and sort; it is null on the last page. `metadata.total` (rows matching the filters) is sent on the first page only. Soft-deleted sessions are hidden.
// @Tags			sessions
// @Produce		json
// @Param			q				query		string	false	"case-insensitive match on name"
// @Param			session_type	query		string	false	"session type"								Enums(SPRING, SUMMER, FALL)
// @Param			season_year		query		int		false	"season year"								minimum(2000)	maximum(2100)
// @Param			status			query		string	false	"status"									Enums(DRAFT, IN_PROGRESS, COMPLETE, CANCELLED)
// @Param			sort			query		string	false	"sort order; a leading - sorts descending"	Enums(name, -name, starts_on, -starts_on, season_year, -season_year)	default(-starts_on)
// @Param			limit			query		int		false	"rows per page, 1-200 (default 10)"
// @Param			cursor			query		string	false	"metadata.next_cursor of the previous page"
// @Success		200				{object}	handler.SuccessResponse[[]sessions.LeagueSessionResponse]
// @Failure		400				{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401				{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403				{object}	handler.ErrorResponse	"403005 PERMISSION_DENIED"
// @Router			/sessions [get]
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	sort, err := parseLeagueSessionSort(query.Get("sort"))
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	seasonYear, err := parseOptionalSeasonYear(query.Get("season_year"))
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	limit, err := model.ParseLimit(query.Get("limit"))
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	page, err := h.service.ListSessions(r.Context(), ListLeagueSessionsServiceParams{
		Query:       query.Get("q"),
		SessionType: LeagueSessionType(query.Get("session_type")),
		SeasonYear:  seasonYear,
		Status:      LeagueSessionStatus(query.Get("status")),
		Sort:        sort,
		Page:        model.CursorParams{Limit: limit, Cursor: query.Get("cursor")},
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]LeagueSessionResponse, 0, len(page.Items))
	for _, session := range page.Items {
		items = append(items, newLeagueSessionResponse(session))
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[[]LeagueSessionResponse]{
		Data:     items,
		Metadata: handler.NewPageMetadata(len(items), page.Total, page.NextCursor),
	})
}

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
func (h *Handler) GetSessionByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	session, err := h.service.GetSessionByID(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[LeagueSessionResponse]{Data: newLeagueSessionResponse(*session)})
}

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
// @Failure		409		{object}	handler.ErrorResponse	"409001 CONFLICT (duplicate name, or another session made active at the same time)"
// @Security		CSRFToken
// @Router			/sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var body CreateLeagueSessionRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	startsOn, err := parseLeagueSessionDate("starts_on", body.StartsOn)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	endsOn, err := parseLeagueSessionDate("ends_on", body.EndsOn)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	params := CreateLeagueSessionServiceParams{
		Name:        body.Name,
		SessionType: body.SessionType,
		SeasonYear:  body.SeasonYear,
		StartsOn:    startsOn,
		EndsOn:      endsOn,
	}
	if body.Status != nil {
		params.Status = *body.Status
	}
	if body.IsActive != nil {
		params.IsActive = *body.IsActive
	}

	session, err := h.service.CreateSession(r.Context(), params)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusCreated, handler.SuccessResponse[LeagueSessionResponse]{Data: newLeagueSessionResponse(*session)})
}

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
// @Failure		409		{object}	handler.ErrorResponse	"409001 CONFLICT (duplicate name, or another session made active at the same time)"
// @Security		CSRFToken
// @Router			/sessions/{id} [patch]
func (h *Handler) UpdateSessionByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body UpdateLeagueSessionRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	startsOn, err := parseOptionalLeagueSessionDate("starts_on", body.StartsOn)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	endsOn, err := parseOptionalLeagueSessionDate("ends_on", body.EndsOn)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	session, err := h.service.UpdateSessionByID(r.Context(), id, UpdateLeagueSessionServiceParams{
		Name:        body.Name,
		SessionType: body.SessionType,
		SeasonYear:  body.SeasonYear,
		StartsOn:    startsOn,
		EndsOn:      endsOn,
		Status:      body.Status,
		IsActive:    body.IsActive,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[LeagueSessionResponse]{Data: newLeagueSessionResponse(*session)})
}

// @Summary		Delete a league session
// @Description	Soft delete: the session is hidden from every read and stops being active. Its divisions, teams and matches are kept. Remove it for good with DELETE /sessions/{id}/permanent.
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
func (h *Handler) DeleteSessionByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.DeleteSessionByID(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// @Summary		Permanently delete a league session
// @Description	Only a session already deleted with DELETE /sessions/{id}. Cascades to the session's divisions, the teams created in them, and their matches, including submitted results. Cannot be undone. Refused with 409 while a team created in this session also plays in another session, since the cascade would remove it from that session too.
// @Tags			sessions
// @Produce		json
// @Param			id	path	string	true	"league session id (uuid)"
// @Success		204	"permanently deleted"
// @Failure		400	{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure		401	{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403	{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Failure		404	{object}	handler.ErrorResponse	"404001 NOT_FOUND"
// @Failure		409	{object}	handler.ErrorResponse	"409001 CONFLICT (not deleted yet, or a team created here plays in another session)"
// @Security		CSRFToken
// @Router			/sessions/{id}/permanent [delete]
func (h *Handler) PermanentDeleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.PermanentDeleteSession(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// @Summary		Soft-delete several league sessions
// @Description	Best effort, in one statement: every live session in `ids` is soft-deleted, as DELETE /sessions/{id} does. A session that cannot be deleted is listed in `failed` with the code and message the single delete returns (404001 missing or already deleted); the others are still deleted. Always 200 once the request itself is valid. 1-100 ids; repeats are ignored.
// @Tags			sessions
// @Accept			json
// @Produce		json
// @Param			body	body		sessions.BulkDeleteLeagueSessionsRequest	true	"league sessions to soft-delete"
// @Success		200		{object}	handler.SuccessResponse[sessions.BulkDeleteLeagueSessionsResponse]
// @Failure		400		{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED (no ids, more than 100, or one is not a uuid)"
// @Failure		401		{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403		{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Security		CSRFToken
// @Router			/sessions [delete]
func (h *Handler) BulkSoftDeleteSessions(w http.ResponseWriter, r *http.Request) {
	var body BulkDeleteLeagueSessionsRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	result, err := h.service.BulkSoftDeleteSessions(r.Context(), body.IDs)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[BulkDeleteLeagueSessionsResponse]{Data: newBulkDeleteLeagueSessionsResponse(*result)})
}

// @Summary		Permanently delete several league sessions
// @Description	Best effort, in one transaction: every session in `ids` that is soft-deleted and homes no team playing in another session is removed, as DELETE /sessions/{id}/permanent does, cascade included. A session that cannot be deleted is listed in `failed` (404001 missing, 409001 not deleted yet or a team created there plays in another session); the others are still deleted. Always 200 once the request itself is valid. 1-100 ids; repeats are ignored. Cannot be undone.
// @Tags			sessions
// @Accept			json
// @Produce		json
// @Param			body	body		sessions.BulkDeleteLeagueSessionsRequest	true	"soft-deleted league sessions to remove"
// @Success		200		{object}	handler.SuccessResponse[sessions.BulkDeleteLeagueSessionsResponse]
// @Failure		400		{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED (no ids, more than 100, or one is not a uuid)"
// @Failure		401		{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403		{object}	handler.ErrorResponse	"403001 CSRF_COOKIE_ABSENT | 403002 CSRF_TOKEN_MISMATCH | 403005 PERMISSION_DENIED"
// @Failure		409		{object}	handler.ErrorResponse	"409001 CONFLICT (a team entered another session while the batch ran; retry)"
// @Security		CSRFToken
// @Router			/sessions/permanent [delete]
func (h *Handler) BulkPermanentDeleteSessions(w http.ResponseWriter, r *http.Request) {
	var body BulkDeleteLeagueSessionsRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	result, err := h.service.BulkPermanentDeleteSessions(r.Context(), body.IDs)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[BulkDeleteLeagueSessionsResponse]{Data: newBulkDeleteLeagueSessionsResponse(*result)})
}

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
func (h *Handler) ListSessionWeeks(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	weeks, err := h.service.ListSessionWeeks(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]LeagueSessionWeekResponse, 0, len(weeks))
	for _, week := range weeks {
		items = append(items, newLeagueSessionWeekResponse(week))
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[[]LeagueSessionWeekResponse]{
		Data:     items,
		Metadata: handler.NewMetadata(len(items)),
	})
}

// parseLeagueSessionSort falls back to defaultLeagueSessionSort itself, because
// model.ParseSort's default is always ascending.
func parseLeagueSessionSort(raw string) (model.SortParams, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultLeagueSessionSort, nil
	}
	return model.ParseSort(raw, leagueSessionSortFields, defaultLeagueSessionSort.Field)
}

func parseOptionalSeasonYear(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	year, err := strconv.Atoi(raw)
	if err != nil {
		return 0, handler.ErrorWithMessage(handler.CodeValidationFailed, "season_year must be a whole number", err)
	}
	return year, nil
}

func parseLeagueSessionDate(field, value string) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, handler.ErrorWithMessage(handler.CodeValidationFailed, field+" must be a date as YYYY-MM-DD", err)
	}
	return date, nil
}

func parseOptionalLeagueSessionDate(field string, value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	date, err := parseLeagueSessionDate(field, *value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}
