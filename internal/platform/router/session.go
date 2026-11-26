package router

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
	"github.com/nguyentanvinh/league-be/internal/sessions"
)

func (r *Router) registerLeagueSessionRoute(api *mux.Router, mw middleware.Middleware) {
	sessionRepo := sessions.NewRepo(r.Pool, r.Logger)
	sessionService := sessions.NewService(sessionRepo, r.Logger)
	sessionHandler := sessions.NewHandler(r.Config, sessionService)

	guard := func(feature config.PermissionFeature, next http.HandlerFunc) http.Handler {
		return mw.RequireSession(mw.RequirePermission(feature, next))
	}

	api.Handle("/sessions", guard(config.PermissionFeatureSessions, sessionHandler.ListSessions)).Methods(http.MethodGet)
	api.Handle("/sessions", guard(config.PermissionFeatureSessions, sessionHandler.CreateSession)).Methods(http.MethodPost)
	api.Handle("/sessions/{id}", guard(config.PermissionFeatureSessions, sessionHandler.GetSessionByID)).Methods(http.MethodGet)
	api.Handle("/sessions/{id}", guard(config.PermissionFeatureSessions, sessionHandler.UpdateSessionByID)).Methods(http.MethodPatch)
	api.Handle("/sessions/{id}", guard(config.PermissionFeatureSessions, sessionHandler.DeleteSessionByID)).Methods(http.MethodDelete)
	api.Handle("/sessions/{id}/permanent", guard(config.PermissionFeatureSessions, sessionHandler.PermanentDeleteSession)).Methods(http.MethodDelete)
	api.Handle("/sessions/{id}/weeks", guard(config.PermissionFeatureSessions, sessionHandler.ListSessionWeeks)).Methods(http.MethodGet)
}
