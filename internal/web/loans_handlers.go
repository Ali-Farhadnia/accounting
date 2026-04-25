package web

import (
	"accounting/internal/db"
	"accounting/internal/i18n"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const loanUpcomingHorizonDays = 30

type loanPartition struct {
	Overdue, DueToday, Upcoming, Later []db.Loan
}

// LoanView adds display fields for the loans UI.
type LoanView struct {
	db.Loan
	RelativeDue string
}

func formatLoanRelative(lang string, day int) string {
	if day == 0 {
		return i18n.T(lang, "loans_rel_today")
	}
	if day < 0 {
		n := -day
		if n == 1 {
			return i18n.T(lang, "loans_rel_1d_late")
		}
		return i18n.Format(lang, "loans_rel_nd_late", n)
	}
	if day == 1 {
		return i18n.T(lang, "loans_rel_tomorrow")
	}
	return i18n.Format(lang, "loans_rel_in_nd", day)
}

func enrichLoanViews(lang string, loc *time.Location, loans []db.Loan) []LoanView {
	now := time.Now().In(loc)
	out := make([]LoanView, 0, len(loans))
	for _, l := range loans {
		day, err := loanDayFromToday(loc, l.DueDate, now)
		if err != nil {
			day = 0
		}
		out = append(out, LoanView{Loan: l, RelativeDue: formatLoanRelative(lang, day)})
	}
	return out
}

func loanDayFromToday(loc *time.Location, dueDateStr string, now time.Time) (int, error) {
	if loc == nil {
		loc = time.UTC
	}
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dueDateStr), loc)
	if err != nil {
		return 0, err
	}
	d0 := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	t0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return int(d0.Sub(t0).Hours() / 24), nil
}

func (h *Handler) partitionLoans(loans []db.Loan) loanPartition {
	loc := h.displayLoc()
	now := time.Now().In(loc)
	ys, ms, ds := now.Date()
	todayStart := time.Date(ys, ms, ds, 0, 0, 0, 0, loc)
	tomorrow := todayStart.AddDate(0, 0, 1)
	horizonEnd := todayStart.AddDate(0, 0, loanUpcomingHorizonDays)
	var p loanPartition
	for _, l := range loans {
		due, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(l.DueDate), loc)
		if err != nil {
			p.Later = append(p.Later, l)
			continue
		}
		if due.Before(todayStart) {
			p.Overdue = append(p.Overdue, l)
		} else if !due.Before(tomorrow) {
			if !due.After(horizonEnd) {
				p.Upcoming = append(p.Upcoming, l)
			} else {
				p.Later = append(p.Later, l)
			}
		} else {
			p.DueToday = append(p.DueToday, l)
		}
	}
	return p
}

// LoansPage lists active loans grouped by due status, plus settled; reminders are computed on each GET.
func (h *Handler) LoansPage(w http.ResponseWriter, r *http.Request) {
	list, _ := db.ListActiveLoans(h.DB)
	p := h.partitionLoans(list)
	settled, _ := db.ListSettledLoans(h.DB, 80)
	loc := h.displayLoc()
	lang := i18n.FromRequest(r, h.defaultLang())
	n := len(list)
	urgentN := len(p.Overdue) + len(p.DueToday)
	var attentionLine string
	if urgentN == 1 {
		attentionLine = i18n.T(lang, "loans_attention_1")
	} else if urgentN > 1 {
		attentionLine = i18n.Format(lang, "loans_attention_n", urgentN)
	}
	h.renderView(w, r, "loans.tmpl", map[string]any{
		"Tab":           "loans",
		"Overdue":       enrichLoanViews(lang, loc, p.Overdue),
		"DueToday":      enrichLoanViews(lang, loc, p.DueToday),
		"Upcoming":      enrichLoanViews(lang, loc, p.Upcoming),
		"Later":         enrichLoanViews(lang, loc, p.Later),
		"Settled":       enrichLoanViews(lang, loc, settled),
		"UpcomingN":     loanUpcomingHorizonDays,
		"TodayISO":      time.Now().In(loc).Format("2006-01-02"),
		"TZName":        loc.String(),
		"dateZoneSub":   h.trFormat(r, "loans_sub_tz", loc.String(), loanUpcomingHorizonDays),
		"NoLoans":       n == 0,
		"UrgentCount":   urgentN,
		"AttentionLine": attentionLine,
		"Flash":         readFlash(r),
	})
}

// AddLoan adds an unsettled loan.
func (h *Handler) AddLoan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/loans", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		redirectMsg(w, r, "/loans", "Label is required.")
		return
	}
	amt, err := strconv.ParseInt(strings.ReplaceAll(r.FormValue("amount"), ",", ""), 10, 64)
	if err != nil || amt < 0 {
		redirectMsg(w, r, "/loans", "Invalid amount.")
		return
	}
	due := strings.TrimSpace(r.FormValue("due"))
	if len(due) != 10 {
		redirectMsg(w, r, "/loans", "Use due date as YYYY-MM-DD.")
		return
	}
	if _, err := time.Parse("2006-01-02", due); err != nil {
		redirectMsg(w, r, "/loans", "Invalid due date.")
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	_, err = db.CreateLoan(h.DB, label, amt, due, note)
	if err != nil {
		redirectMsg(w, r, "/loans", "Could not save: "+err.Error())
		return
	}
	redirectMsg(w, r, "/loans", "Loan saved.")
}

// SettleLoan marks a loan repaid.
func (h *Handler) SettleLoan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/loans", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		redirectMsg(w, r, "/loans", "Missing id.")
		return
	}
	if err := db.SettleLoan(h.DB, id); err != nil {
		redirectMsg(w, r, "/loans", "Could not update: "+err.Error())
		return
	}
	lang := i18n.FromRequest(r, h.defaultLang())
	redirectMsg(w, r, "/loans", i18n.T(lang, "loans_msg_settled"))
}

// DeleteLoan removes a loan row.
func (h *Handler) DeleteLoan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/loans", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		redirectMsg(w, r, "/loans", "Missing id.")
		return
	}
	if err := db.DeleteLoan(h.DB, id); err != nil {
		redirectMsg(w, r, "/loans", "Could not delete: "+err.Error())
		return
	}
	lang := i18n.FromRequest(r, h.defaultLang())
	redirectMsg(w, r, "/loans", i18n.T(lang, "loans_msg_deleted"))
}
