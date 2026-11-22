package router

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

// passThroughMiddleware lets every request through, so a test sees which handler a route
// reaches rather than the session and permission checks in front of it.
type passThroughMiddleware struct{}

func (passThroughMiddleware) Trace(next http.Handler) http.Handler                { return next }
func (passThroughMiddleware) RecordRequestMetrics(next http.Handler) http.Handler { return next }
func (passThroughMiddleware) SecurityHeaders(next http.Handler) http.Handler      { return next }
func (passThroughMiddleware) HSTS(next http.Handler) http.Handler                 { return next }
func (passThroughMiddleware) RequireCSRF(next http.Handler) http.Handler          { return next }
func (passThroughMiddleware) RequireSession(next http.Handler) http.Handler       { return next }
func (passThroughMiddleware) RequirePermission(_ config.PermissionFeature, next http.Handler) http.Handler {
	return next
}

// mux tries routes in registration order. Registered after /teams/{id},
// DELETE /teams/permanent would reach the single delete with id "permanent" — it compiles,
// and the bulk endpoint silently answers "id must be a uuid". An empty ids list tells the
// two apart: only the bulk handler says "ids must hold".
func TestBulkTeamDeleteRoutesReachBulkHandlers(t *testing.T) {
	router := &Router{Config: &config.Config{}, Logger: slog.New(slog.DiscardHandler)}
	api := mux.NewRouter().PathPrefix(PrefixAPIV1).Subrouter()
	router.registerTeamRoute(api, passThroughMiddleware{}, nil)

	for _, path := range []string{"/teams", "/teams/permanent"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodDelete, PrefixAPIV1+path, strings.NewReader(`{"ids": []}`))
			recorder := httptest.NewRecorder()
			api.ServeHTTP(recorder, request)

			if body := recorder.Body.String(); !strings.Contains(body, "ids must hold at least one team id") {
				t.Errorf("DELETE %s reached the wrong handler: %d %s", path, recorder.Code, body)
			}
		})
	}
}
