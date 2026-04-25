-- SQLite schema for personal accounting
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS assets (
  id              TEXT PRIMARY KEY,
  kind            TEXT NOT NULL CHECK (kind IN ('rial', 'gold')),
  label           TEXT NOT NULL DEFAULT '',
  amount_rial     INTEGER,
  gold_mg         REAL, -- milligrams (legacy column gold_grams may exist until migrated)
  created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS transactions (
  id              TEXT PRIMARY KEY,
  kind            TEXT NOT NULL CHECK (kind IN ('income', 'expense')),
  amount_rial     INTEGER NOT NULL,
  category        TEXT NOT NULL DEFAULT '',
  source          TEXT NOT NULL DEFAULT '',
  note            TEXT NOT NULL DEFAULT '',
  occurred_on     TEXT NOT NULL, -- YYYY-MM-DD
  created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_txn_occurred ON transactions(occurred_on);
CREATE INDEX IF NOT EXISTS idx_txn_kind ON transactions(kind);

-- Single row cache for latest Talasea quote
CREATE TABLE IF NOT EXISTS gold_price_cache (
  id                  INTEGER PRIMARY KEY CHECK (id = 1),
  price_rial          REAL NOT NULL, -- Toman per milligram (Talasea; v3 undoes mistaken v2 ×1000)
  price_is_per_0_1g   INTEGER NOT NULL DEFAULT 0, -- legacy: ignored; new rows use 0
  change_24h          TEXT NOT NULL DEFAULT '',
  fetched_at          TEXT NOT NULL
);

-- Personal loans: due_date is a calendar day (use next payment or maturity) in the UI time zone
CREATE TABLE IF NOT EXISTS loans (
  id            TEXT PRIMARY KEY,
  label         TEXT NOT NULL DEFAULT '',
  amount_rial   INTEGER NOT NULL,
  due_date      TEXT NOT NULL, -- YYYY-MM-DD
  note          TEXT NOT NULL DEFAULT '',
  settled       INTEGER NOT NULL DEFAULT 0 CHECK (settled IN (0, 1)),
  created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_loans_due ON loans(due_date);
CREATE INDEX IF NOT EXISTS idx_loans_settled_due ON loans(settled, due_date);
