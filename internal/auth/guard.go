package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

const (
	cookieName   = "acct_auth"
	cookieMaxAge = 30 * 24 * 60 * 60 // 30 days
)

// DefaultPassword is used when env ACCOUNTING_PASSWORD is unset.
const DefaultPassword = "changeme"

// DefaultAuthKey is used when env ACCOUNTING_AUTH_KEY is unset (not for production).
const DefaultAuthKey = "insecure-dev-auth-key"

// Guard holds the expected password and the signed session cookie value.
type Guard struct {
	password  string
	sessionOK string
}

// New creates a guard. key material is used to derive the session cookie value;
// password is the login passphrase (plain, minimal security).
func New(password, authKey string) *Guard {
	if password == "" {
		password = DefaultPassword
	}
	if authKey == "" {
		authKey = DefaultAuthKey
	}
	return &Guard{
		password:  password,
		sessionOK: sessionToken([]byte(authKey)),
	}
}

func sessionToken(key []byte) string {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte("accounting|session|v1"))
	return hex.EncodeToString(m.Sum(nil))
}

// PasswordOK checks the submitted password.
func (g *Guard) PasswordOK(p string) bool {
	if g == nil {
		return false
	}
	// Not constant-time across length; sufficient for a minimal self-hosted gate.
	return p == g.password
}

// Authenticated returns true if the request carries a valid session cookie.
func (g *Guard) Authenticated(r *http.Request) bool {
	if g == nil {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return c.Value == g.sessionOK
}

// IsPublicPath skips auth (login, static assets, language switch).
func IsPublicPath(path string) bool {
	if path == "/login" || path == "/logout" || path == "/lang" {
		return true
	}
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	return false
}

// SetSession sets the httpOnly session cookie.
func (g *Guard) SetSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    g.sessionOK,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(),
	})
}

// ClearSession removes the session cookie.
func (g *Guard) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// isHTTPS is a minimal hint; in Docker behind TLS terminator, you may set Forwarded headers — keep Secure off unless you set it via env in a future change.
func isHTTPS() bool { return false }

// Wrap enforces password gate for all non-public routes.
func (g *Guard) Wrap(next http.Handler) http.Handler {
	if g == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if g.Authenticated(r) {
			next.ServeHTTP(w, r)
			return
		}
		redir := "/login?next=" + url.QueryEscape(RedirectTarget(r.URL.RequestURI()))
		http.Redirect(w, r, redir, http.StatusSeeOther)
	})
}

// RedirectTarget only allows same-site path+query (avoids open redirects).
func RedirectTarget(raw string) string {
	if raw == "" {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "/"
	}
	// only path + query, no //evil.com
	if u.Host != "" || u.Scheme != "" {
		return "/"
	}
	if u.Path == "" {
		return "/"
	}
	if !strings.HasPrefix(u.Path, "/") {
		return "/"
	}
	out := u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}
