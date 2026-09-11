package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"GMS/internal/channel"
	"GMS/internal/config"
	"GMS/internal/database"
	"GMS/internal/login"
	"GMS/internal/wzs"
	"GMS/internal/world"
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
	// P4.1: login -> channel handoff wiring. The login-ticket registry is the
	// Java LoginServer.loginAuth static map (CHAR_SELECT writes it, the
	// channel session reads it); external_ip is Java MapleParty.IP地址.
	registry := world.NewLoginRegistry()
	ls.SetRegistry(registry)
	if err := ls.SetExternalIP(cfg.Server.ExternalIP); err != nil {
		lg.Warn("invalid server.external_ip, falling back to loopback", "err", err, "ip", cfg.Server.ExternalIP)
		_ = ls.SetExternalIP("127.0.0.1")
	}

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

		// P3.5: drop tables (Java server.life.MapleMonsterInformationProvider
		// reads them). Only the counts are logged here - the caching provider
		// (internal/life) is wired into the kill/drop path in P4.3. The rows
		// come from the 079MAX2 database, not from the wz files
		// (docs/SESSION_STATE.md §P3.5).
		mobs, rows, globals, derr := db.DropStats(context.Background())
		if derr != nil {
			lg.Warn("drop tables unreadable", "err", derr)
		} else {
			lg.Info("drop tables loaded", "rows", rows, "monsters", mobs, "global", globals)
		}
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

	// P4.1: channel servers (Java ChannelServer.newInstance(1..N), ports
	// 7574+channel). They share the single process, so "registering to the
	// world" is direct wiring: port lookup for CHAR_SELECT + load reporting
	// for SERVERLIST (Java LoginServer.addChannel / LoginWorker).
	chs := channel.New(channel.ConfigFrom(cfg), lg)
	// P4.3b: Map.wz map data for the channel maps (Java MapleMapFactory's
	// static source provider, one per channel). A nil root - the wz-degraded
	// startup above - only warns: maps then load as bare instances.
	chs.SetWZ(wzRoot)
	// P4.2: character rows for PLAYER_LOGGEDIN (Java loadCharFromDB). With the
	// DB down the field session still gets hello/PING but PLAYER_LOGGEDIN
	// closes the connection (mirrors the login-degraded mode).
	if db != nil {
		chs.SetStore(db)
		// P4.5b: the ZEVMS admin-console switches (Java
		// gui/Start.GetConfigValues -> Start.ConfigValuesMap, loaded once from
		// Start.startServer). Read once here, like Java; an empty or unreadable
		// table leaves every switch at 0 = feature on, so a DB hiccup can never
		// switch gameplay off.
		chs.SetConfigValues(db)
		if n, cerr := chs.ReloadConfigValues(context.Background()); cerr != nil {
			lg.Warn("configvalues unreadable, all switches stay on", "err", cerr)
		} else {
			lg.Info("configvalues loaded", "switches", n)
		}
	}
	ls.SetChannelPortLookup(func(ch int) (int, bool) {
		cs := chs.Channel(ch)
		if cs == nil {
			return 0, false
		}
		return cs.Port(), true
	})
	chs.SetLoadReporter(ls.SetChannelLoad)
	if err := chs.Start(); err != nil {
		lg.Error("channel server failed", "err", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	lg.Info("shutting down")
	chs.Stop()
	ls.Stop()
}
