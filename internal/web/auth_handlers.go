package web

import (
	"accounting/internal/auth"
	"net/http"
	"net/url"
)

// LoginGet shows the login form (or redirects if already signed in).
func (h *Handler) LoginGet(w http.ResponseWriter, r *http.Request) {
	if h.Auth != nil && h.Auth.Authenticated(r) {
		http.Redirect(w, r, auth.RedirectTarget(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	h.renderView(w, r, "login.tmpl", map[string]any{
		"Next":  r.URL.Query().Get("next"),
		"Flash": readFlash(r),
	})
}

// LoginPost validates the password and sets a session cookie.
func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	next := r.FormValue("next")
	if !h.Auth.PasswordOK(r.FormValue("password")) {
		redirectMsg(w, r, "/login?next="+url.QueryEscape(next), "Invalid password.")
		return
	}
	h.Auth.SetSession(w)
	http.Redirect(w, r, auth.RedirectTarget(next), http.StatusSeeOther)
}

// Logout clears the session cookie and sends the user to the login page.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.Auth != nil {
		h.Auth.ClearSession(w)
	}
	redirectMsg(w, r, "/login", "Signed out.")
}
