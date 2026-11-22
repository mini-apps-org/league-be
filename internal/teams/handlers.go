package teams

import (
	"errors"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

type Handler struct {
	handler.Handler
	service Service
}

func NewHandler(cfg *config.Config, service Service) Handler {
	return Handler{Handler: handler.Handler{Config: cfg}, service: service}
}

// @Summary	List teams
// @Tags		teams
// @Produce	json
// @Param		division_id	query		string	false	"only teams entered in this division (uuid)"
// @Success	200			{object}	config.SuccessResponse{data=[]teams.TeamResponse}
// @Failure	400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure	401			{object}	handler.ErrorResponse	"40101 SESSION_COOKIE_ABSENT | 40102 SESSION_INVALID"
// @Failure	403			{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED"
// @Router		/teams [get]
func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.service.ListTeams(r.Context(), ListTeamsServiceParams{
		DivisionID: r.URL.Query().Get("division_id"),
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]TeamResponse, 0, len(teams))
	for _, team := range teams {
		items = append(items, newTeamResponse(team))
	}

	total := int64(len(items))
	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Count: &total, Data: items})
}

// @Summary		Create a team
// @Description	`division_id` is required: a team is created in a division. `team_code` is the team code XXyyA (region, host location, team sequence), unique within the division; lower case is accepted and stored upper case. The team then enters a division with POST /teams/{id}/divisions, and the captain is set with PATCH once the player is on the roster.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			body	body		teams.CreateTeamRequest	true	"team to create"
// @Success		201		{object}	config.SuccessResponse{data=teams.TeamResponse}
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

	h.ReturnWithJSON(w, http.StatusCreated, config.SuccessResponse{Data: newTeamResponse(*team)})
}

// @Summary	Get a team
// @Tags		teams
// @Produce	json
// @Param		id	path		string	true	"team id (uuid)"
// @Success	200	{object}	config.SuccessResponse{data=teams.TeamResponse}
// @Failure	400	{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure	404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Router		/teams/{id} [get]
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	team, err := h.service.GetTeam(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newTeamResponse(*team)})
}

// @Summary		Update a team
// @Description	Partial update: an absent field is left unchanged. `captain_player_id` must be on the team's active roster in some division.
// @Tags			teams
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"team id (uuid)"
// @Param			body	body		teams.UpdateTeamRequest	true	"fields to change"
// @Success		200		{object}	config.SuccessResponse{data=teams.TeamResponse}
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

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newTeamResponse(*team)})
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
func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.DeleteTeam(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
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
func (h *Handler) HardDeleteTeam(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.HardDeleteTeam(r.Context(), id); err != nil {
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
// @Success		201		{object}	config.SuccessResponse{data=teams.DivisionTeamResponse}
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

	h.ReturnWithJSON(w, http.StatusCreated, config.SuccessResponse{Data: newDivisionTeamResponse(*entry)})
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
// @Description	Active players only, with their stats in the division (zero before the first match).
// @Tags			rosters
// @Produce		json
// @Param			id			path		string	true	"team id (uuid)"
// @Param			division_id	query		string	true	"division id (uuid)"
// @Success		200			{object}	config.SuccessResponse{data=[]teams.RosterPlayerResponse}
// @Failure		400			{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404			{object}	handler.ErrorResponse	"40401 NOT_FOUND (team has not entered the division)"
// @Router			/teams-roster/{id} [get]
func (h *Handler) ListRosterPlayers(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	players, err := h.service.ListRosterPlayers(r.Context(), r.URL.Query().Get("division_id"), teamID)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]RosterPlayerResponse, 0, len(players))
	for _, player := range players {
		items = append(items, newRosterPlayerResponse(player))
	}

	total := int64(len(items))
	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Count: &total, Data: items})
}

// @Summary		Add a player to a team's roster in a division
// @Description	A player can be on only one active roster per division, and the team must have entered the division.
// @Tags			rosters
// @Accept			json
// @Produce		json
// @Param			id			path		string							true	"team id (uuid)"
// @Param			division_id	query		string							true	"division id (uuid)"
// @Param			body		body		teams.AddRosterPlayerRequest	true	"player to add"
// @Success		201			{object}	config.SuccessResponse{data=teams.RosterPlayerResponse}
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

	h.ReturnWithJSON(w, http.StatusCreated, config.SuccessResponse{Data: newRosterPlayerResponse(*player)})
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
// @Success		200			{object}	config.SuccessResponse{data=teams.RosterPlayerResponse}
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

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newRosterPlayerResponse(*player)})
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
