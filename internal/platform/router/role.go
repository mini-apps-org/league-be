package router

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/nguyentanvinh/league-be/internal/permissions"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
	"github.com/nguyentanvinh/league-be/internal/roles"
)

func (r *Router) registerRoleRoute(api *mux.Router, mw middleware.Middleware, permissionService permissions.Service) {
	permissionHandler := permissions.NewHandler(r.Config, permissionService)

	roleRepo := roles.NewRepo(r.Pool)
	roleService := roles.NewService(roleRepo)
	roleHandler := roles.NewHandler(r.Config, roleService)

	guard := func(feature config.PermissionFeature, next http.HandlerFunc) http.Handler {
		return mw.RequireSession(mw.RequirePermission(feature, next))
	}

	api.Handle("/roles", guard(config.PermissionFeatureRoles, roleHandler.ListRoles)).Methods(http.MethodGet)
	api.Handle("/roles", guard(config.PermissionFeatureRoles, roleHandler.CreateRole)).Methods(http.MethodPost)
	api.Handle("/roles/{id}", guard(config.PermissionFeatureRoles, roleHandler.GetRole)).Methods(http.MethodGet)
	api.Handle("/roles/{id}", guard(config.PermissionFeatureRoles, roleHandler.UpdateRole)).Methods(http.MethodPatch)
	api.Handle("/roles/{id}", guard(config.PermissionFeatureRoles, roleHandler.DeleteRole)).Methods(http.MethodDelete)

	// TODO vincent: Will be remove to another folder
	api.Handle("/permissions", guard(config.PermissionFeaturePermissions, permissionHandler.ListPermissions)).Methods(http.MethodGet)
	api.Handle("/permissions/{id}", guard(config.PermissionFeaturePermissions, permissionHandler.GetPermissionByID)).Methods(http.MethodGet)
}
