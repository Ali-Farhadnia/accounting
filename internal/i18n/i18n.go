// Package i18n provides English and Persian (Farsi) UI strings.
package i18n

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	LangEN = "en"
	LangFA = "fa"
	// CookieName stores the user UI language.
	CookieName   = "acct_lang"
	cookieMaxAge = 400 * 24 * 60 * 60 // ~400 days
)

// T returns the string for key in the given language, falling back to English.
func T(lang, key string) string {
	lang = normLang(lang)
	m := en
	if lang == LangFA {
		m = fa
	}
	if s, ok := m[key]; ok {
		return s
	}
	if s, ok := en[key]; ok {
		return s
	}
	return key
}

// Dict returns a copy of all strings for templates (e.g. {{index .L "nav_home"}}).
func Dict(lang string) map[string]string {
	out := make(map[string]string, len(en))
	for k, v := range en {
		out[k] = v
	}
	if normLang(lang) == LangFA {
		for k, v := range fa {
			out[k] = v
		}
	}
	return out
}

// FromRequest reads the language cookie, or defaultLang if missing/invalid.
func FromRequest(r *http.Request, defaultLang string) string {
	def := normOrDefault(defaultLang, LangEN)
	if c, err := r.Cookie(CookieName); err == nil {
		if v := normLang(c.Value); v == LangEN || v == LangFA {
			return v
		}
	}
	return def
}

func normOrDefault(s, fallback string) string {
	v := normLang(s)
	if v == "" {
		v = normLang(fallback)
	}
	if v == "" {
		return LangEN
	}
	return v
}

// SetCookie sets the language preference cookie.
func SetCookie(w http.ResponseWriter, lang string) {
	lang = normLang(lang)
	if lang != LangEN && lang != LangFA {
		lang = LangEN
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    lang,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: false, // readable if we need JS later
		SameSite: http.SameSiteLaxMode,
	})
}

// normLang maps common aliases. Unknown values return "".
func normLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "fa", "farsi", "per":
		return LangFA
	case "en", "eng":
		return LangEN
	default:
		return ""
	}
}

// Dir returns "rtl" or "ltr" for the html dir attribute.
func Dir(lang string) string {
	if normLang(lang) == LangFA {
		return "rtl"
	}
	return "ltr"
}

// HTMLAttr returns BCP 47 style lang attribute: fa or en.
func HTMLAttr(lang string) string {
	if normLang(lang) == LangFA {
		return "fa"
	}
	return "en"
}

// ParseLangQuery validates ?l= from the language switcher. Empty if invalid.
func ParseLangQuery(s string) string {
	v := normLang(s)
	if v == LangEN || v == LangFA {
		return v
	}
	return ""
}

// en and fa must define the same keys; fa can omit keys to inherit from en (Dict merges).
var en = map[string]string{
	"brand": "Accounting",

	"nav_home":    "Home",
	"nav_assets":  "Assets",
	"nav_txns":    "Txns",
	"nav_reports": "Reports",
	"nav_loans":   "Loans",
	"nav_logout":  "Log out",
	"lang_en":     "EN",
	"lang_fa":     "فا",

	"title_home":         "Home",
	"title_assets":       "Assets",
	"title_transactions": "Transactions",
	"title_reports":      "Reports",
	"title_loans":        "Loans",
	"title_login":        "Sign in",
	"meta_title_suffix":  "Accounting",

	"home_heading":   "Summary",
	"home_sub_times": "times shown in %s · stored UTC", // %s = TZ name
	"home_cash":      "Cash (Rial)",
	"home_cash_sub":  "rial in asset rows",
	"home_gold":      "Gold",
	"home_gold_sub":  "mg · ≈ %s Rial", // formatRial GoldRial
	"home_total":     "Total (equiv.)",
	"home_total_sub": "last gold fetch: %s",
	"home_income":    "Income (all time)",
	"home_expense":   "Expenses (all time)",
	"home_net":       "Net (income − expense)",
	"home_footer":    "Rial + gold (Talasea) · local SQLite",
	"home_quote_yes": "Gold is valued at the last fetched rate (see Assets).",
	"home_quote_no":  "Gold price not fetched yet — open Assets and refresh the quote.",

	"assets_heading":     "Assets",
	"assets_price_unit":  "API price is Toman per milligram (Talasea); Rial ≈ mg × Toman/mg × 10.",
	"assets_clock":       "clock: %s",
	"assets_gold_market": "Gold market (Talasea)",
	"assets_price_sub":   "Toman / mg (cached)",
	"assets_24h":         "24h: %s · fetched %s",
	"assets_no_price":    "No price cached yet. Use the button below.",
	"assets_refresh":     "Refresh price",
	"assets_totals":      "Totals",
	"assets_lbl_cash":    "Cash",
	"assets_lbl_gold":    "Gold",
	"assets_lbl_goldval": "Gold value",
	"assets_add_rial":    "Add Rial",
	"assets_add_gold":    "Add gold",
	"assets_label":       "Label",
	"assets_amt_rial":    "Amount (Rial)",
	"assets_amt_mg":      "Gold (milligrams)",
	"assets_ph_wallet":   "e.g. wallet",
	"assets_ph_physical": "e.g. physical",
	"assets_btn_rial":    "Add Rial asset",
	"assets_btn_gold":    "Add gold",
	"assets_lines":       "Lines",
	"assets_th_type":     "Type",
	"assets_th_amount":   "Amount",
	"assets_th_equiv":    "≈ Rial (at rate)",
	"assets_equiv_need_rate": "Refresh the gold price to see Rial equivalent.",
	"assets_no_rows":     "No rows yet.",
	"assets_footer":      "POST /assets/refresh-gold · Talasea API",
	"kind_rial":          "rial",
	"kind_gold":          "gold",
	"rial_unit":          "Rial",
	"word_mg":            "mg",

	"tx_heading":   "Transactions",
	"tx_sub":       "Income and expenses in Rial. Use category and source for reports.",
	"tx_date_hint": "Date in %s",
	"tx_new":       "New",
	"tx_type":      "Type",
	"tx_opt_in":    "Income",
	"tx_opt_out":   "Expense",
	"tx_amount":    "Amount (Rial)",
	"tx_category":  "Category",
	"tx_source":    "Source / account",
	"tx_date":      "Date",
	"tx_note":      "Note",
	"tx_ph_food":   "e.g. food",
	"tx_ph_source": "e.g. card, cash",
	"tx_save":      "Save",
	"tx_recent":    "Recent",
	"tx_th_date":   "Date",
	"tx_th_amt":    "Amount",
	"tx_th_cat":    "Category",
	"tx_pill_in":   "+",
	"tx_pill_out":  "−",
	"tx_empty":     "No transactions yet.",
	"tx_footer":    "Amounts in Rial",

	"rep_heading":     "Reports",
	"rep_month_hint":  "Month groups use the transaction date (%s)",
	"rep_all_time":    "All time",
	"rep_lbl_income":  "Income",
	"rep_lbl_expense": "Expense",
	"rep_lbl_net":     "Net",
	"rep_by_month":    "By month (YYYY-MM)",
	"rep_th_month":    "Month",
	"rep_th_in":       "Income",
	"rep_th_out":      "Expense",
	"rep_th_net":      "Net",
	"rep_no_months":   "No transaction data for monthly breakdown.",
	"rep_by_source":   "By source / account",
	"rep_th_source":   "Source",
	"rep_no_data":     "No data.",
	"rep_by_category": "By category",
	"rep_th_category": "Category",
	"rep_empty":       "(empty)",
	"rep_footer":      "Aggregated from transactions",

	"loans_heading":    "Loans & reminders",
	"loans_attention_1": "1 loan needs your attention",
	"loans_attention_n": "%d loans need your attention",
	"loans_sub_tz":     "Due dates use the calendar in %s. Upcoming: due in the next %d days (after today).",
	"loans_due_soon_title": "Due now",
	"loans_overdue":    "Overdue",
	"loans_due_today":  "Due today",
	"loans_upcoming":   "Upcoming",
	"loans_later":      "Later (beyond horizon)",
	"loans_all_clear":  "You have no open loans yet.",
	"loans_empty_hint":  "Add your first loan to get due-date reminders when you open this page.",
	"loans_cta_add":     "Add a loan",
	"loans_add_new":    "Add loan",
	"loans_lbl_label":  "Name / who",
	"loans_lbl_amount": "Amount (Rial, outstanding)",
	"loans_lbl_due":    "Due date",
	"loans_lbl_note":   "Note (optional)",
	"loans_add_note_op": "Optional note",
	"loans_ph_label":   "e.g. bank, person",
	"loans_ph_note":    "e.g. account number",
	"loans_due_help":   "The due date is compared to today in your display time zone when you open this page.",
	"loans_btn_add":    "Add loan",
	"loans_th_label":   "Name",
	"loans_th_amount":  "Amount",
	"loans_th_due":     "Due",
	"loans_th_rel":     "When",
	"loans_th_actions": "Actions",
	"loans_th_note":    "Note",
	"loans_btn_settle": "Mark repaid",
	"loans_btn_delete": "Delete",
	"loans_js_delete":  "Delete this loan?",
	"loans_settled":    "Settled (recent)",
	"loans_pill_overdue": "Overdue",
	"loans_pill_today": "Due today",
	"loans_msg_settled": "Marked as repaid.",
	"loans_msg_deleted": "Deleted.",
	"loans_footer":     "Reminders on page load. SMS and other channels can be added later.",
	"loans_rel_today":  "Today",
	"loans_rel_tomorrow": "Tomorrow",
	"loans_rel_1d_late": "1 day late",
	"loans_rel_nd_late": "%d days late",
	"loans_rel_in_nd":  "in %d days",

	"login_heading":  "Sign in",
	"login_sub":      "Use the server password to continue.",
	"login_password": "Password",
	"login_submit":   "Sign in",
	"login_footer":   "Set credentials via env (see env.example in the repo).",

	"gold_no_fetch": "No price cached yet.",
}

// fa: Persian strings. Missing keys fall back to en in Dict.
var fa = map[string]string{
	"brand": "حسابداری",

	"nav_home":    "خانه",
	"nav_assets":  "دارایی",
	"nav_txns":    "تراکنش",
	"nav_reports": "گزارش",
	"nav_loans":   "وام/قرض",
	"nav_logout":  "خروج",
	"lang_en":     "EN",
	"lang_fa":     "فا",

	"title_home":         "خانه",
	"title_assets":       "دارایی",
	"title_transactions": "تراکنش‌ها",
	"title_reports":      "گزارش",
	"title_loans":        "وام",
	"title_login":        "ورود",
	"meta_title_suffix":  "حسابداری",

	"home_heading":   "خلاصه",
	"home_sub_times": "نمایش زمان در %s · ذخیره UTC",
	"home_cash":      "نقد (ریال)",
	"home_cash_sub":  "ریال در سطرها",
	"home_gold":      "طلا",
	"home_gold_sub":  "میلی‌گرم · ≈ ‌%s ریال",
	"home_total":     "جمع (معادل)",
	"home_total_sub": "آخرین واکشی قیمت طلا: %s",
	"home_income":    "درآمد (همه‌زمانه)",
	"home_expense":   "هزینه (همه‌زمانه)",
	"home_net":       "خالص (درآمد − هزینه)",
	"home_footer":    "ریال + طلا (طلاسی) · SQLite محلی",
	"home_quote_yes": "ارزش طلا بر اساس آخرین نرخ واکشی شده (تنظیمات در دارایی).",
	"home_quote_no":  "قیمت طلا واکشی نشده — «دارایی» را باز کرده و نرخ را بروز کنید.",

	"assets_heading":     "دارایی",
	"assets_price_unit":  "قیمت API: تومان به‌ازای هر میلی‌گرم طلا (طلاسی)؛ ریال ≈ mg × تومان/mg × ۱۰.",
	"assets_clock":       "ساعت: %s",
	"assets_gold_market": "بازار طلا (طلاسی)",
	"assets_price_sub":   "تومان / mg (کش)",
	"assets_24h":         "۲۴س: %s · واکشی %s",
	"assets_no_price":    "نرخی ذخیره نشده. دکمه زیر را بزنید.",
	"assets_refresh":     "بروزرسانی نرخ",
	"assets_totals":      "جمع",
	"assets_lbl_cash":    "نقد",
	"assets_lbl_gold":    "طلا",
	"assets_lbl_goldval": "ارزش طلا",
	"assets_add_rial":    "افزودن ریال",
	"assets_add_gold":    "افزودن طلا",
	"assets_label":       "عنوان",
	"assets_amt_rial":    "مبلغ (ریال)",
	"assets_amt_mg":      "طلا (میلی‌گرم)",
	"assets_ph_wallet":   "مثلاً کیف",
	"assets_ph_physical": "مثلاً فیزیکی",
	"assets_btn_rial":    "ثبت ریال",
	"assets_btn_gold":    "ثبت طلا",
	"assets_lines":       "سطرها",
	"assets_th_type":     "نوع",
	"assets_th_amount":   "مقدار",
	"assets_th_equiv":    "≈ ریال (با نرخ)",
	"assets_equiv_need_rate": "برای معادل ریال، نرخ طلا را بروز کنید.",
	"assets_no_rows":     "هنوز سطری نیست.",
	"assets_footer":      "POST /assets/refresh-gold · API طلاسی",
	"kind_rial":          "ریال",
	"kind_gold":          "طلا",
	"rial_unit":          "ریال",
	"word_mg":            "mg",

	"tx_heading":   "تراکنش‌ها",
	"tx_sub":       "درآمد و هزینه به ریال. برای گزارش‌ها دسته و منبع را پر کنید.",
	"tx_date_hint": "تاریخ در %s",
	"tx_new":       "جدید",
	"tx_type":      "نوع",
	"tx_opt_in":    "درآمد",
	"tx_opt_out":   "هزینه",
	"tx_amount":    "مبلغ (ریال)",
	"tx_category":  "دسته",
	"tx_source":    "منبع / حساب",
	"tx_date":      "تاریخ",
	"tx_note":      "یادداشت",
	"tx_ph_food":   "مثلاً خوراک",
	"tx_ph_source": "مثلاً کارت، نقد",
	"tx_save":      "ذخیره",
	"tx_recent":    "اخیر",
	"tx_th_date":   "تاریخ",
	"tx_th_amt":    "مبلغ",
	"tx_th_cat":    "دسته",
	"tx_pill_in":   "+",
	"tx_pill_out":  "−",
	"tx_empty":     "هنوز تراکنشی نیست.",
	"tx_footer":    "مبالغ به ریال",

	"rep_heading":     "گزارش",
	"rep_month_hint":  "گروه ماهیانه بر اساس تاریخ تراکنش (%s) است",
	"rep_all_time":    "همه‌زمانه",
	"rep_lbl_income":  "درآمد",
	"rep_lbl_expense": "هزینه",
	"rep_lbl_net":     "خالص",
	"rep_by_month":    "به تفکیک ماه (YYYY-MM)",
	"rep_th_month":    "ماه",
	"rep_th_in":       "درآمد",
	"rep_th_out":      "هزینه",
	"rep_th_net":      "خالص",
	"rep_no_months":   "داده‌ای برای تفکیک ماهیانه نیست.",
	"rep_by_source":   "به تفکیک منبع / حساب",
	"rep_th_source":   "منبع",
	"rep_no_data":     "داده‌ای نیست.",
	"rep_by_category": "به تفکیک دسته",
	"rep_th_category": "دسته",
	"rep_empty":       "(خالی)",
	"rep_footer":      "تجمیع از جدول تراکنش‌ها",

	"loans_heading":    "وام و یادآور",
	"loans_attention_1": "۱ وام نیاز به توجه دارد",
	"loans_attention_n": "%d وام نیاز به توجه دارد",
	"loans_sub_tz":     "تاریخ سررسید بر اساس تقویم و منطقه‌ی %s. «نزدیک»: %d روز پس از امروز.",
	"loans_due_soon_title": "سررسید",
	"loans_overdue":    "عقب‌افتاده",
	"loans_due_today":  "سررسید امروز",
	"loans_upcoming":   "نزدیک",
	"loans_later":      "بعدی (بعد از افق)",
	"loans_all_clear":  "هنوز وام بازی ثبت نکرده‌اید.",
	"loans_empty_hint":  "اولین وام را اضافه کنید تا یادآور سررسید ببینید.",
	"loans_cta_add":     "افزودن وام",
	"loans_add_new":    "افزودن وام",
	"loans_lbl_label":  "عنوان / طرف",
	"loans_lbl_amount": "مبلغ (ریال، مانده)",
	"loans_lbl_due":    "تاریخ سررسید",
	"loans_lbl_note":   "یادداشت (اختیاری)",
	"loans_add_note_op": "یادداشت اختیاری",
	"loans_ph_label":   "مثلاً بانک، شخص",
	"loans_ph_note":    "مثلاً شماره قرارداد",
	"loans_due_help":   "هنگام باز کردن صفحه، سررسید با «امروز» در منطقه‌ی نمایش شما مقایسه می‌شود.",
	"loans_btn_add":    "ثبت وام",
	"loans_th_label":   "عنوان",
	"loans_th_amount":  "مبلغ",
	"loans_th_due":     "سررسید",
	"loans_th_rel":     "چه‌وقت",
	"loans_th_actions": "اقدام",
	"loans_th_note":    "یادداشت",
	"loans_btn_settle": "تسویه شد",
	"loans_btn_delete": "حذف",
	"loans_js_delete":  "این مورد حذف شود؟",
	"loans_settled":    "تسویه‌شده (اخیر)",
	"loans_pill_overdue": "عقب‌افتاده",
	"loans_pill_today": "امروز",
	"loans_msg_settled": "به‌عنوان تسویه ثبت شد.",
	"loans_msg_deleted": "حذف شد.",
	"loans_footer":     "یادآور هنگام بارگذاری صفحه. پیامک و بقیه بعداً.",
	"loans_rel_today":  "امروز",
	"loans_rel_tomorrow": "فردا",
	"loans_rel_1d_late": "۱ روز تأخیر",
	"loans_rel_nd_late": "%d روز تأخیر",
	"loans_rel_in_nd":  "تا %d روز دیگر",

	"login_heading":  "ورود",
	"login_sub":      "رمز سرور را وارد کنید.",
	"login_password": "رمز",
	"login_submit":   "ورود",
	"login_footer":   "مقادیر را از env تنظیم کنید (نمونه: env.example در مخزن).",

	"gold_no_fetch": "نرخی ذخیره نشده.",
}

// Format applies fmt.Sprintf to T(lang, key) with the given args.
func Format(lang, key string, a ...any) string {
	return fmt.Sprintf(T(lang, key), a...)
}
