package config

type SuccessResponse struct {
	SessionID string `json:"session_id"`
	Count     *int64 `json:"count"`
	Data      any    `json:"data"`
	Metadata  any    `json:"metadata,omitempty"`
}

type ContextKey string

const (
	ContextKeyRequestID ContextKey = "request_id"
	ContextKeyUserID    ContextKey = "user_id"
)

const HeaderRequestID = "X-Request-Id"

var AllowHeadersAPI = []string{"Accept", "Content-Type", "Origin", "X-CSRF-Token", HeaderRequestID}

var ExposeHeadersAPI = []string{HeaderRequestID}
