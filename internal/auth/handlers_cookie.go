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
	// compile error. Asserted against the router in TestOAuthTxCookiePathCoversRoutes.
	OAuthTxCookiePath = "/api/v1/auth/google"

	defautlCookePath = "/"
)

// secure keeps the Secure attribute and the __Host- prefix it requires in one
// place, so the two cannot disagree.
func (h *Handler) isSecure() bool {
	return h.Config.IsProduction()
}

// cookieName prefixes the cookie in production, so no subdomain can overwrite it.
func (h *Handler) getCookieName(cookieName string) string {
	return getCookieNameWithCookiePrefixed(h.Config, cookieName)
}

// cookieNameFor picks the strongest prefix each cookie actually qualifies for.
// The browser silently DISCARDS a cookie whose attributes do not back its
// prefix (RFC 6265bis 4.1.3), so this cannot be a single hardcoded string:
//
//	__Host-    needs Secure, no Domain, and Path exactly "/"
//	__Secure-  needs Secure only — any Path is fine
//
// oauth_tx is scoped to OAuthTxCookiePath and csrf carries a Domain, so neither
// can satisfy __Host-. Naming one __Host- anyway means the browser stores
// nothing: login fails with "no oauth_tx cookie", writes fail with 40302.
func getCookieNameWithCookiePrefixed(cfg *config.Config, cookieName string) string {
	if !cfg.IsProduction() {
		return cookieName
	}
	// if h.getCookiePathByCookieName(cookieName) != "/" || h.getCookieDomainByCookieName(cookieName) != "" {
	if getCookiePathByCookieName(cookieName) != "/" {
		return "__Secure-" + cookieName
	}
	return "__Host-" + cookieName
}

// cookiePathFor is the Path each cookie is set with. The prefix decision reads
// it too, so the two can never drift apart.
func getCookiePathByCookieName(cookieName string) string {
	if cookieName == oauthTxCookie {
		return OAuthTxCookiePath
	}
	return "/"
}

func getCookieDomainByCookieName(cfg *config.Config, cookieName string) string {
	if cookieName == csrfCookie {
		return cfg.CookieDomain()
	}
	return ""
}

// setCookie is the only place cookie attributes are chosen. ttl becomes Max-Age,
// not Expires, so a skewed client clock cannot mis-expire the cookie. SameSite is
// Lax, not Strict: Google's redirect back is a cross-site top-level navigation.
func (h *Handler) setCookie(w http.ResponseWriter, params CookieParams) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.getCookieName(params.name),
		Value:    params.value,
		Path:     params.path,
		Domain:   getCookieDomainByCookieName(h.Config, params.name),
		MaxAge:   int(params.ttl.Seconds()),
		HttpOnly: params.httpOnly,
		Secure:   h.isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name, path string) {
	h.setCookie(w, CookieParams{
		name:     name,
		path:     path,
		ttl:      -time.Second,
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
	h.setCookie(w, CookieParams{
		name:     sessionCookie,
		value:    sessionID,
		path:     defautlCookePath,
		ttl:      h.Config.Auth.SessionTTL,
		httpOnly: true,
	})
}

// setCSRFCookie is intentionally not HttpOnly: the SPA must read it to echo it
// back in a header. It is not a credential, only proof of same-origin access.
func (h *Handler) setCSRFCookie(w http.ResponseWriter, token string) {
	h.setCookie(w, CookieParams{
		name:     csrfCookie,
		value:    token,
		path:     defautlCookePath,
		ttl:      h.Config.Auth.SessionTTL,
		httpOnly: false,
	})
}

// setOAuthTxCookie carries the sealed login transaction, scoped to the callback
// path and expiring with the transaction so nothing else can reach it.
func (h *Handler) setOAuthTxCookie(w http.ResponseWriter, sealed string) {
	h.setCookie(w, CookieParams{
		name:     oauthTxCookie,
		value:    sealed,
		path:     OAuthTxCookiePath,
		ttl:      loginTxTTL,
		httpOnly: true,
	})
}

// CSRFCookieName is the resolved name the CSRF middleware must read. It takes
// config because the router wires middleware before any Handler exists.
func CSRFCookieName(cfg *config.Config) string {
	return getCookieNameWithCookiePrefixed(cfg, csrfCookie)
}
