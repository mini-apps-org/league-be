package config

type ContextKey string

const (
	ContextKeyRequestID   ContextKey = "request_id"
	ContextKeyUserID      ContextKey = "user_id"
	ContextKeyTransaction ContextKey = "transaction"
)

const HeaderRequestID = "X-Request-Id"

var AllowHeadersAPI = []string{"Accept", "Content-Type", "Origin", "X-CSRF-Token", HeaderRequestID}

var ExposeHeadersAPI = []string{HeaderRequestID}
