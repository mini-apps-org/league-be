package router

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nguyentanvinh/league-be/internal/auth"
	"github.com/nguyentanvinh/league-be/internal/permissions"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

const (
	PrefixAPIV1       = "/api/v1"
	PathSwagger       = "/swagger"
	PathSwaggerPrefix = "/swagger/"
	PathSwaggerUI     = "/swagger/index.html"
)

type Router struct {
	Config *config.Config
	Pool   *pgxpool.Pool
}

func (r *Router) NewRouter(ctx context.Context) (*mux.Router, error) {
	authHandler, err := r.newAuthHandler(ctx)
	if err != nil {
		return nil, err
	}

	permissionService := permissions.NewService(permissions.NewRepo(r.Pool))

	mw := middleware.New(r.Config, auth.CSRFCookieName(r.Config), authHandler, permissionService)

	router := mux.NewRouter()
	router.Use(mw.SecurityHeaders)

	if r.Config.IsProduction() {
		router.Use(mw.HSTS)
	}

	handleSwagger(router)

	api := router.PathPrefix(PrefixAPIV1).Subrouter()

	api.Use(mw.RequireCSRF)

	r.registerHealthRoute(api)
	registerAuthRoute(api, authHandler, mw)
	r.registerRoleRoute(api, mw, permissionService)

	return router, nil
}

func handleSwagger(router *mux.Router) {
	router.HandleFunc(PathSwagger, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, PathSwaggerUI, http.StatusMovedPermanently)
	}).Methods(http.MethodGet)

	router.PathPrefix(PathSwaggerPrefix).Handler(httpSwagger.Handler(
		httpSwagger.URL("doc.json"),
	)).Methods(http.MethodGet)
}
