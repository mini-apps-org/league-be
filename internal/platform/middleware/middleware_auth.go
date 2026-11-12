package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/auth"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// sessionAuthenticator is what AuthenticationWithSession needs, stated as an interface so
// this package depends on three methods rather than the whole auth handler —
// and so a test can supply a fake without a database or a Google round trip.
type sessionAuthenticator interface {
	GetSessionCookie(r *http.Request) string
	ClearSessionCookie(w http.ResponseWriter)
	SessionUser(ctx context.Context, sessionID string) (*auth.User, error)
}

type Auth struct {
	handler.Handler
	sessions sessionAuthenticator
}

func NewAuthentication(cfg *config.Config, sessions sessionAuthenticator) *Auth {
	return &Auth{Handler: handler.Handler{Config: cfg}, sessions: sessions}
}

func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := a.sessions.GetSessionCookie(r)
		if sessionID == "" {
			a.ReturnWithError(w, r, handler.Error(handler.CodeNoSession, errors.New("no session cookie")))
			return
		}

		user, err := a.sessions.SessionUser(r.Context(), sessionID)
		if err != nil {
			a.sessions.ClearSessionCookie(w)
			a.ReturnWithError(w, r, handler.Error(handler.CodeSessionInvalid, err))
			return
		}

		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), user)))
	})
}
