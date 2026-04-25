package web

import (
	"accounting/internal/app"
	"accounting/internal/auth"
	"accounting/internal/db"
	"accounting/internal/gold"
	"accounting/internal/i18n"
	"database/sql"
	"errors"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// assetLineView is a list row: adds per-line Rial equivalent for gold when a price is cached.
type assetLineView struct {
	db.Asset
	GoldRialLine sql.NullInt64
}

type Handler struct {
	DB                *sql.DB
	Auth              *auth.Guard
	Display           *time.Location // display time zone; stored times in DB are UTC
	GoldAPIURL       string  // optional; empty uses package default
	GoldRialPerToman float64 // Rial per Toman; default 10
	GoldPriceScale   float64 // optional extra factor on toman/mg; default 1
	DefaultLang      string  // "en" or "fa" when language cookie is absent
	tmpl             *template.Template
	tmu              sync.Mutex
}

func (h *Handler) rialForGoldHolding(mg, tomanPerMg float64) float64 {
	rt := gold.DefaultRialPerToman
	if h != nil && h.GoldRialPerToman > 0 {
		rt = h.GoldRialPerToman
	}
	s := 1.0
	if h != nil && h.GoldPriceScale > 0 {
		s = h.GoldPriceScale
	}
	return gold.RialFromTomanPerMilligram(mg, tomanPerMg, rt, s)
}

func (h *Handler) displayLoc() *time.Location {
	if h != nil && h.Display != nil {
		return h.Display
	}
	return time.UTC
}

func (h *Handler) parseTemplates() *template.Template {
	t := template.New("").Funcs(app.TmplFuncs(h.displayLoc()))
	entries, err := fs.Glob(Files, "templates/*.tmpl")
	if err != nil {
		panic(err)
	}
	// Register partials before pages that {{template "nav_main" .}}.
	slices.SortFunc(entries, func(a, b string) int {
		pa, pb := path.Base(a), path.Base(b)
		if pa == "partials.tmpl" {
			return -1
		}
		if pb == "partials.tmpl" {
			return 1
		}
		return strings.Compare(a, b)
	})
	for _, e := range entries {
		b, err := fs.ReadFile(Files, e)
		if err != nil {
			panic(err)
		}
		name := path.Base(e)
		if t, err = t.New(name).Parse(string(b)); err != nil {
			panic(err)
		}
	}
	return t
}

func (h *Handler) getTemplates() *template.Template {
	h.tmu.Lock()
	defer h.tmu.Unlock()
	if h.tmpl == nil {
		h.tmpl = h.parseTemplates()
	}
	return h.tmpl
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	tt := h.getTemplates()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tt.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func readFlash(r *http.Request) string {
	return r.URL.Query().Get("msg")
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	rial, _ := db.SumRialAssets(h.DB)
	mg, _ := db.SumGoldMilligrams(h.DB)
	var goldRial int64
	lang := i18n.FromRequest(r, h.defaultLang())
	var quoteNote string
	gp, err := db.GetCachedGoldPrice(h.DB)
	if err == nil {
		p := h.rialForGoldHolding(mg, gp.PriceRial)
		goldRial = int64(p + 0.5)
		quoteNote = i18n.T(lang, "home_quote_yes")
	} else {
		quoteNote = i18n.T(lang, "home_quote_no")
	}
	tinc, _ := db.TotalIncome(h.DB)
	texp, _ := db.TotalExpense(h.DB)
	lf := lastFetchString(gp, err, h.displayLoc())
	subTimes := i18n.Format(lang, "home_sub_times", h.displayLoc().String())
	totalSub := i18n.Format(lang, "home_total_sub", lf)
	h.renderView(w, r, "home.tmpl", map[string]any{
		"Tab":        "home",
		"RialAssets": rial,
		"GoldMg":     mg,
		"GoldRial":   goldRial,
		"TotalEquiv": rial + goldRial,
		"Income":     tinc,
		"Expense":    texp,
		"NetFlow":    tinc - texp,
		"QuoteNote":  quoteNote,
		"SubTimes":   subTimes,
		"TotalSub":   totalSub,
		"LastFetch":  lf,
		"TZName":     h.displayLoc().String(),
		"Flash":      readFlash(r),
	})
}

func lastFetchString(gp db.GoldPriceRow, err error, loc *time.Location) string {
	if err != nil {
		return "—"
	}
	if gp.FetchedAt.IsZero() {
		return "—"
	}
	if loc == nil {
		loc = time.UTC
	}
	return gp.FetchedAt.In(loc).Format("2006-01-02 15:04")
}

func (h *Handler) AssetsPage(w http.ResponseWriter, r *http.Request) {
	list, _ := db.ListAssets(h.DB)
	mg, _ := db.SumGoldMilligrams(h.DB)
	rial, _ := db.SumRialAssets(h.DB)
	gp, err := db.GetCachedGoldPrice(h.DB)
	var goldRial int64
	var priceF float64
	var change24 string
	if err == nil {
		priceF = gp.PriceRial
		change24 = gp.RawChange24h
		goldRial = int64(h.rialForGoldHolding(mg, gp.PriceRial) + 0.5)
	}
	lf := lastFetchString(gp, err, h.displayLoc())
	gold24 := ""
	if err == nil {
		gold24 = i18n.Format(i18n.FromRequest(r, h.defaultLang()), "assets_24h", change24, lf)
	}
	lines := make([]assetLineView, 0, len(list))
	for _, a := range list {
		v := assetLineView{Asset: a}
		if a.Kind == db.AssetGold && err == nil && a.GoldMilligrams.Valid {
			eq := int64(h.rialForGoldHolding(a.GoldMilligrams.Float64, gp.PriceRial) + 0.5)
			v.GoldRialLine = sql.NullInt64{Int64: eq, Valid: true}
		}
		lines = append(lines, v)
	}
	h.renderView(w, r, "assets.tmpl", map[string]any{
		"Tab":         "assets",
		"Assets":      lines,
		"SumRial":     rial,
		"SumGoldMg":   mg,
		"GoldRial":    goldRial,
		"Price":       priceF,
		"Change24":    change24,
		"HasQuote":    err == nil,
		"LastFetch":   lf,
		"GoldLine24":  gold24,
		"TZName":      h.displayLoc().String(),
		"PriceUnit":   h.tr(r, "assets_price_unit"),
		"clockSuffix": h.trFormat(r, "assets_clock", h.displayLoc().String()),
		"Flash":       readFlash(r),
	})
}

func (h *Handler) AddAssetRial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	label := strings.TrimSpace(r.FormValue("label"))
	amt, err := strconv.ParseInt(strings.ReplaceAll(r.FormValue("amount"), ",", ""), 10, 64)
	if err != nil || amt < 0 {
		redirectMsg(w, r, "/assets", "Invalid Rial amount.")
		return
	}
	_, err = db.CreateAssetRial(h.DB, label, amt)
	if err != nil {
		redirectMsg(w, r, "/assets", "Could not save: "+err.Error())
		return
	}
	redirectMsg(w, r, "/assets", "Rial asset added.")
}

func (h *Handler) AddAssetGold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	label := strings.TrimSpace(r.FormValue("label"))
	g, err := parseFloatForm(r.FormValue("gold_mg"))
	if err != nil || g < 0 {
		redirectMsg(w, r, "/assets", "Invalid gold amount (milligrams).")
		return
	}
	_, err = db.CreateAssetGold(h.DB, label, g)
	if err != nil {
		redirectMsg(w, r, "/assets", "Could not save: "+err.Error())
		return
	}
	redirectMsg(w, r, "/assets", "Gold position added.")
}

func (h *Handler) RefreshGold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	c := &gold.Client{BaseURL: h.GoldAPIURL}
	q, err := c.FetchQuote()
	if err != nil {
		frag := `<p class="flash err">` + html.EscapeString(err.Error()) + `</p>`
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(frag))
			return
		}
		redirectMsg(w, r, "/assets", "Failed to fetch gold price: "+err.Error())
		return
	}
	pr, err := strconv.ParseFloat(strings.TrimSpace(q.Price), 64)
	if err != nil {
		redirectMsg(w, r, "/assets", "API returned an invalid price.")
		return
	}
	if err := db.UpsertGoldPrice(h.DB, pr, q.Change24h); err != nil {
		redirectMsg(w, r, "/assets", "Could not save price: "+err.Error())
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		lf := time.Now().In(h.displayLoc()).Format("2006-01-02 15:04")
		gold24 := i18n.Format(i18n.FromRequest(r, h.defaultLang()), "assets_24h", q.Change24h, lf)
		h.renderView(w, r, "part_goldbar.tmpl", map[string]any{
			"Price":      pr,
			"Change24":   q.Change24h,
			"HasQuote":   true,
			"LastFetch":  lf,
			"GoldLine24": gold24,
		})
		return
	}
	redirectMsg(w, r, "/assets", "Gold price updated.")
}

func (h *Handler) TxnsPage(w http.ResponseWriter, r *http.Request) {
	list, _ := db.ListRecentTransactions(h.DB, 200)
	loc := h.displayLoc()
	today := time.Now().In(loc).Format("2006-01-02")
	h.renderView(w, r, "transactions.tmpl", map[string]any{
		"Tab":            "txns",
		"Today":          today,
		"TZName":         loc.String(),
		"dateZoneSuffix": h.trFormat(r, "tx_date_hint", loc.String()),
		"Transactions":   list,
		"Flash":          readFlash(r),
	})
}

func (h *Handler) AddTxn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/transactions", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	kind := r.FormValue("kind")
	if kind != db.TxnIncome && kind != db.TxnExpense {
		redirectMsg(w, r, "/transactions", "Invalid transaction type.")
		return
	}
	amt, err := strconv.ParseInt(strings.ReplaceAll(r.FormValue("amount"), ",", ""), 10, 64)
	if err != nil || amt <= 0 {
		redirectMsg(w, r, "/transactions", "Invalid amount.")
		return
	}
	cat := strings.TrimSpace(r.FormValue("category"))
	src := strings.TrimSpace(r.FormValue("source"))
	note := strings.TrimSpace(r.FormValue("note"))
	loc := h.displayLoc()
	day, err := time.Parse("2006-01-02", r.FormValue("occurred_on"))
	if err != nil {
		now := time.Now().In(loc)
		day = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	} else {
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	}
	_, err = db.CreateTransaction(h.DB, kind, amt, cat, src, note, day)
	if err != nil {
		redirectMsg(w, r, "/transactions", "Could not save: "+err.Error())
		return
	}
	redirectMsg(w, r, "/transactions", "Transaction saved.")
}

type monthRowT struct {
	Month   string
	Income  int64
	Expense int64
}

func (h *Handler) ReportsPage(w http.ResponseWriter, r *http.Request) {
	mtot, _ := db.MonthTotals(h.DB)
	var keys []string
	for k := range mtot {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int { return strings.Compare(b, a) })
	months := make([]monthRowT, 0, len(keys))
	for _, k := range keys {
		v := mtot[k]
		months = append(months, monthRowT{Month: k, Income: v.Income, Expense: v.Expense})
	}
	src, _ := db.SourceTotals(h.DB)
	cat, _ := db.CategoryTotals(h.DB)
	tinc, _ := db.TotalIncome(h.DB)
	texp, _ := db.TotalExpense(h.DB)
	h.renderView(w, r, "reports.tmpl", map[string]any{
		"Tab":         "reports",
		"Months":      months,
		"Source":      src,
		"Category":    cat,
		"Income":      tinc,
		"Expense":     texp,
		"Net":         tinc - texp,
		"TZName":      h.displayLoc().String(),
		"repMonthSub": h.trFormat(r, "rep_month_hint", h.displayLoc().String()),
		"Flash":       readFlash(r),
	})
}

func (h *Handler) Static() http.Handler {
	fsys, err := fs.Sub(Files, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(fsys)))
}

func redirectMsg(w http.ResponseWriter, r *http.Request, path, msg string) {
	u := path
	if msg != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		u = path + sep + "msg=" + url.QueryEscape(msg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func parseFloatForm(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	return strconv.ParseFloat(s, 64)
}
