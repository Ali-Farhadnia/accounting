package app

import (
	"database/sql"
	"html/template"
	"strconv"
	"strings"
	"time"
)

// Ctx is template context helper.
type Ctx struct {
	Title string
	Tab   string
}

// TmplFuncs returns template helpers bound to a display location (all stored times are UTC).
func TmplFuncs(loc *time.Location) template.FuncMap {
	if loc == nil {
		loc = time.UTC
	}
	return template.FuncMap{
		"formatRial":  formatRial,
		"formatMg":    formatMilligrams,
		"formatRialN": nullRial,
		"formatMgN":   nullMilligram,
		"add":         func(a, b int64) int64 { return a + b },
		"sub":         func(a, b int64) int64 { return a - b },
		"isIncome":    func(k string) bool { return k == "income" },
		"isExpense":   func(k string) bool { return k == "expense" },
		"ts":          func() int64 { return time.Now().Unix() },
		"localTime": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.In(loc).Format("2006-01-02 15:04")
		},
		"localDate": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.In(loc).Format("2006-01-02")
		},
		"localToday": func() string {
			return time.Now().In(loc).Format("2006-01-02")
		},
	}
}

func formatRial(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var out string
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(c)
	}
	if neg {
		return "−" + out
	}
	return out
}

// formatMilligrams formats milligrams with grouping and up to 2 decimal places.
func formatMilligrams(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	parts := strings.SplitN(s, ".", 2)
	intStr := parts[0]
	if intStr == "" {
		intStr = "0"
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	sign := ""
	if neg {
		sign = "−"
	}
	var b strings.Builder
	b.WriteString(sign)
	for k, c := range intStr {
		if k > 0 && (len(intStr)-k)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if frac != "" {
		b.WriteByte('.')
		b.WriteString(frac)
	}
	return b.String()
}

func nullRial(n sql.NullInt64) string {
	if !n.Valid {
		return "—"
	}
	return formatRial(n.Int64)
}

func nullMilligram(n sql.NullFloat64) string {
	if !n.Valid {
		return "—"
	}
	return formatMilligrams(n.Float64)
}
