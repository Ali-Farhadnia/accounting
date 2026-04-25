package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// parseDBTime parses timestamps stored in UTC (RFC3339) or legacy SQLite "YYYY-MM-DD HH:MM:SS".
func parseDBTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time: %q", s)
}

//go:embed schema.sql
var schemaFS embed.FS

// assetsGoldSelectExpr is a column expression (milligrams) for reading legacy and new rows; set in migrate.
var assetsGoldSelectExpr = "0"

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)
	if err := migrate(sqldb); err != nil {
		_ = sqldb.Close()
		return nil, err
	}
	return sqldb, nil
}

func migrate(sqldb *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := sqldb.Exec(string(schema)); err != nil {
		return err
	}
	var v int
	if err := sqldb.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v < 1 {
		if err := migrateToMilligramsV1(sqldb); err != nil {
			return err
		}
		if _, err := sqldb.Exec("PRAGMA user_version = 1"); err != nil {
			return err
		}
		v = 1
	}
	if v < 2 {
		if err := migrateRialPerGramV2(sqldb); err != nil {
			return err
		}
		if _, err := sqldb.Exec("PRAGMA user_version = 2"); err != nil {
			return err
		}
		v = 2
	}
	if v < 3 {
		if err := migrateTomanPerMgV3(sqldb); err != nil {
			return err
		}
		if _, err := sqldb.Exec("PRAGMA user_version = 3"); err != nil {
			return err
		}
	}
	if err := refreshAssetsGoldExpr(sqldb); err != nil {
		return err
	}
	return nil
}

func tableColumnSet(sqldb *sql.DB, table string) (map[string]bool, error) {
	allowed := map[string]struct{}{
		"assets":           {},
		"gold_price_cache": {},
	}
	if _, ok := allowed[table]; !ok {
		return nil, fmt.Errorf("table not allowed: %q", table)
	}
	rows, err := sqldb.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func refreshAssetsGoldExpr(sqldb *sql.DB) error {
	c, err := tableColumnSet(sqldb, "assets")
	if err != nil {
		return err
	}
	_, hasG := c["gold_grams"]
	_, hasM := c["gold_mg"]
	switch {
	case hasM && hasG:
		assetsGoldSelectExpr = "COALESCE(gold_mg, gold_grams * 1000.0)"
	case hasM:
		assetsGoldSelectExpr = "gold_mg"
	case hasG:
		assetsGoldSelectExpr = "gold_grams * 1000.0"
	default:
		assetsGoldSelectExpr = "0"
	}
	return nil
}

// migrateToMilligramsV1 backfills gold_mg from gold_grams and normalizes cached price to rial / mg.
func migrateToMilligramsV1(sqldb *sql.DB) error {
	ac, err := tableColumnSet(sqldb, "assets")
	if err != nil {
		return err
	}
	if ac["gold_grams"] {
		if !ac["gold_mg"] {
			if _, err := sqldb.Exec(`ALTER TABLE assets ADD COLUMN gold_mg REAL`); err != nil {
				return err
			}
		}
		if _, err := sqldb.Exec(
			`UPDATE assets SET gold_mg = gold_grams * 1000.0 WHERE kind = 'gold' AND gold_grams IS NOT NULL`); err != nil {
			return err
		}
	} else if !ac["gold_mg"] {
		if _, err := sqldb.Exec(`ALTER TABLE assets ADD COLUMN gold_mg REAL`); err != nil {
			return err
		}
	}
	pc, err := tableColumnSet(sqldb, "gold_price_cache")
	if err != nil {
		return err
	}
	if !pc["price_is_per_0_1g"] {
		return nil
	}
	var p float64
	var flag int
	err = sqldb.QueryRow(`SELECT price_rial, price_is_per_0_1g FROM gold_price_cache WHERE id = 1`).Scan(&p, &flag)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var newP float64
	if flag != 0 {
		newP = p / 100.0
	} else {
		newP = p / 1000.0
	}
	_, err = sqldb.Exec(
		`UPDATE gold_price_cache SET price_rial = ?, price_is_per_0_1g = 0 WHERE id = 1`,
		newP,
	)
	return err
}

// migrateRialPerGramV2 rewrites a v1 price cache: v1 stored "rial per milligram" (gram price / 1000).
// Valuation is now (milligrams/1000) × rial per gram, so the cache should hold rial per gram.
func migrateRialPerGramV2(sqldb *sql.DB) error {
	_, err := sqldb.Exec(
		`UPDATE gold_price_cache SET price_rial = price_rial * 1000.0 WHERE id = 1`,
	)
	return err
}

// migrateTomanPerMgV3 undoes v2: the live API is Toman per milligram, not Rial per gram; v2's ×1000
// was a mistaken conversion. Divide so the cache again matches the API string after refresh.
func migrateTomanPerMgV3(sqldb *sql.DB) error {
	_, err := sqldb.Exec(
		`UPDATE gold_price_cache SET price_rial = price_rial / 1000.0 WHERE id = 1`,
	)
	return err
}

// AssetType matches DB enum.
const (
	AssetRial = "rial"
	AssetGold = "gold"
)

// TxnType for transactions.
const (
	TxnIncome  = "income"
	TxnExpense = "expense"
)

type Asset struct {
	ID            string
	Kind          string
	Label         string
	AmountRial    sql.NullInt64
	GoldMilligrams sql.NullFloat64
	CreatedAt     time.Time
}

type Transaction struct {
	ID         string
	Kind       string
	AmountRial int64
	Category   string
	Source     string
	Note       string
	OccurredOn time.Time
	CreatedAt  time.Time
}

// GoldPriceRow: column price_rial holds the API spot as Toman per milligram (Talasea getGoldPrice `price`).
type GoldPriceRow struct {
	PriceRial     float64 // DB column price_rial; Toman per mg
	FetchedAt     time.Time
	RawChange24h  string
}

// CreateAssetRial adds a rial balance line (can be used for multiple "wallets" via label).
func CreateAssetRial(sqldb *sql.DB, label string, amountRial int64) (string, error) {
	id := newID()
	_, err := sqldb.Exec(
		`INSERT INTO assets (id, kind, label, amount_rial, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, AssetRial, label, amountRial, time.Now().UTC().Format(time.RFC3339),
	)
	return id, err
}

// CreateAssetGold stores gold in milligrams.
func CreateAssetGold(sqldb *sql.DB, label string, goldMg float64) (string, error) {
	id := newID()
	_, err := sqldb.Exec(
		`INSERT INTO assets (id, kind, label, gold_mg, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, AssetGold, label, goldMg, time.Now().UTC().Format(time.RFC3339),
	)
	return id, err
}

func ListAssets(sqldb *sql.DB) ([]Asset, error) {
	q := fmt.Sprintf(
		`SELECT id, kind, label, amount_rial, %s AS gold_mg, created_at FROM assets ORDER BY created_at`,
		assetsGoldSelectExpr,
	)
	rows, err := sqldb.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Asset
	for rows.Next() {
		var a Asset
		var created string
		if err := rows.Scan(&a.ID, &a.Kind, &a.Label, &a.AmountRial, &a.GoldMilligrams, &created); err != nil {
			return nil, err
		}
		var err error
		a.CreatedAt, err = parseDBTime(created)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SumRialAssets returns total rial in asset rows of kind rial.
func SumRialAssets(sqldb *sql.DB) (int64, error) {
	var s sql.NullInt64
	err := sqldb.QueryRow(
		`SELECT COALESCE(SUM(amount_rial), 0) FROM assets WHERE kind = ?`,
		AssetRial,
	).Scan(&s)
	if err != nil {
		return 0, err
	}
	if !s.Valid {
		return 0, nil
	}
	return s.Int64, nil
}

// SumGoldMilligrams is total gold in milligrams.
func SumGoldMilligrams(sqldb *sql.DB) (float64, error) {
	q := fmt.Sprintf(
		`SELECT COALESCE(SUM(%s), 0) FROM assets WHERE kind = ?`,
		assetsGoldSelectExpr,
	)
	var s sql.NullFloat64
	err := sqldb.QueryRow(q, AssetGold).Scan(&s)
	if err != nil {
		return 0, err
	}
	if !s.Valid {
		return 0, nil
	}
	return s.Float64, nil
}

func CreateTransaction(sqldb *sql.DB, kind string, amountRial int64, category, source, note string, occurredOn time.Time) (string, error) {
	id := newID()
	_, err := sqldb.Exec(
		`INSERT INTO transactions (id, kind, amount_rial, category, source, note, occurred_on, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, kind, amountRial, category, source, note, occurredOn.Format("2006-01-02"), time.Now().UTC().Format(time.RFC3339),
	)
	return id, err
}

func ListRecentTransactions(sqldb *sql.DB, limit int) ([]Transaction, error) {
	rows, err := sqldb.Query(
		`SELECT id, kind, amount_rial, category, source, note, occurred_on, created_at
		 FROM transactions ORDER BY occurred_on DESC, created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransactions(rows)
}

func ListTransactionsInRange(sqldb *sql.DB, from, to time.Time) ([]Transaction, error) {
	rows, err := sqldb.Query(
		`SELECT id, kind, amount_rial, category, source, note, occurred_on, created_at
		 FROM transactions WHERE occurred_on >= ? AND occurred_on <= ? ORDER BY occurred_on`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransactions(rows)
}

func scanTransactions(rows *sql.Rows) ([]Transaction, error) {
	var out []Transaction
	for rows.Next() {
		var t Transaction
		var on, created string
		if err := rows.Scan(&t.ID, &t.Kind, &t.AmountRial, &t.Category, &t.Source, &t.Note, &on, &created); err != nil {
			return nil, err
		}
		var err error
		t.OccurredOn, err = time.ParseInLocation("2006-01-02", on, time.UTC)
		if err != nil {
			return nil, err
		}
		t.CreatedAt, err = parseDBTime(created)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MonthTotals: month key "2006-01" -> income, expense
func MonthTotals(sqldb *sql.DB) (map[string]struct{ Income, Expense int64 }, error) {
	rows, err := sqldb.Query(`
		SELECT strftime('%Y-%m', occurred_on) AS m,
			SUM(CASE WHEN kind = 'income' THEN amount_rial ELSE 0 END) AS inc,
			SUM(CASE WHEN kind = 'expense' THEN amount_rial ELSE 0 END) AS exp
		FROM transactions GROUP BY m ORDER BY m DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{ Income, Expense int64 })
	for rows.Next() {
		var m string
		var inc, exp sql.NullInt64
		if err := rows.Scan(&m, &inc, &exp); err != nil {
			return nil, err
		}
		out[m] = struct{ Income, Expense int64 }{
			Income:  nullI64(inc),
			Expense: nullI64(exp),
		}
	}
	return out, rows.Err()
}

// SourceBreakdown: source -> total for filter on income+expense (net by source would be complex; we show signed sum per source for the period)
func SourceTotals(sqldb *sql.DB) ([]struct {
	Source  string
	Income  int64
	Expense int64
}, error) {
	rows, err := sqldb.Query(`
		SELECT COALESCE(source, '') AS s,
			SUM(CASE WHEN kind = 'income' THEN amount_rial ELSE 0 END) AS inc,
			SUM(CASE WHEN kind = 'expense' THEN amount_rial ELSE 0 END) AS exp
		FROM transactions GROUP BY s ORDER BY (inc+exp) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Source  string
		Income  int64
		Expense int64
	}
	for rows.Next() {
		var s string
		var inc, exp sql.NullInt64
		if err := rows.Scan(&s, &inc, &exp); err != nil {
			return nil, err
		}
		out = append(out, struct {
			Source  string
			Income  int64
			Expense int64
		}{s, nullI64(inc), nullI64(exp)})
	}
	return out, rows.Err()
}

func CategoryTotals(sqldb *sql.DB) ([]struct {
	Category string
	Income   int64
	Expense  int64
}, error) {
	rows, err := sqldb.Query(`
		SELECT COALESCE(category, '') AS c,
			SUM(CASE WHEN kind = 'income' THEN amount_rial ELSE 0 END) AS inc,
			SUM(CASE WHEN kind = 'expense' THEN amount_rial ELSE 0 END) AS exp
		FROM transactions GROUP BY c ORDER BY (inc+exp) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Category string
		Income   int64
		Expense  int64
	}
	for rows.Next() {
		var c string
		var inc, exp sql.NullInt64
		if err := rows.Scan(&c, &inc, &exp); err != nil {
			return nil, err
		}
		out = append(out, struct {
			Category string
			Income   int64
			Expense  int64
		}{c, nullI64(inc), nullI64(exp)})
	}
	return out, rows.Err()
}

func TotalIncome(sqldb *sql.DB) (int64, error) {
	var n sql.NullInt64
	err := sqldb.QueryRow(`SELECT COALESCE(SUM(amount_rial),0) FROM transactions WHERE kind = 'income'`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return nullI64(n), nil
}

func TotalExpense(sqldb *sql.DB) (int64, error) {
	var n sql.NullInt64
	err := sqldb.QueryRow(`SELECT COALESCE(SUM(amount_rial),0) FROM transactions WHERE kind = 'expense'`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return nullI64(n), nil
}

// UpsertGoldPrice stores Toman per milligram and 24h change from the API.
func UpsertGoldPrice(sqldb *sql.DB, tomanPerMilligram float64, change24h string) error {
	_, err := sqldb.Exec(
		`INSERT INTO gold_price_cache (id, price_rial, price_is_per_0_1g, change_24h, fetched_at) VALUES (1, ?, 0, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET price_rial=excluded.price_rial, price_is_per_0_1g=0, change_24h=excluded.change_24h, fetched_at=excluded.fetched_at`,
		tomanPerMilligram, change24h, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// Loan is money owed (or lent); due_date is YYYY-MM-DD, interpreted in the UI time zone.
type Loan struct {
	ID         string
	Label      string
	AmountRial int64
	DueDate    string
	Note       string
	Settled    bool
	CreatedAt  time.Time
}

// CreateLoan adds an active (unsettled) loan.
func CreateLoan(sqldb *sql.DB, label string, amountRial int64, dueDateYYYYMMDD, note string) (string, error) {
	id := newID()
	_, err := sqldb.Exec(
		`INSERT INTO loans (id, label, amount_rial, due_date, note, settled, created_at) VALUES (?, ?, ?, ?, ?, 0, ?)`,
		id, label, amountRial, dueDateYYYYMMDD, note, time.Now().UTC().Format(time.RFC3339),
	)
	return id, err
}

// ListActiveLoans returns unsettled loans ordered by due date.
func ListActiveLoans(sqldb *sql.DB) ([]Loan, error) {
	rows, err := sqldb.Query(
		`SELECT id, label, amount_rial, due_date, note, settled, created_at
		 FROM loans WHERE settled = 0 ORDER BY due_date, label`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLoans(rows)
}

// ListSettledLoans returns recent settled loans.
func ListSettledLoans(sqldb *sql.DB, limit int) ([]Loan, error) {
	if limit < 1 {
		limit = 50
	}
	rows, err := sqldb.Query(
		`SELECT id, label, amount_rial, due_date, note, settled, created_at
		 FROM loans WHERE settled = 1 ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLoans(rows)
}

func scanLoans(rows *sql.Rows) ([]Loan, error) {
	var out []Loan
	for rows.Next() {
		var l Loan
		var created string
		var settled int
		if err := rows.Scan(&l.ID, &l.Label, &l.AmountRial, &l.DueDate, &l.Note, &settled, &created); err != nil {
			return nil, err
		}
		l.Settled = settled != 0
		var err error
		l.CreatedAt, err = parseDBTime(created)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SettleLoan marks a loan as repaid. Idempotent.
func SettleLoan(sqldb *sql.DB, id string) error {
	_, err := sqldb.Exec(`UPDATE loans SET settled = 1 WHERE id = ?`, id)
	return err
}

// DeleteLoan removes a row (e.g. mistake).
func DeleteLoan(sqldb *sql.DB, id string) error {
	_, err := sqldb.Exec(`DELETE FROM loans WHERE id = ?`, id)
	return err
}

func GetCachedGoldPrice(sqldb *sql.DB) (GoldPriceRow, error) {
	var r GoldPriceRow
	var fetched string
	err := sqldb.QueryRow(
		`SELECT price_rial, change_24h, fetched_at FROM gold_price_cache WHERE id = 1`,
	).Scan(&r.PriceRial, &r.RawChange24h, &fetched)
	if err == sql.ErrNoRows {
		return r, sql.ErrNoRows
	}
	if err != nil {
		return r, err
	}
	tm, err := time.Parse(time.RFC3339, fetched)
	if err == nil {
		r.FetchedAt = tm.UTC()
	}
	return r, nil
}

func nullI64(n sql.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

func newID() string {
	return uuid.New().String()
}
