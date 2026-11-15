package middleware

import (
	"context"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

type sessionHandler interface {
	GetSessionCookie(r *http.Request) string
	ClearSessionCookie(w http.ResponseWriter)
	AuthenticateSession(ctx context.Context, sessionID string) (context.Context, error)
}

type permissionCodeReader interface {
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type Middleware interface {
	SecurityHeaders(next http.Handler) http.Handler
	HSTS(next http.Handler) http.Handler
	RequireCSRF(next http.Handler) http.Handler
	RequireSession(next http.Handler) http.Handler
	RequirePermission(feature config.PermissionFeature, next http.Handler) http.Handler
}

type middleware struct {
	handler.Handler
	csrfCookieName    string
	sessionHandler    sessionHandler
	permissionService permissionCodeReader
}

func New(cfg *config.Config, csrfCookieName string, sessionHandler sessionHandler, permissionService permissionCodeReader) Middleware {
	return &middleware{
		Handler:           handler.Handler{Config: cfg},
		csrfCookieName:    csrfCookieName,
		sessionHandler:    sessionHandler,
		permissionService: permissionService,
	}
}
