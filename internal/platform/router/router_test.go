package router

import (
	"strings"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/auth"
)

// TestOAuthTxCookiePathCoversRoutes is the assertion auth.OAuthTxCookiePath's
// comment promises. It lives here rather than in the auth package because
// router imports auth — referencing PrefixAPIV1 from there is an import cycle,
// and this package is the one that owns the mounted paths.
//
// If the cookie's Path stops covering the callback route the browser withholds
// the cookie, CompleteLogin fails with "no oauth_tx cookie", and login is
// impossible — with no compile error and nothing in the server logs but a
// generic login_failed redirect.
func TestOAuthTxCookiePathCoversRoutes(t *testing.T) {
	// Mirrors registerAuthRoute: api.PathPrefix(PrefixAPIV1) -> "/auth/google" -> leaf.
	const authGooglePrefix = "/auth/google"
	routes := map[string]string{
		"login":    PrefixAPIV1 + authGooglePrefix + "/login",
		"callback": PrefixAPIV1 + authGooglePrefix + "/callback",
	}

	for name, route := range routes {
		if !strings.HasPrefix(route, auth.OAuthTxCookiePath) {
			t.Errorf("%s route %q is not covered by OAuthTxCookiePath %q; the browser would withhold the cookie",
				name, route, auth.OAuthTxCookiePath)
			continue
		}
		t.Logf("  %-8s %-34s covered by %q", name, route, auth.OAuthTxCookiePath)
	}
}

// The cookie path must also stay inside the versioned API prefix, or it would
// scope the cookie wider than the routes that need it.
func TestOAuthTxCookiePathStaysWithinAPIPrefix(t *testing.T) {
	if !strings.HasPrefix(auth.OAuthTxCookiePath, PrefixAPIV1) {
		t.Errorf("OAuthTxCookiePath %q escapes the API prefix %q",
			auth.OAuthTxCookiePath, PrefixAPIV1)
	}
}
