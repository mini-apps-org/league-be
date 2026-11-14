package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

func (m *middleware) RequirePermission(feature config.PermissionFeature, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A routing mistake; unchecked, it would require "role." and deny everyone.
		permissionAction, ok := config.HTTPMethodToPermissionAction[r.Method]
		if !ok {
			m.ReturnWithError(w, r, handler.Error(handler.CodeInternal, fmt.Errorf("permission: no action for method %s", r.Method)))
			return
		}
		permissionCode := config.GetPermissionByFeatureAndAction(feature, permissionAction)

		ctx := r.Context()

		userID, ok := m.ExtractUserIDFromContext(ctx)
		if !ok {
			m.ReturnWithError(w, r, handler.Error(handler.CodeNoUserInContext, errors.New("permission: no user id in context")))
			return
		}

		permissionCodes, err := m.permissionService.GetPermissionCodesByUserID(ctx, userID)

		if err != nil {
			m.ReturnWithError(w, r, handler.Error(handler.CodeInternal, err))
			return
		}

		if !slices.Contains(permissionCodes, permissionCode) {
			m.ReturnWithError(w, r, handler.Error(handler.CodePermissionDenied, fmt.Errorf("permission: user %s lacks %s", userID, permissionCode)))
			return
		}

		next.ServeHTTP(w, r)
	})
}
