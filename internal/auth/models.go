package auth

import (
	"context"
	"time"
)

type User struct {
	ID            string    `db:"id"`
	Email         string    `db:"email"`
	EmailVerified bool      `db:"email_verified"`
	Name          string    `db:"name"`
	AvatarURL     string    `db:"avatar_url"`
	CreatedAt     time.Time `db:"created_at"`
	LastLoginAt   time.Time `db:"last_login_at"`
}

// The authenticated user travels in the request context. The key and both
// accessors live here, beside User, rather than in platform/middleware: platform
// never imports a feature, so middleware only receives the enriched context.
type userKey struct{}

// WithUser is called by Handler.AuthenticateSession once a session resolves.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

func ExtractUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey{}).(*User)
	return user, ok
}

// GoogleIdentityParams carries ID token claims into the upsert. Every field
// comes from a JWKS-verified token, never from the frontend.
type GoogleIdentityParams struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
	HD            string
}

type Session struct {
	ID         string    `db:"id"`
	UserID     string    `db:"user_id"`
	CreatedAt  time.Time `db:"created_at"`
	LastSeenAt time.Time `db:"last_seen_at"`
}

// LoginTransaction is one in-flight Google login, sealed into the oauth_tx
// cookie. Short json tags keep the ciphertext small.
type LoginTransaction struct {
	State        string    `json:"s"`
	Nonce        string    `json:"n"`
	CodeVerifier string    `json:"v"`
	ExpiresAt    time.Time `json:"e"`
}
