// Package config loads application settings from the environment.
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variable names (prefix ACCOUNTING_ for all app settings).
const (
	EnvHTTPAddr = "ACCOUNTING_HTTP_ADDR" // e.g. :8080
	EnvDB       = "ACCOUNTING_DB"        // SQLite file path
	EnvPassword = "ACCOUNTING_PASSWORD"  // web login password
	EnvAuthKey  = "ACCOUNTING_AUTH_KEY"  // session cookie HMAC key
	EnvTZ       = "ACCOUNTING_TZ"        // IANA time zone for UI
	// EnvGoldAPIURL is optional; empty uses the built-in Talasea default.
	EnvGoldAPIURL = "ACCOUNTING_GOLD_API_URL"
	// EnvLang is the default UI language when no cookie: en or fa.
	EnvLang = "ACCOUNTING_LANG"
	// EnvGoldRialPerToman: Rial per one Toman (Iran default: 10).
	EnvGoldRialPerToman = "ACCOUNTING_GOLD_RIAL_PER_TOMAN"
	// EnvGoldPriceScale: optional extra multiplier on mg × toman/mg × rial per toman.
	EnvGoldPriceScale = "ACCOUNTING_GOLD_PRICE_SCALE"
	// EnvGoldGramRialScale is deprecated: same as EnvGoldPriceScale if EnvGoldPriceScale is unset.
	EnvGoldGramRialScale = "ACCOUNTING_GOLD_GRAM_RIAL_SCALE"
)

// Defaults when env vars are empty (not for production secrets).
const (
	DefaultHTTPAddr = ":8080"
	DefaultDB       = "data/app.db"
	DefaultPassword = "changeme"
	DefaultAuthKey  = "insecure-dev-auth-key"
	DefaultTZ   = "Asia/Tehran"
	DefaultLang = "fa"
)

// Config holds resolved settings.
type Config struct {
	HTTPAddr   string
	DBPath     string
	Password   string
	AuthKey    string
	Timezone   string
	DisplayLoc *time.Location
	GoldAPIURL         string
	GoldRialPerToman   float64 // Rial per Toman; default 10
	GoldPriceScale     float64 // optional extra factor; default 1
	DefaultLang        string  // "en" or "fa" — default when acct_lang cookie is absent
}

// Load reads configuration from the environment. Unknown or invalid
// time zones fall back to DefaultTZ, then UTC.
func Load() *Config {
	c := &Config{
		HTTPAddr:   firstNonEmpty(os.Getenv(EnvHTTPAddr), DefaultHTTPAddr),
		DBPath:     firstNonEmpty(os.Getenv(EnvDB), DefaultDB),
		Timezone:   firstNonEmpty(strings.TrimSpace(os.Getenv(EnvTZ)), DefaultTZ),
		GoldAPIURL: strings.TrimSpace(os.Getenv(EnvGoldAPIURL)),
	}
	if p := strings.TrimSpace(os.Getenv(EnvPassword)); p != "" {
		c.Password = p
	} else {
		c.Password = DefaultPassword
		log.Printf("warning: %s is not set, using default password %q (set it in production)", EnvPassword, DefaultPassword)
	}
	if k := strings.TrimSpace(os.Getenv(EnvAuthKey)); k != "" {
		c.AuthKey = k
	} else {
		c.AuthKey = DefaultAuthKey
		log.Printf("warning: %s is not set, using a development default (set a long random value in production)", EnvAuthKey)
	}

	c.DisplayLoc = loadLocation(c.Timezone)
	c.GoldRialPerToman = loadFloatEnv(EnvGoldRialPerToman, 10.0, "Rial per Toman for gold")
	if c.GoldRialPerToman <= 0 {
		log.Printf("warning: %s must be > 0, using 10.0", EnvGoldRialPerToman)
		c.GoldRialPerToman = 10.0
	}
	c.GoldPriceScale = 1.0
	if strings.TrimSpace(os.Getenv(EnvGoldPriceScale)) != "" {
		c.GoldPriceScale = loadFloatEnv(EnvGoldPriceScale, 1.0, "gold price extra scale")
	} else if strings.TrimSpace(os.Getenv(EnvGoldGramRialScale)) != "" {
		c.GoldPriceScale = loadFloatEnv(EnvGoldGramRialScale, 1.0, "gold price extra scale")
	}
	if c.GoldPriceScale <= 0 {
		c.GoldPriceScale = 1.0
	}
	c.DefaultLang = loadDefaultLang()
	return c
}

func loadDefaultLang() string {
	s := strings.ToLower(strings.TrimSpace(os.Getenv(EnvLang)))
	switch s {
	case "", "fa", "farsi":
		return "fa"
	case "en", "eng", "english":
		return "en"
	default:
		log.Printf("warning: %s=%q is not en|fa, using %q", EnvLang, s, DefaultLang)
		return DefaultLang
	}
}

func loadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("warning: %s=%q invalid (%v), trying default %q", EnvTZ, name, err, DefaultTZ)
		loc, err = time.LoadLocation(DefaultTZ)
		if err != nil {
			log.Printf("warning: could not load %q, using UTC", DefaultTZ)
			return time.UTC
		}
	}
	return loc
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return b
}

// loadFloatEnv parses an optional positive float; empty uses def; invalid logs and uses def.
func loadFloatEnv(name string, def float64, what string) float64 {
	s := strings.TrimSpace(os.Getenv(name))
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("warning: %s is not a valid %s, using %g (%v)", name, what, def, err)
		return def
	}
	return v
}
