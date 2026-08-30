package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"GMS/internal/config"
	"GMS/internal/database"
	"GMS/internal/login"
	"GMS/internal/wzs"
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
	// P2.5: name shown in login popups (Java MapleParty.开服名字).
	ls.SetServerName(cfg.Server.WorldName)

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

	// P3.3: wz data (Java net.sf.odinms.wzpath). A missing/partial export is
	// degraded, not fatal - nothing in the login path needs it yet.
	wzRoot, err := wzs.OpenRoot(cfg.WZ.Path)
	if err != nil {
		lg.Warn("wz data unavailable", "path", cfg.WZ.Path, "err", err)
	} else {
		lg.Info("wz root opened", "path", cfg.WZ.Path)
		if cfg.WZ.LoadNames {
			names, err := wzs.LoadNames(wzRoot)
			if err != nil {
				lg.Warn("wz name tables unavailable", "err", err)
			} else {
				for _, e := range names.Errors {
					lg.Warn("wz name table", "err", e)
				}
				lg.Info("wz names loaded",
					"items", names.Counts.Items,
					"maps", names.Counts.Maps,
					"mobs", names.Counts.Mobs,
					"npcs", names.Counts.NPCs,
					"skills", names.Counts.Skills,
					"forbidden", names.Counts.Forbidden)
			}
		}
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
