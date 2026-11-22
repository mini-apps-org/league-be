package teams

import (
	"errors"
	"net/http"

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

// @Summary		List teams
// @Description	Paged: `limit` rows per page (default 10). Pass `metadata.next_cursor` back as `cursor` for the next page, with the same filters and sort; it is null on the last page. `metadata.total` (rows matching the filters) is sent on the first page only.
// @Tags			teams
// @Produce		json
// @Param			division_id	query		string	false	"only teams entered in this division (uuid)"
// @Param			venue_id	query		string	false	"only teams hosted at this venue (uuid)"
// @Param			q			query		string	false	"search: name or team_code contains this, case-insensitive"
// @Param			sort		query		string	false	"sort field, - prefix for descending (default team_code)"	Enums(team_code, -team_code, name, -name, created_at, -created_at)
// @Param			limit		query		int		false	"rows per page, 1-200 (default 10)"
// @Param			cursor		query		string	false	"metadata.next_cursor of the previous page"
// @Success		200			{object}	handler.SuccessResponse[[]teams.TeamResponse]
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		401			{object}	handler.ErrorResponse	"40101 SESSION_COOKIE_ABSENT | 40102 SESSION_INVALID"
// @Failure		403			{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED"
// @Router			/teams [get]
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	sort, err := model.ParseSort(query.Get("sort"), teamSortFields, TeamSortByTeamCode)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	limit, err := model.ParseLimit(query.Get("limit"))
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	page, err := h.service.ListTeams(r.Context(), ListTeamsServiceParams{
		DivisionID: query.Get("division_id"),
		VenueID:    query.Get("venue_id"),
		Query:      query.Get("q"),
		Sort:       sort,
		Page:       model.CursorParams{Limit: limit, Cursor: query.Get("cursor")},
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]TeamResponse, 0, len(page.Items))
	for _, team := range page.Items {
		items = append(items, newTeamResponse(team))
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[[]TeamResponse]{
		Data:     items,
		Metadata: handler.NewPageMetadata(len(items), page.Total, page.NextCursor),
	})
}

// @Summary		Create a team
// @Description	`division_id` is required: the team is created in that division and entered into it. `team_code` is the team code XXyyA (region, host location, team sequence), unique within the division; lower case is accepted and stored upper case. It can enter further divisions with POST /teams/{id}/divisions, and the captain is set with PATCH once the player is on the roster.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			body	body		teams.CreateTeamRequest	true	"team to create"
// @Success		201		{object}	handler.SuccessResponse[teams.TeamResponse]
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		403		{object}	handler.ErrorResponse	"40301 CSRF_COOKIE_ABSENT | 40302 CSRF_TOKEN_MISMATCH | 40305 PERMISSION_DENIED"
// @Failure		409		{object}	handler.ErrorResponse	"40901 CONFLICT (team_code taken)"
// @Security		CSRFToken
// @Router			/teams [post]
func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var body CreateTeamRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	team, err := h.service.CreateTeam(r.Context(), CreateTeamServiceParams{ //nolint:staticcheck // S1016: explicit mapping keeps the JSON and service types independent
		DivisionID: body.DivisionID,
		TeamCode:   body.TeamCode,
		Name:       body.Name,
		VenueID:    body.VenueID,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusCreated, handler.SuccessResponse[TeamResponse]{Data: newTeamResponse(*team)})
}

// @Summary	Get a team
// @Tags		teams
// @Produce	json
// @Param		id	path		string	true	"team id (uuid)"
// @Success	200	{object}	handler.SuccessResponse[teams.TeamResponse]
// @Failure	400	{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure	404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Router		/teams/{id} [get]
func (h *Handler) GetTeamByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	team, err := h.service.GetTeamByID(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[TeamResponse]{Data: newTeamResponse(*team)})
}

// @Summary		Update a team
// @Description	Partial update: an absent field is left unchanged. `captain_player_id` must be on the team's active roster in some division.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"team id (uuid)"
// @Param			body	body		teams.UpdateTeamRequest	true	"fields to change"
// @Success		200		{object}	handler.SuccessResponse[teams.TeamResponse]
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404		{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Failure		409		{object}	handler.ErrorResponse	"40901 CONFLICT (team_code taken)"
// @Security		CSRFToken
// @Router			/teams/{id} [patch]
func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body UpdateTeamRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	team, err := h.service.UpdateTeam(r.Context(), id, UpdateTeamServiceParams{ //nolint:staticcheck // S1016: explicit mapping keeps the JSON and service types independent
		TeamCode:        body.TeamCode,
		Name:            body.Name,
		CaptainPlayerID: body.CaptainPlayerID,
		VenueID:         body.VenueID,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[TeamResponse]{Data: newTeamResponse(*team)})
}

// @Summary		Delete a team
// @Description	Soft delete: the team is hidden and its active roster entries are closed, freeing its players. Fixtures, division entries, roster history and its team_code are kept until DELETE /teams/{id}/permanent.
// @Tags			teams
// @Produce		json
// @Param			id	path	string	true	"team id (uuid)"
// @Success		204	"deleted"
// @Failure		404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Security		CSRFToken
// @Router			/teams/{id} [delete]
func (h *Handler) DeleteTeamByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.DeleteTeamByID(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// @Summary		Soft-delete several teams
// @Description	Best effort, in one statement: every live team in `ids` is soft-deleted and its active roster closed, as DELETE /teams/{id} does. A team that cannot be deleted is listed in `failed` with the code and message the single delete returns (40401 missing or already deleted); the others are still deleted. Always 200 once the request itself is valid. 1-100 ids; repeats are ignored.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			body	body		teams.BulkDeleteTeamsRequest	true	"teams to soft-delete"
// @Success		200		{object}	handler.SuccessResponse[teams.BulkDeleteTeamsResponse]
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED (no ids, more than 100, or one is not a uuid)"
// @Failure		403		{object}	handler.ErrorResponse	"40301 CSRF_COOKIE_ABSENT | 40302 CSRF_TOKEN_MISMATCH | 40305 PERMISSION_DENIED"
// @Security		CSRFToken
// @Router			/teams [delete]
func (h *Handler) BulkSoftDeleteTeams(w http.ResponseWriter, r *http.Request) {
	var body BulkDeleteTeamsRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	result, err := h.service.BulkSoftDeleteTeams(r.Context(), body.IDs)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[BulkDeleteTeamsResponse]{Data: newBulkDeleteTeamsResponse(*result)})
}

// @Summary		Permanently delete several teams
// @Description	Best effort, in one transaction: every team in `ids` that is soft-deleted and has no fixture is removed, as DELETE /teams/{id}/permanent does. A team that cannot be deleted is listed in `failed` (40401 missing, 40901 not deleted yet or has fixtures); the others are still deleted. Always 200 once the request itself is valid. 1-100 ids; repeats are ignored.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			body	body		teams.BulkDeleteTeamsRequest	true	"soft-deleted teams to remove"
// @Success		200		{object}	handler.SuccessResponse[teams.BulkDeleteTeamsResponse]
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED (no ids, more than 100, or one is not a uuid)"
// @Failure		403		{object}	handler.ErrorResponse	"40301 CSRF_COOKIE_ABSENT | 40302 CSRF_TOKEN_MISMATCH | 40305 PERMISSION_DENIED"
// @Security		CSRFToken
// @Router			/teams/permanent [delete]
func (h *Handler) BulkPermanentDeleteTeams(w http.ResponseWriter, r *http.Request) {
	var body BulkDeleteTeamsRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	result, err := h.service.BulkPermanentDeleteTeams(r.Context(), body.IDs)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[BulkDeleteTeamsResponse]{Data: newBulkDeleteTeamsResponse(*result)})
}

// @Summary		Permanently delete a team
// @Description	Only a team already deleted with DELETE /teams/{id}. Cascades to its division entries and roster history; a team with scheduled or played matches cannot be removed. Cannot be undone.
// @Tags			teams
// @Produce		json
// @Param			id	path	string	true	"team id (uuid)"
// @Success		204	"permanently deleted"
// @Failure		404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Failure		409	{object}	handler.ErrorResponse	"40901 CONFLICT (not deleted yet, or team has matches)"
// @Security		CSRFToken
// @Router			/teams/{id}/permanent [delete]
func (h *Handler) PermanentDeleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.PermanentDeleteTeam(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// @Summary		Enter a team into a division
// @Description	How a permanent team plays a season: one entry per division.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			id		path		string							true	"team id (uuid)"
// @Param			body	body		teams.AddDivisionTeamRequest	true	"division to enter"
// @Success		201		{object}	handler.SuccessResponse[teams.DivisionTeamResponse]
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404		{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Failure		409		{object}	handler.ErrorResponse	"40901 CONFLICT (already entered)"
// @Security		CSRFToken
// @Router			/teams/{id}/divisions [post]
func (h *Handler) AddDivisionTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body AddDivisionTeamRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	entry, err := h.service.AddDivisionTeam(r.Context(), body.DivisionID, teamID)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusCreated, handler.SuccessResponse[DivisionTeamResponse]{Data: newDivisionTeamResponse(*entry)})
}

// @Summary		Withdraw a team from a division
// @Description	Cascades to the team's roster in that division. A team with a fixture in the division cannot be withdrawn.
// @Tags			teams
// @Produce		json
// @Param			id			path	string	true	"team id (uuid)"
// @Param			division_id	path	string	true	"division id (uuid)"
// @Success		204			"withdrawn"
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Failure		409			{object}	handler.ErrorResponse	"40901 CONFLICT (team has matches)"
// @Security		CSRFToken
// @Router			/teams/{id}/divisions/{division_id} [delete]
func (h *Handler) RemoveDivisionTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	divisionID, err := handler.ParsePathUUID(r, "division_id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.RemoveDivisionTeam(r.Context(), divisionID, teamID); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// @Summary		List a team's roster in a division
// @Description	Active players only, with their stats in the division (zero before the first match). Paged: `limit` rows per page (default 10); pass `metadata.next_cursor` back as `cursor`, with the same search and sort; it is null on the last page. `metadata.total` (rows matching the filters) is sent on the first page only.
// @Tags			rosters
// @Produce		json
// @Param			id			path		string	true	"team id (uuid)"
// @Param			division_id	query		string	true	"division id (uuid)"
// @Param			q			query		string	false	"search: full_name or member_id contains this, case-insensitive"
// @Param			sort		query		string	false	"sort field, - prefix for descending (default full_name)"	Enums(full_name, -full_name, member_id, -member_id, skill_level, -skill_level, matches_played, -matches_played, matches_won, -matches_won, total_points, -total_points, joined_at, -joined_at)
// @Param			limit		query		int		false	"rows per page, 1-200 (default 10)"
// @Param			cursor		query		string	false	"metadata.next_cursor of the previous page"
// @Success		200			{object}	handler.SuccessResponse[[]teams.RosterPlayerResponse]
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND (team has not entered the division)"
// @Router			/teams-roster/{id} [get]
func (h *Handler) ListRosterPlayers(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	query := r.URL.Query()

	sort, err := model.ParseSort(query.Get("sort"), rosterSortFields, RosterSortByFullName)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	limit, err := model.ParseLimit(query.Get("limit"))
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	page, err := h.service.ListRosterPlayers(r.Context(), ListRosterPlayersServiceParams{
		DivisionID: query.Get("division_id"),
		TeamID:     teamID,
		Query:      query.Get("q"),
		Sort:       sort,
		Page:       model.CursorParams{Limit: limit, Cursor: query.Get("cursor")},
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]RosterPlayerResponse, 0, len(page.Items))
	for _, player := range page.Items {
		items = append(items, newRosterPlayerResponse(player))
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[[]RosterPlayerResponse]{
		Data:     items,
		Metadata: handler.NewPageMetadata(len(items), page.Total, page.NextCursor),
	})
}

// @Summary		Add a player to a team's roster in a division
// @Description	A player can be on only one active roster per division, and the team must have entered the division.
// @Tags			rosters
// @Accept			json
// @Produce		json
// @Param			id			path		string							true	"team id (uuid)"
// @Param			division_id	query		string							true	"division id (uuid)"
// @Param			body		body		teams.AddRosterPlayerRequest	true	"player to add"
// @Success		201			{object}	handler.SuccessResponse[teams.RosterPlayerResponse]
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND (player, or team has not entered the division)"
// @Failure		409			{object}	handler.ErrorResponse	"40901 CONFLICT (already on a roster in the division)"
// @Security		CSRFToken
// @Failure		403	{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED (not the team captain, or an unpaid member)"
// @Router			/teams-roster/{id} [post]
func (h *Handler) AddRosterPlayer(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body AddRosterPlayerRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	actorUserID, err := h.actorUserID(r)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	player, err := h.service.AddRosterPlayer(r.Context(), actorUserID, RosterEntryServiceParams{
		DivisionID: r.URL.Query().Get("division_id"),
		TeamID:     teamID,
		PlayerID:   body.PlayerID,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusCreated, handler.SuccessResponse[RosterPlayerResponse]{Data: newRosterPlayerResponse(*player)})
}

// @Summary		Transfer a player to another team
// @Description	Closes the player's roster entry and opens one on `team_id`, in `division_id` or the current division when absent, in one transaction. The team captain cannot be transferred.
// @Tags			rosters
// @Accept			json
// @Produce		json
// @Param			id			path		string								true	"current team id (uuid)"
// @Param			player_id	path		string								true	"player id (uuid)"
// @Param			division_id	query		string								true	"current division id (uuid)"
// @Param			body		body		teams.TransferRosterPlayerRequest	true	"target team and division"
// @Success		200			{object}	handler.SuccessResponse[teams.RosterPlayerResponse]
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND (team, roster entry, or target team has not entered the division)"
// @Failure		409			{object}	handler.ErrorResponse	"40901 CONFLICT (captain, or already on a roster in the target division)"
// @Security		CSRFToken
// @Failure		403	{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED (not the team captain, or an unpaid member)"
// @Router			/teams-roster/{id}/{player_id} [patch]
func (h *Handler) TransferRosterPlayer(w http.ResponseWriter, r *http.Request) {
	fromTeamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	playerID, err := handler.ParsePathUUID(r, "player_id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body TransferRosterPlayerRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	actorUserID, err := h.actorUserID(r)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	player, err := h.service.TransferRosterPlayer(r.Context(), actorUserID, TransferRosterPlayerServiceParams{
		From: RosterEntryServiceParams{
			DivisionID: r.URL.Query().Get("division_id"),
			TeamID:     fromTeamID,
			PlayerID:   playerID,
		},
		ToDivisionID: body.DivisionID,
		ToTeamID:     body.TeamID,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[RosterPlayerResponse]{Data: newRosterPlayerResponse(*player)})
}

// @Summary		Remove a player from a team's roster in a division
// @Description	Closes the roster entry rather than deleting it, so match history keeps the player's team. The team captain cannot be removed.
// @Tags			rosters
// @Produce		json
// @Param			id			path	string	true	"team id (uuid)"
// @Param			player_id	path	string	true	"player id (uuid)"
// @Param			division_id	query	string	true	"division id (uuid)"
// @Success		204			"removed"
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND (team or roster entry)"
// @Failure		409			{object}	handler.ErrorResponse	"40901 CONFLICT (captain)"
// @Security		CSRFToken
// @Failure		403	{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED (not the team captain, or an unpaid member)"
// @Router			/teams-roster/{id}/{player_id} [delete]
func (h *Handler) RemoveRosterPlayer(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}
	playerID, err := handler.ParsePathUUID(r, "player_id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	actorUserID, err := h.actorUserID(r)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	err = h.service.RemoveRosterPlayer(r.Context(), actorUserID, RosterEntryServiceParams{
		DivisionID: r.URL.Query().Get("division_id"),
		TeamID:     teamID,
		PlayerID:   playerID,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}

// actorUserID is the signed-in user RequireSession put in the context; the roster rules
// depend on who is asking.
func (h *Handler) actorUserID(r *http.Request) (string, error) {
	userID, ok := h.ExtractUserIDFromContext(r.Context())
	if !ok {
		return "", handler.Error(handler.CodeNoUserInContext, errors.New("teams: no user id in context"))
	}
	return userID, nil
}
