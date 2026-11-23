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
// @Success		200	{object}	handler.SuccessResponse[[]permissions.PermissionResponse]
// @Failure		401	{object}	handler.ErrorResponse	"401001 SESSION_COOKIE_ABSENT | 401002 SESSION_INVALID"
// @Failure		403	{object}	handler.ErrorResponse	"403005 PERMISSION_DENIED"
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

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[[]PermissionResponse]{
		Data:     items,
		Metadata: handler.NewMetadata(len(items)),
	})
}

// @Summary	Get a permission
// @Tags		permissions
// @Produce	json
// @Param		id	path		string	true	"permission id (uuid)"
// @Success	200	{object}	handler.SuccessResponse[permissions.PermissionResponse]
// @Failure	400	{object}	handler.ErrorResponse	"400001 VALIDATION_FAILED"
// @Failure	404	{object}	handler.ErrorResponse	"404001 NOT_FOUND"
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

	h.ReturnWithJSON(w, http.StatusOK, handler.SuccessResponse[PermissionResponse]{Data: newPermissionResponse(*permission)})
}
