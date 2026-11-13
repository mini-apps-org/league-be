package router

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nguyentanvinh/league-be/internal/auth"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
	"github.com/nguyentanvinh/league-be/internal/secret"
)

func (r *Router) newAuthHandler(ctx context.Context) (*auth.Handler, error) {
	googleAuth, err := auth.NewGoogleAuth(ctx, *r.Config)
	if err != nil {
		return nil, fmt.Errorf("router: google auth: %w", err)
	}

	sealer, err := secret.NewBox(r.Config.Auth.EncryptionKey, r.Config.Auth.EncryptionKeyVersion)
	if err != nil {
		return nil, fmt.Errorf("router: encryption key: %w", err)
	}

	authRepo := auth.NewRepo(r.Pool)

	authService := auth.NewService(r.Config, googleAuth, authRepo, sealer)
	authHandler := auth.NewHandler(r.Config, authService)

	authService.StartSessionCleanup(ctx)

	return &authHandler, nil
}

func registerAuthRoute(api *mux.Router, authHandler *auth.Handler, mw *middleware.Middleware) {
	authRouter := api.PathPrefix("/auth/google").Subrouter()
	authRouter.HandleFunc("/login", authHandler.Login).Methods(http.MethodGet)
	authRouter.HandleFunc("/callback", authHandler.CallbackLogin).Methods(http.MethodGet)

	api.Handle("/auth/logout", http.HandlerFunc(authHandler.Logout)).Methods(http.MethodPost)
	api.Handle("/me",
		mw.RequireSession(http.HandlerFunc(authHandler.Me)),
	).Methods(http.MethodGet)
}
