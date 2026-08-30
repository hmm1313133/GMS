package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"GMS/internal/config"
	"GMS/internal/database"
	"GMS/internal/login"
)

// gms is the single-binary entry point (Java gui.ZEVMS -> cmd/gms).
// Roles (login/channel/cashshop) are enabled by config; P1 smoke wires only
// the login listener.
func main() {
	cfgPath := "configs/gms.toml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		// config file optional for smoke tests; defaults apply
		cfg = config.Defaults()
	}

	lg := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(lg)

	ls := login.New(cfg.Login, lg)
	// P2.3: world/channel-list wiring (Java LoginServer statics).
	ls.SetWorlds(login.WorldConfigFrom(cfg))

	// P2.2: account DB for the login flow. When the DB is unreachable we
	// still serve the P1 smoke path (hello/PING) and LOGIN_PASSWORD answers
	// "account not found" (loginok=5) instead of crashing the port.
	db, err := database.Open(cfg.Database)
	if err != nil {
		lg.Warn("database unavailable, login auth degraded", "err", err)
	} else {
		defer db.Close()
		ls.SetStore(db)
		lg.Info("database connected")
	}

	if err := ls.Start(); err != nil {
		lg.Error("login server failed", "err", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	lg.Info("shutting down")
	ls.Stop()
}
