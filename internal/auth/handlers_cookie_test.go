package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

// Every bug this file guards against is production-only and invisible to the
// compiler: getCookieNameWithCookiePrefixed short-circuits outside production,
// so local development never validates a prefix against its attributes.

func testHandler(env, frontend string) *Handler {
	cfg := &config.Config{Env: env}
	cfg.Auth.SessionTTL = 12 * time.Hour
	cfg.AppBaseURL.Frontend = frontend
	return &Handler{Handler: handler.Handler{Config: cfg}}
}

// setters is every cookie the package writes, so a new one must be added here.
func setters(h *Handler) map[string]func(*httptest.ResponseRecorder) {
	return map[string]func(*httptest.ResponseRecorder){
		sessionCookie: func(w *httptest.ResponseRecorder) { h.setSessionCookie(w, "VALUE") },
		csrfCookie:    func(w *httptest.ResponseRecorder) { h.setCSRFCookie(w, "VALUE") },
		oauthTxCookie: func(w *httptest.ResponseRecorder) { h.setOAuthTxCookie(w, "VALUE") },
	}
}

// domainMatch is the browser's rule (RFC 6265 5.1.3). A cookie with a Domain is
// only accepted if the host that SET it domain-matches that Domain, and is only
// sent to hosts that domain-match it.
func domainMatch(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// A prefix the attributes do not back is discarded outright by the browser —
// no error, no warning, the cookie simply never exists.
func TestCookiePrefixesAreBackedByAttributes(t *testing.T) {
	h := testHandler("production", "https://www.mvip-league.com")
	for base, set := range setters(h) {
		w := httptest.NewRecorder()
		set(w)
		c := w.Result().Cookies()[0]
		t.Logf("%-9s %s", base, w.Header().Get("Set-Cookie"))

		switch {
		case strings.HasPrefix(c.Name, "__Host-"):
			if !c.Secure || c.Domain != "" || c.Path != defaultCookiePath {
				t.Errorf("%s discarded: __Host- needs Secure + no Domain + Path=%q, got secure=%v domain=%q path=%q",
					c.Name, defaultCookiePath, c.Secure, c.Domain, c.Path)
			}
		case strings.HasPrefix(c.Name, "__Secure-"):
			if !c.Secure {
				t.Errorf("%s discarded: __Secure- needs Secure", c.Name)
			}
		default:
			t.Errorf("%s has no prefix in production", c.Name)
		}
	}
}

// Path must come from the derivation, never from a caller, or a cookie ends up
// named for one path and written at another.
func TestEmittedPathMatchesTheDerivedPath(t *testing.T) {
	h := testHandler("production", "https://www.mvip-league.com")
	for base, set := range setters(h) {
		w := httptest.NewRecorder()
		set(w)
		if got, want := w.Result().Cookies()[0].Path, getCookiePathByCookieName(base); got != want {
			t.Errorf("%s: emitted Path=%q, derived %q", base, got, want)
		}
	}
}

// The csrf cookie is the only one the SPA must read, and it has to survive being
// set by the API host while being readable from the frontend host.
func TestCSRFCookieReachesBothHosts(t *testing.T) {
	const apiHost, spaHost = "be.mvip-league.com", "www.mvip-league.com"
	h := testHandler("production", "https://"+spaHost)

	w := httptest.NewRecorder()
	h.setCSRFCookie(w, "TOKEN")
	c := w.Result().Cookies()[0]

	if !domainMatch(apiHost, c.Domain) {
		t.Fatalf("rejected: %s may not set Domain=%q", apiHost, c.Domain)
	}
	if !domainMatch(spaHost, c.Domain) {
		t.Fatalf("document.cookie on %s would be empty: Domain=%q does not cover it", spaHost, c.Domain)
	}
	if c.HttpOnly {
		t.Error("csrf must not be HttpOnly; the SPA has to read it")
	}
}

// sid and oauth_tx must stay host-locked and unreadable by script.
func TestSessionAndOAuthTxStayHostLocked(t *testing.T) {
	h := testHandler("production", "https://www.mvip-league.com")
	for _, base := range []string{sessionCookie, oauthTxCookie} {
		w := httptest.NewRecorder()
		setters(h)[base](w)
		c := w.Result().Cookies()[0]
		if c.Domain != "" {
			t.Errorf("%s: Domain=%q, must stay host-locked", base, c.Domain)
		}
		if !c.HttpOnly {
			t.Errorf("%s: must be HttpOnly", base)
		}
	}
}

// Cookie identity is (name, domain, path): a clear that differs in any of the
// three expires a different cookie and leaves the real one alive.
func TestClearCookieMatchesSetCookie(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		h := testHandler(env, "https://www.mvip-league.com")
		for base, set := range setters(h) {
			sw, cw := httptest.NewRecorder(), httptest.NewRecorder()
			set(sw)
			h.clearCookie(cw, base)
			s, c := sw.Result().Cookies()[0], cw.Result().Cookies()[0]
			if s.Name != c.Name || s.Path != c.Path || s.Domain != c.Domain {
				t.Errorf("%s/%s: set(%s,%s,%s) != clear(%s,%s,%s)",
					env, base, s.Name, s.Path, s.Domain, c.Name, c.Path, c.Domain)
			}
			if c.MaxAge >= 0 {
				t.Errorf("%s/%s: clear MaxAge=%d, must be negative", env, base, c.MaxAge)
			}
		}
	}
}

// getCookie has to look for the name setCookie wrote.
func TestCookieNameRoundTrips(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		h := testHandler(env, "https://www.mvip-league.com")
		for base, set := range setters(h) {
			w := httptest.NewRecorder()
			set(w)
			r := httptest.NewRequest(http.MethodGet, "https://be.mvip-league.com/api/v1/me", nil)
			for _, c := range w.Result().Cookies() {
				r.AddCookie(c)
			}
			if got := h.getCookie(r, base); got != "VALUE" {
				t.Errorf("%s/%s: getCookie returned %q", env, base, got)
			}
		}
	}
}

// The CSRF middleware is constructed from config in the router before any
// Handler exists, so the name it reads is computed by a second path and must
// agree — otherwise the gate looks for a cookie nobody sets and every write 403s.
func TestCSRFCookieNameAgreesWithTheEmittedCookie(t *testing.T) {
	for _, env := range []string{"development", "production"} {
		h := testHandler(env, "https://www.mvip-league.com")
		w := httptest.NewRecorder()
		h.setCSRFCookie(w, "TOKEN")
		if got, want := CSRFCookieName(h.Config), w.Result().Cookies()[0].Name; got != want {
			t.Errorf("%s: CSRFCookieName=%q but the cookie is %q", env, got, want)
		}
	}
}

// Local development runs over plain http, so a Secure or prefixed cookie there
// would break login on localhost.
func TestDevelopmentHasNoPrefixAndNoSecure(t *testing.T) {
	h := testHandler("development", "http://localhost:5174")
	for base, set := range setters(h) {
		w := httptest.NewRecorder()
		set(w)
		c := w.Result().Cookies()[0]
		if c.Name != base || c.Secure || c.Domain != "" {
			t.Errorf("%s: name=%q secure=%v domain=%q want %q/false/empty",
				base, c.Name, c.Secure, c.Domain, base)
		}
	}
}
