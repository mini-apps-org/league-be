package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

func TestTrace(t *testing.T) {
	tests := []struct {
		name       string
		incoming   string
		wantKeptID bool
	}{
		{name: "missing header mints one", incoming: "", wantKeptID: false},
		{name: "client header is kept", incoming: "trace-abc-123", wantKeptID: true},
		{name: "oversized header is replaced", incoming: strings.Repeat("a", maxRequestIDLength+1), wantKeptID: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var contextRequestID string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contextRequestID, _ = r.Context().Value(config.ContextKeyRequestID).(string)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.incoming != "" {
				request.Header.Set(config.HeaderRequestID, test.incoming)
			}
			recorder := httptest.NewRecorder()

			(&middleware{}).Trace(next).ServeHTTP(recorder, request)

			responseRequestID := recorder.Header().Get(config.HeaderRequestID)
			if responseRequestID != contextRequestID {
				t.Fatalf("response header %q differs from context value %q", responseRequestID, contextRequestID)
			}
			if test.wantKeptID && responseRequestID != test.incoming {
				t.Fatalf("got %q, want client's %q kept", responseRequestID, test.incoming)
			}
			if !test.wantKeptID {
				if _, err := uuid.Parse(responseRequestID); err != nil {
					t.Fatalf("got %q, want a minted UUID: %v", responseRequestID, err)
				}
			}
		})
	}
}
