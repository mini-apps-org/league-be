package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

// maxRequestIDLength bounds a client-sent ID, which is copied into every log
// line of the request.
const maxRequestIDLength = 128

// Trace keeps the client's X-Request-Id, or mints a UUID when it is missing or
// too long, then echoes it on the response and puts it on the context for logging.
func (m *middleware) Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(config.HeaderRequestID)
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = uuid.NewString()
		}

		w.Header().Set(config.HeaderRequestID, requestID)
		ctx := context.WithValue(r.Context(), config.ContextKeyRequestID, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
