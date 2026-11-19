package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordRequestMetrics(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		status    int
		wantRoute string
		wantCode  string
	}{
		{name: "subrouter route is labelled by template", path: "/api/v1/roles/123", status: http.StatusForbidden, wantRoute: "/api/v1/roles/{id}", wantCode: "403"},
		{name: "handler that never writes a status counts as 200", path: "/api/v1/me", status: 0, wantRoute: "/api/v1/me", wantCode: "200"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.status != 0 {
					w.WriteHeader(test.status)
				}
			})

			router := mux.NewRouter()
			router.Use((&middleware{}).RecordRequestMetrics)
			api := router.PathPrefix("/api/v1").Subrouter()
			api.Handle("/roles/{id}", next).Methods(http.MethodGet)
			api.Handle("/me", next).Methods(http.MethodGet)

			counter := httpRequestCounter.WithLabelValues(test.wantRoute, "get", test.wantCode)
			countBefore := testutil.ToFloat64(counter)

			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, test.path, nil))

			if got := testutil.ToFloat64(counter) - countBefore; got != 1 {
				t.Fatalf("http_requests_total{route=%q,code=%q} rose by %v, want 1", test.wantRoute, test.wantCode, got)
			}
			if got := testutil.CollectAndCount(httpRequestDurationHistogram, "http_request_duration_seconds"); got == 0 {
				t.Fatal("http_request_duration_seconds recorded no series")
			}
		})
	}
}
