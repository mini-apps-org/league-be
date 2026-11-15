package permissions

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

// @Summary		List permissions
// @Description	The catalog is a fixed vocabulary, returned whole. It is read-only over HTTP: codes are compared by authorization code elsewhere in the backend, so they are added and removed by migration.
// @Tags			permissions
// @Produce		json
// @Success		200	{object}	config.SuccessResponse{data=[]permissions.PermissionResponse}
// @Failure		401	{object}	handler.ErrorResponse	"40101 SESSION_COOKIE_ABSENT | 40102 SESSION_INVALID"
// @Failure		403	{object}	handler.ErrorResponse	"40305 PERMISSION_DENIED"
// @Router			/permissions [get]
func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	permissions, err := h.service.ListPermissions(r.Context())
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	items := make([]PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		items = append(items, newPermissionResponse(permission))
	}

	total := int64(len(items))
	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Count: &total, Data: items})
}

// @Summary	Get a permission
// @Tags		permissions
// @Produce	json
// @Param		id	path		string	true	"permission id (uuid)"
// @Success	200	{object}	config.SuccessResponse{data=permissions.PermissionResponse}
// @Failure	400	{object}	handler.ErrorResponse	"40001 VALIDATION_FAILED"
// @Failure	404	{object}	handler.ErrorResponse	"40401 NOT_FOUND"
// @Router		/permissions/{id} [get]
func (h *Handler) GetPermissionByID(w http.ResponseWriter, r *http.Request) {
	id, err := handler.ParsePathUUID(r, "id")
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	permission, err := h.service.GetPermissionByID(r.Context(), id)
	if err != nil {
		h.ReturnWithError(w, r, err)
		return
	}

	h.ReturnWithJSON(w, http.StatusOK, config.SuccessResponse{Data: newPermissionResponse(*permission)})
}
