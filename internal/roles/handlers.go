package roles

import (
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

// @Summary	List roles
// @Tags		roles
// @Produce	json
// @Success	200	{object}	config.SuccessResponse{data=[]roles.RoleResponse}
// @Failure	401	{object}	handler.ErrorResponse	"40101 SESSION_COOKIE_ABSENT | 40102 SESSION_INVALID"
// @Failure	403	{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED"
// @Router		/roles [get]
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]RoleResponse, 0, len(roles))
	for _, role := range roles {
		items = append(items, newRoleResponse(role))
	}

	total := int64(len(items))
	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Count: &total, Data: items})
}

// @Summary		Create a role
// @Description	`code` is the authorization contract and cannot be changed afterwards.
// @Tags			roles
// @Accept			json
// @Produce		json
// @Param			body	body		roles.CreateRoleRequest	true	"role to create"
// @Success		201		{object}	config.SuccessResponse{data=roles.RoleResponse}
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		403		{object}	handler.ErrorResponse	"40301 CSRF_COOKIE_ABSENT | 40302 CSRF_TOKEN_MISMATCH | 40305 PERMISSION_DENIED"
// @Failure		409		{object}	handler.ErrorResponse	"40901 CONFLICT"
// @Security		CSRFToken
// @Router			/roles [post]
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var body CreateRoleRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	role, err := h.service.CreateRole(r.Context(), CreateRoleServiceParams{
		Code:        RoleCode(body.Code),
		Name:        body.Name,
		Description: body.Description,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusCreated, config.SuccessResponse{Data: newRoleResponse(*role)})
}

// @Summary	Get a role
// @Tags		roles
// @Produce	json
// @Param		id	path		string	true	"role id (uuid)"
// @Success	200	{object}	config.SuccessResponse{data=roles.RoleResponse}
// @Failure	400	{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure	404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Router		/roles/{id} [get]
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	role, err := h.service.GetRole(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newRoleResponse(*role)})
}

// @Summary		Update a role
// @Description	Partial update: an absent field is left unchanged. `code` cannot be changed — create a new role instead.
// @Tags			roles
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"role id (uuid)"
// @Param			body	body		roles.UpdateRoleRequest	true	"fields to change"
// @Success		200		{object}	config.SuccessResponse{data=roles.RoleResponse}
// @Failure		400		{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure		404		{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Security		CSRFToken
// @Router			/roles/{id} [patch]
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	var body UpdateRoleRequest
	if err := h.DecodeJSON(w, r, &body); err != nil {
		h.ReturnWithError(w, r, handler.InvalidJSONBodyError(err))
		return
	}

	role, err := h.service.UpdateRole(r.Context(), id, UpdateRoleServiceParams{ //nolint:staticcheck // S1016: explicit mapping keeps the JSON and service types independent
		Name:        body.Name,
		Description: body.Description,
	})
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newRoleResponse(*role)})
}

// @Summary		Delete a role
// @Description	Cascades: rows in roles_permissions and users_roles go with it, so everyone holding this role loses what it granted. SUPER_ADMIN cannot be deleted.
// @Tags			roles
// @Produce		json
// @Param			id	path	string	true	"role id (uuid)"
// @Success		204	"deleted"
// @Failure		404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Failure		409	{object}	handler.ErrorResponse	"40901 CONFLICT (SUPER_ADMIN)"
// @Security		CSRFToken
// @Router			/roles/{id} [delete]
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	if err := h.service.DeleteRole(r.Context(), id); err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusNoContent, nil)
}
