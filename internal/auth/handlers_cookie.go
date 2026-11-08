package auth

import (
	"net/http"
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

const (
	sessionCookie = "sid"      // server-side session id
	oauthTxCookie = "oauth_tx" // binds a login transaction to this browser
	csrfCookie    = "csrf"     // double-submit token; readable by JS on purpose

	// OAuthTxCookiePath scopes the oauth_tx cookie so nothing outside the Google
	// login routes can read it. It MUST stay a prefix of the mounted callback path:
	// the browser withholds the cookie on a path mismatch, and the callback then
	// fails with "no oauth_tx cookie" — a silent, total login outage with no
	// compile error. Asserted in the router package, which owns the real routes.
	OAuthTxCookiePath = "/api/v1/auth/google"

	defaultCookiePath = "/"
)

// cookieParams is the per-cookie input to setCookie. There is deliberately no
// path field: Path is derived from the name, by the same function the prefix
// decision uses, so the two cannot contradict each other. A cookie named for one
// path but written at another is invisible to the compiler and costs a login.
type cookieParams struct {
	name     string
	value    string
	ttl      time.Duration
	httpOnly bool
}

// isSecure gates the Secure attribute, which both cookie prefixes require.
func (h *Handler) isSecure() bool {
	return h.Config.IsProduction()
}

// getCookieName resolves the on-the-wire name for a base cookie name.
func (h *Handler) getCookieName(cookieName string) string {
	return getCookieNameWithCookiePrefixed(h.Config, cookieName)
}

// getCookieNameWithCookiePrefixed picks the strongest prefix each cookie
// actually qualifies for. A browser silently DISCARDS a cookie whose attributes
// do not back its prefix (RFC 6265bis 4.1.3), so this cannot be one hardcoded
// string:
//
//	__Host-    needs Secure, no Domain, and Path exactly "/"
//	__Secure-  needs Secure only — any Domain, any Path
//
// Either a non-root Path or a Domain therefore disqualifies __Host-. oauth_tx is
// path-scoped and csrf carries a Domain, so both can only be __Secure-.
func getCookieNameWithCookiePrefixed(cfg *config.Config, cookieName string) string {
	if !cfg.IsProduction() {
		return cookieName
	}
	if getCookiePathByCookieName(cookieName) != defaultCookiePath ||
		getCookieDomainByCookieName(cfg, cookieName) != "" {
		return "__Secure-" + cookieName
	}
	return "__Host-" + cookieName
}

// getCookiePathByCookieName is the single source of each cookie's Path. Both the
// Set-Cookie header and the prefix decision read it, so they cannot drift apart.
func getCookiePathByCookieName(cookieName string) string {
	if cookieName == oauthTxCookie {
		return OAuthTxCookiePath
	}
	return defaultCookiePath
}

// getCookieDomainByCookieName widens csrf and nothing else. The SPA has to READ
// that value out of document.cookie to echo it in X-CSRF-Token, and a cookie
// locked to the API's host is invisible to a page served from another host.
//
// sid and oauth_tx stay host-locked: the browser already sends them to the API
// on destination-host match, so widening them would buy nothing and would let
// any sibling subdomain read and overwrite the session.
func getCookieDomainByCookieName(cfg *config.Config, cookieName string) string {
	if cookieName == csrfCookie {
		return cfg.CookieDomain()
	}
	return ""
}

// setCookie is the only place cookie attributes are chosen. ttl becomes Max-Age,
// not Expires, so a skewed client clock cannot mis-expire the cookie. SameSite is
// Lax, not Strict: Google's redirect back is a cross-site top-level navigation.
func (h *Handler) setCookie(w http.ResponseWriter, params cookieParams) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.getCookieName(params.name),
		Value:    params.value,
		Path:     getCookiePathByCookieName(params.name),
		Domain:   getCookieDomainByCookieName(h.Config, params.name),
		MaxAge:   int(params.ttl.Seconds()),
		HttpOnly: params.httpOnly,
		Secure:   h.isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearCookie expires a cookie. Name, Path and Domain are all derived from the
// name, which is what makes the deletion target the cookie that was set: cookie
// identity is (name, domain, path), and getting any one of the three wrong
// expires a different cookie while the real one survives.
func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	h.setCookie(w, cookieParams{
		name:     name,
		ttl:      -time.Second, // MaxAge < 0 deletes it
		httpOnly: true,
	})
}

func (h *Handler) readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(h.getCookieName(name))
	if err != nil {
		return ""
	}
	return c.Value
}

// setSessionCookie keeps the id HttpOnly: no script reads it, so XSS cannot
// steal it the way it could from localStorage.
func (h *Handler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	h.setCookie(w, cookieParams{
		name:     sessionCookie,
		value:    sessionID,
		ttl:      h.Config.Auth.SessionTTL,
		httpOnly: true,
	})
}

// setCSRFCookie is intentionally not HttpOnly: the SPA must read it to echo it
// back in a header. It is not a credential, only proof of same-origin access.
func (h *Handler) setCSRFCookie(w http.ResponseWriter, token string) {
	h.setCookie(w, cookieParams{
		name:     csrfCookie,
		value:    token,
		ttl:      h.Config.Auth.SessionTTL,
		httpOnly: false,
	})
}

// setOAuthTxCookie carries the sealed login transaction, scoped to the callback
// path and expiring with the transaction so nothing else can reach it.
func (h *Handler) setOAuthTxCookie(w http.ResponseWriter, sealed string) {
	h.setCookie(w, cookieParams{
		name:     oauthTxCookie,
		value:    sealed,
		ttl:      loginTxTTL,
		httpOnly: true,
	})
}

// CSRFCookieName is the resolved name the CSRF middleware must read. It takes
// config because the router wires middleware before any Handler exists.
func CSRFCookieName(cfg *config.Config) string {
	return getCookieNameWithCookiePrefixed(cfg, csrfCookie)
}
