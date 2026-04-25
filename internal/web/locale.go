package web

import (
	"accounting/internal/auth"
	"accounting/internal/i18n"
	"net/http"
	"net/url"
)

// viewData adds language, direction, dictionary, and language-switch URLs to template data.
func (h *Handler) viewData(r *http.Request, base map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any)
	}
	def := h.defaultLang()
	lang := i18n.FromRequest(r, def)
	base["Lang"] = lang
	base["L"] = i18n.Dict(lang)
	base["Dir"] = i18n.Dir(lang)
	base["HTMLLang"] = i18n.HTMLAttr(lang)
	next := r.URL.RequestURI()
	if next == "" {
		next = "/"
	}
	q := url.QueryEscape(next)
	base["URLLangEN"] = "/lang?l=" + i18n.LangEN + "&next=" + q
	base["URLLangFA"] = "/lang?l=" + i18n.LangFA + "&next=" + q
	return base
}

func (h *Handler) defaultLang() string {
	if h != nil && h.DefaultLang != "" {
		return h.DefaultLang
	}
	return i18n.LangEN
}

func (h *Handler) renderView(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	h.render(w, name, h.viewData(r, data))
}

// SetLang sets the acct_lang cookie and redirects (public route).
func (h *Handler) SetLang(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	l := r.URL.Query().Get("l")
	if l == "" {
		l = r.URL.Query().Get("lang")
	}
	lang := i18n.ParseLangQuery(l)
	if lang != i18n.LangEN && lang != i18n.LangFA {
		lang = h.defaultLang()
	}
	i18n.SetCookie(w, lang)
	next := auth.RedirectTarget(r.URL.Query().Get("next"))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *Handler) tr(r *http.Request, key string) string {
	return i18n.T(i18n.FromRequest(r, h.defaultLang()), key)
}

func (h *Handler) trFormat(r *http.Request, key string, a ...any) string {
	return i18n.Format(i18n.FromRequest(r, h.defaultLang()), key, a...)
}
