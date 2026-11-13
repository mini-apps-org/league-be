package middleware

import (
	"errors"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

func (m *Middleware) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := m.sessions.GetSessionCookie(r)
		if sessionID == "" {
			m.ReturnWithError(w, r, handler.Error(handler.CodeNoSession, errors.New("no session cookie")))
			return
		}

		ctx, err := m.sessions.AuthenticateSession(r.Context(), sessionID)
		if err != nil {
			m.sessions.ClearSessionCookie(w)
			m.ReturnWithError(w, r, handler.Error(handler.CodeSessionInvalid, err))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
