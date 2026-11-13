package middleware

import (
	"context"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// sessionAuthenticator is what RequireSession needs, stated as an interface so
// this package depends on three methods rather than the whole auth handler —
// and so a test can supply a fake without a database or a Google round trip.
// AuthenticateSession returns the request context already carrying the user,
// which keeps this package free of any import of auth.
type sessionAuthenticator interface {
	GetSessionCookie(r *http.Request) string
	ClearSessionCookie(w http.ResponseWriter)
	AuthenticateSession(ctx context.Context, sessionID string) (context.Context, error)
}

// Middleware holds every dependency the HTTP middlewares need, so the router
// builds it once and each middleware is a method with the mux.MiddlewareFunc
// signature.
//
// csrfCookieName is injected, not imported from auth: this package sits below
// the feature packages.
type Middleware struct {
	handler.Handler
	csrfCookieName string
	sessions       sessionAuthenticator
}

func New(cfg *config.Config, csrfCookieName string, sessions sessionAuthenticator) *Middleware {
	return &Middleware{
		Handler:        handler.Handler{Config: cfg},
		csrfCookieName: csrfCookieName,
		sessions:       sessions,
	}
}
