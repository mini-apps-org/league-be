package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

const (
	sessionCookie = "sid"      // server-side session id
	oauthTxCookie = "oauth_tx" // binds a login transaction to this browser
	csrfCookie    = "csrf"     // double-submit token; readable by JS on purpose
)

// OAuthTxCookiePath scopes the oauth_tx cookie so nothing outside the Google
// login routes can read it. It MUST stay a prefix of the mounted callback path:
// the browser withholds the cookie on a path mismatch, and the callback then
// fails with "no oauth_tx cookie" — a silent, total login outage with no
// compile error.
const OAuthTxCookiePath = "/api/v1/auth/google"

// cookiePath is the single source of each cookie's Path. Both the Set-Cookie
// header and the prefix decision read it, so a cookie can never be written at
// one path and named for another — that mismatch is silent, and costs a login.
func cookiePath(base string) string {
	if base == oauthTxCookie {
		return OAuthTxCookiePath
	}
	return "/"
}

// secure keeps the Secure attribute and the prefixes that require it in one
// place, so the two cannot disagree.
func (h *Handler) secure() bool {
	return h.Config.IsProduction()
}

func (h *Handler) cookieName(base string) string {
	return cookieNameFor(h.Config, base)
}

// cookieNameFor prefixes cookies in production. It is __Secure- and not __Host-
// because __Host- forbids a Domain attribute, and the browser silently DROPS a
// cookie whose attributes do not back its prefix (RFC 6265bis 4.1.3). __Secure-
// asks only for Secure, which production sets.
func cookieNameFor(cfg *config.Config, base string) string {
	if !cfg.IsProduction() {
		return base
	}
	return "__Secure-" + base
}

// setCookie is the only place cookie attributes are chosen. ttl becomes Max-Age,
// not Expires, so a skewed client clock cannot mis-expire the cookie. SameSite is
// Lax, not Strict: Google's redirect back is a cross-site top-level navigation.
//
// Domain is the frontend's host, so the SPA can read the csrf token even though
// the API set it. That also means every subdomain of it can read these cookies.
func (h *Handler) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration, httpOnly bool) {
	fmt.Println(h.Config.CookieDomain(), cookiePath(name), "h.Config.CookieDomain(), cookiePath(name)")
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName(name),
		Value:    value,
		Path:     "/",
		Domain:   h.Config.CookieDomain(),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: httpOnly,
		Secure:   h.secure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearCookie must reproduce the original Name, Path and Domain exactly, or the
// browser treats it as a different cookie and the old one survives.
func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	h.setCookie(w, name, "", -time.Second, true) // MaxAge < 0 deletes it
}

func (h *Handler) readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(h.cookieName(name))
	if err != nil {
		return ""
	}
	return c.Value
}

// setSessionCookie keeps the id HttpOnly: no script reads it, so XSS cannot
// steal it the way it could from localStorage.
func (h *Handler) setSessionCookie(w http.ResponseWriter, sessionID string) {
	h.setCookie(w, sessionCookie, sessionID, h.Config.Auth.SessionTTL, true)
}

// setCSRFCookie is intentionally not HttpOnly: the SPA must read it to echo it
// back in a header. It is not a credential, only proof of same-origin access.
// This is the one cookie that needs COOKIE_DOMAIN when the SPA is on another
// host — document.cookie cannot see a cookie locked to the API's host.
func (h *Handler) setCSRFCookie(w http.ResponseWriter, token string) {
	h.setCookie(w, csrfCookie, token, h.Config.Auth.SessionTTL, false)
}

// setOAuthTxCookie carries the sealed login transaction, scoped to the callback
// path and expiring with the transaction so nothing else can reach it.
func (h *Handler) setOAuthTxCookie(w http.ResponseWriter, sealed string) {
	h.setCookie(w, oauthTxCookie, sealed, loginTxTTL, true)
}

// CSRFCookieName is the resolved name the CSRF middleware must read. It takes
// config because the router wires middleware before any Handler exists.
func CSRFCookieName(cfg *config.Config) string {
	return cookieNameFor(cfg, csrfCookie)
}
