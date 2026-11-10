package common

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// RandomToken mints a session id, state or nonce: 32 CSPRNG bytes, base64url so
// it is safe in a cookie and a URL.
func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
