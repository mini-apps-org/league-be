package router

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
	"github.com/nguyentanvinh/league-be/internal/teams"
)

func (r *Router) registerTeamRoute(api *mux.Router, mw middleware.Middleware, permissionReader teams.PermissionReader) {
	teamRepo := teams.NewRepo(r.Pool, r.Logger)
	teamService := teams.NewService(teamRepo, permissionReader, r.Logger)
	teamHandler := teams.NewHandler(r.Config, teamService)

	guard := func(feature config.PermissionFeature, next http.HandlerFunc) http.Handler {
		return mw.RequireSession(mw.RequirePermission(feature, next))
	}

	api.Handle("/teams", guard(config.PermissionFeatureTeams, teamHandler.ListTeams)).Methods(http.MethodGet)
	api.Handle("/teams", guard(config.PermissionFeatureTeams, teamHandler.CreateTeam)).Methods(http.MethodPost)
	api.Handle("/teams/{id}", guard(config.PermissionFeatureTeams, teamHandler.GetTeamByID)).Methods(http.MethodGet)
	api.Handle("/teams/{id}", guard(config.PermissionFeatureTeams, teamHandler.UpdateTeam)).Methods(http.MethodPatch)
	api.Handle("/teams/{id}", guard(config.PermissionFeatureTeams, teamHandler.DeleteTeamByID)).Methods(http.MethodDelete)
	api.Handle("/teams/{id}/permanent", guard(config.PermissionFeatureTeams, teamHandler.PermanentDeleteTeam)).Methods(http.MethodDelete)
	api.Handle("/teams/{id}/divisions", guard(config.PermissionFeatureTeams, teamHandler.AddDivisionTeam)).Methods(http.MethodPost)
	api.Handle("/teams/{id}/divisions/{division_id}", guard(config.PermissionFeatureTeams, teamHandler.RemoveDivisionTeam)).Methods(http.MethodDelete)

	// TODO vincent: Will move another file later
	api.Handle("/teams-roster/{id}", guard(config.PermissionFeatureTeamRosters, teamHandler.ListRosterPlayers)).Methods(http.MethodGet)
	api.Handle("/teams-roster/{id}", guard(config.PermissionFeatureTeamRosters, teamHandler.AddRosterPlayer)).Methods(http.MethodPost)
	api.Handle("/teams-roster/{id}/{player_id}", guard(config.PermissionFeatureTeamRosters, teamHandler.TransferRosterPlayer)).Methods(http.MethodPatch)
	api.Handle("/teams-roster/{id}/{player_id}", guard(config.PermissionFeatureTeamRosters, teamHandler.RemoveRosterPlayer)).Methods(http.MethodDelete)
}
