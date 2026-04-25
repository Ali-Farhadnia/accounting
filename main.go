package main

import (
	"accounting/internal/auth"
	"accounting/internal/config"
	"accounting/internal/db"
	"accounting/internal/web"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	var flagListen, flagDB string
	flag.StringVar(&flagListen, "listen", "", "override $"+config.EnvHTTPAddr)
	flag.StringVar(&flagDB, "db", "", "override $"+config.EnvDB)
	flag.Parse()

	cfg := config.Load()
	if flagListen != "" {
		cfg.HTTPAddr = flagListen
	}
	if flagDB != "" {
		cfg.DBPath = flagDB
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o750); err != nil {
		log.Fatal(err)
	}
	sqldb, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	g := auth.New(cfg.Password, cfg.AuthKey)
	h := &web.Handler{
		DB: sqldb, Auth: g, Display: cfg.DisplayLoc, GoldAPIURL: cfg.GoldAPIURL, DefaultLang: cfg.DefaultLang,
		GoldRialPerToman: cfg.GoldRialPerToman,
		GoldPriceScale:   cfg.GoldPriceScale,
	}

	log.Printf("config: %s=%q %s=%q %s=%q %s=%q", config.EnvHTTPAddr, cfg.HTTPAddr, config.EnvDB, cfg.DBPath, config.EnvTZ, cfg.Timezone, config.EnvLang, cfg.DefaultLang)
	log.Printf("data times stored in UTC; UI uses time zone %s", cfg.DisplayLoc.String())
	if cfg.GoldAPIURL != "" {
		log.Printf("gold price API override: %s", cfg.GoldAPIURL)
	}
	if cfg.GoldRialPerToman != 10 {
		log.Printf("gold: %s=%g (Rial per Toman for spot)", config.EnvGoldRialPerToman, cfg.GoldRialPerToman)
	}
	if cfg.GoldPriceScale != 1 {
		log.Printf("gold: %s=%g (extra factor on spot)", config.EnvGoldPriceScale, cfg.GoldPriceScale)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", http.HandlerFunc(h.Home))
	mux.Handle("GET /assets", http.HandlerFunc(h.AssetsPage))
	mux.Handle("POST /assets/rial", http.HandlerFunc(h.AddAssetRial))
	mux.Handle("POST /assets/gold", http.HandlerFunc(h.AddAssetGold))
	mux.Handle("POST /assets/refresh-gold", http.HandlerFunc(h.RefreshGold))
	mux.Handle("GET /transactions", http.HandlerFunc(h.TxnsPage))
	mux.Handle("POST /transactions", http.HandlerFunc(h.AddTxn))
	mux.Handle("GET /reports", http.HandlerFunc(h.ReportsPage))
	mux.Handle("GET /loans", http.HandlerFunc(h.LoansPage))
	mux.Handle("POST /loans", http.HandlerFunc(h.AddLoan))
	mux.Handle("POST /loans/settle", http.HandlerFunc(h.SettleLoan))
	mux.Handle("POST /loans/delete", http.HandlerFunc(h.DeleteLoan))
	mux.Handle("GET /static/", h.Static())
	mux.Handle("GET /login", http.HandlerFunc(h.LoginGet))
	mux.Handle("POST /login", http.HandlerFunc(h.LoginPost))
	mux.Handle("GET /logout", http.HandlerFunc(h.Logout))
	mux.Handle("GET /lang", http.HandlerFunc(h.SetLang))

	log.Printf("listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, g.Wrap(mux)); err != nil {
		log.Fatal(err)
	}
}
