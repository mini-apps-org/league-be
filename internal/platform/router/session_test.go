package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
	"github.com/nguyentanvinh/league-be/internal/platform/middleware"
)

type signedInSession struct{}

func (signedInSession) GetSessionCookie(*http.Request) string  { return "test-sid" }
func (signedInSession) ClearSessionCookie(http.ResponseWriter) {}
func (signedInSession) AuthenticateSession(ctx context.Context, _ string) (context.Context, error) {
	return handler.ContextWithUserID(ctx, "test-user"), nil
}

type grantedPermissions []string

func (p grantedPermissions) GetPermissionCodesByUserID(context.Context, string) ([]string, error) {
	return p, nil
}

// Each route must ask for the session.* code of its own method. A route guarded by the
// wrong feature or method would let a read-only user create or delete sessions, or lock
// the operator out. Every request here is malformed so that, once past the permission
// check, the handler answers 400 before reaching the database the test does not have.
func TestLeagueSessionRoutesRequirePermissionForMethod(t *testing.T) {
	routes := []struct {
		method, path, body, permission string
	}{
		{http.MethodGet, "/sessions?sort=bad", "", "session.read"},
		{http.MethodPost, "/sessions", "{", "session.create"},
		{http.MethodGet, "/sessions/not-a-uuid", "", "session.read"},
		{http.MethodPatch, "/sessions/not-a-uuid", "{}", "session.update"},
		{http.MethodDelete, "/sessions/not-a-uuid", "", "session.delete"},
		{http.MethodGet, "/sessions/not-a-uuid/weeks", "", "session.read"},
	}
	allSessionCodes := []string{"session.read", "session.create", "session.update", "session.delete"}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			others := []string{}
			for _, code := range allSessionCodes {
				if code != route.permission {
					others = append(others, code)
				}
			}

			if status, code := serveLeagueSessionRoute(t, grantedPermissions{route.permission}, route.method, route.path, route.body); status != http.StatusBadRequest {
				t.Errorf("with %s: got %d/%d, want the handler's 400", route.permission, status, code)
			}
			if status, code := serveLeagueSessionRoute(t, grantedPermissions(others), route.method, route.path, route.body); code != handler.CodePermissionDenied {
				t.Errorf("without %s: got %d/%d, want 403/%d", route.permission, status, code, handler.CodePermissionDenied)
			}
		})
	}
}

func serveLeagueSessionRoute(t *testing.T, permissions grantedPermissions, method, path, body string) (int, handler.ErrorCode) {
	t.Helper()

	router := &Router{Config: &config.Config{}, Logger: slog.New(slog.DiscardHandler)}
	mw := middleware.New(router.Config, "csrf", signedInSession{}, permissions)
	api := mux.NewRouter().PathPrefix(PrefixAPIV1).Subrouter()
	router.registerLeagueSessionRoute(api, mw)

	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, httptest.NewRequest(method, PrefixAPIV1+path, strings.NewReader(body)))

	var response handler.ErrorResponse
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	return recorder.Code, response.Error.Code
}
