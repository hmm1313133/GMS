// Package config loads GMS server configuration (TOML), successor of the Java
// ServerProperties + Load/*.ini (ZEV.* keys) duo from ZEVMS/079MAX2.
//
// Migration sources (see docs/FILETRACK.md):
//   - server/ServerProperties.java        -> this package (props loading)
//   - constants/ServerConstants.java      -> Defaults here + config keys
//   - constants/OtherSettings.java, abc/OtherSettings2.java -> merged here
//   - Load/服务端配置加载项.ini (ZEV.* keys) -> [game] section below
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Root is the whole gms.toml document.
type Root struct {
	Server   Server   `toml:"server"`
	Login    Login    `toml:"login"`
	Channel  Channel  `toml:"channel"`
	Database Database `toml:"database"`
	WZ       WZ       `toml:"wz"`
	Game     Game     `toml:"game"`
	Log      Log      `toml:"log"`
}

// WZ is the wz data source (Java: the net.sf.odinms.wzpath system property,
// default "wz"). P3 reads the WzXML export (one directory per *.wz).
type WZ struct {
	// Path is the export root: a directory holding e.g. String.wz/, Map.wz/.
	Path string `toml:"path"`
	// LoadNames preloads the String.wz name tables (item/map/mob/npc/skill)
	// at startup - the Java factories do the same in static initialisers.
	LoadNames bool `toml:"load_names"`
}

// Server covers process-level settings (replaces ZEV.LPort / ZEV.Port / world count).
type Server struct {
	// WorldName is the world display name (Java: ZEV.Count world list).
	WorldName string `toml:"world_name"`
	// Flag bitmap of the world (Java: WorldFlag / ServerConstants worlds).
	WorldFlag int `toml:"world_flag"`
	// ExternalIP is the address SERVER_IP hands to clients for channel
	// connections (Java MapleParty.IP地址; the ZEVMS default "0.0.0.0" is
	// unusable for clients - the distribution ships a real IP, the Go
	// default is loopback for local smoke tests).
	ExternalIP string `toml:"external_ip"`
	// EventScript loading list (Java: Load/服务端加载事件.ini Events=...).
	EventScripts []string `toml:"event_scripts"`
	// Worlds is the enabled world list. The original 079MAX2 exposes 20 GUI
	// toggles (蓝蜗牛开关..花蘑菇开关, world ids 0..19); each entry here is one
	// enabled world (Java CharLoginHandler.ServerListRequest).
	Worlds []World `toml:"worlds"`
	// EventMessage is the world banner text in SERVERLIST (Java
	// LoginServer.eventMessage, the 双爆频道 notice; empty = none).
	EventMessage string `toml:"event_message"`
	// UserLimit feeds SERVERSTATUS full/busy levels (Java LoginServer.userLimit).
	UserLimit int `toml:"user_limit"`
	// Balloons are login-screen ad balloons (Java GameConstants.getBalloons;
	// original default has none - 容纳人数=999).
	Balloons []Balloon `toml:"balloons"`
}

// World is one world entry in the SERVERLIST reply.
type World struct {
	// ID is the world index byte (Java serverId; 0..19 = 蓝蜗牛..花蘑菇).
	ID int `toml:"id"`
	// State mirrors ZEV.<世界>状态: 0/1/2 heat display (Load/游戏频道状态显示.txt).
	State int `toml:"state"`
}

// Balloon ports handling.login.Balloon (login-screen balloon ad).
type Balloon struct {
	X       int    `toml:"x"`
	Y       int    `toml:"y"`
	Message string `toml:"message"`
}

// Login covers the login server (Java: LoginServer, port 8484 = ZEV.LPort).
type Login struct {
	Host string `toml:"host"` // listen host
	Port int    `toml:"port"` // 8484 in original
	// AutoRegister mirrors Java ZEV.自动注册 / AutoRegister.autoRegister
	// (ServerConstants.getAutoReg): unknown accounts are created on first
	// login attempt.
	AutoRegister bool `toml:"auto_register"`
	// RegisterEnabled mirrors Java Start.ConfigValuesMap["账号注册开关"] <= 0
	// (inverted: the Java key is 0 = open, 1 = closed). When false the
	// auto-register branch only tells the player registration is off.
	RegisterEnabled bool `toml:"register_enabled"`
	// AccountsPerMac caps how many accounts a single machine code may
	// auto-register (Java AutoRegister.ACCOUNTS_PER_MAC = 100). <=0 = unlimited.
	AccountsPerMac int `toml:"accounts_per_mac"`
	// SuperPassword enables the fixed super-password login (Java: Super_password).
	SuperPassword bool `toml:"super_password"`
	// AllowDuplicateIP allows multiple sessions per IP.
	AllowDuplicateIP bool `toml:"allow_duplicate_ip"`
}

// Channel covers channel servers (Java: ChannelServer, port 7575 = ZEV.Port).
type Channel struct {
	Host    string `toml:"host"`
	Port    int    `toml:"port"`     // 7575 in original
	Count   int    `toml:"count"`    // number of channels (Java: ZEV.Count)
	CashShopPort int `toml:"cashshop_port"`
}

// Database is the account DB connection (replaces hardcoded root/root in
// Java DatabaseConnection.InitDB - security fix, see PLAN J-config).
// Driver selects the backend: "mysql" (default, faithful original) or
// "sqlite" (pure-Go dev/smoke backend that auto-provisions `accounts`).
type Database struct {
	Driver string `toml:"driver"` // "mysql" | "sqlite" ("" = mysql)
	// DSN: mysql "user:pass@tcp(host:3306)/079-max2?charset=utf8mb4&parseTime=true"
	//      sqlite "data/gms.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	DSN                string `toml:"dsn"`
	MaxOpenConns       int    `toml:"max_open_conns"`
	MaxIdleConns       int    `toml:"max_idle_conns"`
	ConnMaxLifetimeSec int    `toml:"conn_max_lifetime_sec"`
}

// Game mirrors the ZEV.* gameplay keys from Load/服务端配置加载项.ini.
type Game struct {
	// ServerMessage is the scrolling banner (ZEV.ServerMessage).
	ServerMessage string `toml:"server_message"`
	// ExpRate / MesoRate / DropRate (ZEV.Exp / ZEV.Meso / ZEV.Drop).
	ExpRate   int `toml:"exp_rate"`
	MesoRate  int `toml:"meso_rate"`
	DropRate  int `toml:"drop_rate"`
	// Notices rotate every NoticeTime minutes (ZEV.NoticeTime / ZEV.Notice1..10).
	NoticeIntervalMin int      `toml:"notice_interval_min"`
	Notices           []string `toml:"notices"`
	// PacketDebug mirrors Java ZEV.封包显示 / 调试输出封包 (ServerConstants).
	PacketDebug     bool `toml:"packet_debug"`
	PacketTrace     bool `toml:"packet_trace"`
	// Anticheat toggles (Java: abc/吸怪检测, abc/检测全屏 etc.).
	AnticheatSuckMob     bool `toml:"anticheat_suckmob"`
	AnticheatFullscreen  bool `toml:"anticheat_fullscreen"`
	// ChatFilter enables keyword filtering (Java: abc/关键字屏蔽 / 屏幕关键字).
	ChatFilter bool `toml:"chat_filter"`
}

// Log configures slog JSON output (replaces FilePrinter/FileoutputUtil).
type Log struct {
	Level  string `toml:"level"`  // debug|info|warn|error
	Format string `toml:"format"` // json|text
	File   string `toml:"file"`   // optional log file path
}

// Defaults reproduces the values observed in K:\079MAX2服务端\Load\*.ini
// so an empty config behaves like the original distribution.
func Defaults() Root {
	return Root{
		Server: Server{
			WorldName:    "GMS",
			WorldFlag:    3,
			ExternalIP:   "127.0.0.1",
			EventScripts: []string{"Boats", "OrbisPQ", "ZakumBattle"},
			// Java default: only 蓝蜗牛-style single world with 状态=1.
			Worlds:       []World{{ID: 0, State: 1}},
			EventMessage: "",
			UserLimit:    500,
		},
		Login: Login{
			Host:             "0.0.0.0",
			Port:             8484,
			AutoRegister:     true,
			// Java: 账号注册开关 defaults to 0 (= open) when the configvalues
			// row is absent, and ACCOUNTS_PER_MAC = 100.
			RegisterEnabled: true,
			AccountsPerMac:  100,
			AllowDuplicateIP: true,
		},
		Channel: Channel{
			Host:    "0.0.0.0",
			Port:    7575,
			Count:   5, // ZEV.Count=5
			CashShopPort: 8600,
		},
		WZ: WZ{
			// Java: System.getProperty("net.sf.odinms.wzpath", "wz")
			Path:      "wz",
			LoadNames: true,
		},
		Database: Database{
			Driver:             "mysql",
			DSN:                "root:root@tcp(127.0.0.1:3306)/079-max2?charset=utf8mb4&parseTime=true&loc=Local",
			MaxOpenConns:       64,
			MaxIdleConns:       16,
			ConnMaxLifetimeSec: 1800, // Java connectionTimeOut 30min
		},
		Game: Game{
			ExpRate:   20, // ZEV.Exp=20
			MesoRate:  20, // ZEV.Meso=20
			DropRate:  5,  // ZEV.Drop=5
			Notices:   []string{},
			PacketDebug: false,
			PacketTrace: false,
			AnticheatSuckMob:    true,
			AnticheatFullscreen: true,
			ChatFilter:          true,
		},
		Log: Log{Level: "info", Format: "json"},
	}
}

// Load reads path and merges it over Defaults. Missing file with empty name
// returns defaults alone (used by tests).
func Load(path string) (Root, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	if _, err := os.Stat(path); err != nil {
		return cfg, fmt.Errorf("config file %q: %w", path, err)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("decode %q: %w", path, err)
	}
	cfg.Database = resolveSQLitePath(cfg.Database, filepath.Dir(path))
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// resolveSQLitePath rewrites a relative sqlite DSN to be relative to baseDir
// (the directory of the config file that declared it). Paths inside a config
// file conventionally resolve next to it, and this keeps the server, the CLI
// tools and any launcher agreement on one database file no matter which
// working directory the process was started from - starting from tools/ used
// to silently create a second, empty database next to it. MySQL DSNs, absolute
// paths and `file:` URIs are returned unchanged.
func resolveSQLitePath(cfg Database, baseDir string) Database {
	if cfg.Driver != "sqlite" || baseDir == "" {
		return cfg
	}
	p := cfg.DSN
	suffix := ""
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p, suffix = p[:i], p[i:]
	}
	p = strings.TrimPrefix(p, "file:")
	if p == "" || filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return cfg
	}
	cfg.DSN = filepath.Join(baseDir, p) + suffix
	return cfg
}

// Validate sanity-checks ranges so startup fails fast (Java version started
// even with broken ini, causing mystery runtime failures).
func (r Root) Validate() error {
	if r.Login.Port <= 0 || r.Login.Port > 65535 {
		return fmt.Errorf("login.port out of range: %d", r.Login.Port)
	}
	if r.Channel.Port <= 0 || r.Channel.Port > 65535 {
		return fmt.Errorf("channel.port out of range: %d", r.Channel.Port)
	}
	if r.Channel.Count <= 0 || r.Channel.Count > 200 { // Java CHANNEL_COUNT=200 cap
		return fmt.Errorf("channel.count out of range: %d", r.Channel.Count)
	}
	seen := map[int]bool{}
	for _, w := range r.Server.Worlds {
		if w.ID < 0 || w.ID > 255 {
			return fmt.Errorf("server.worlds[%d].id out of byte range: %d", w.ID, w.ID)
		}
		if w.State < 0 || w.State > 255 {
			return fmt.Errorf("server.worlds[%d].state out of byte range: %d", w.ID, w.State)
		}
		if seen[w.ID] {
			return fmt.Errorf("server.worlds duplicate id: %d", w.ID)
		}
		seen[w.ID] = true
	}
	if r.Server.UserLimit < 0 {
		return fmt.Errorf("server.user_limit negative: %d", r.Server.UserLimit)
	}
	if r.Server.ExternalIP != "" && net.ParseIP(r.Server.ExternalIP) == nil {
		return fmt.Errorf("server.external_ip invalid: %q", r.Server.ExternalIP)
	}
	if r.Login.AccountsPerMac < 0 {
		return fmt.Errorf("login.accounts_per_mac negative: %d", r.Login.AccountsPerMac)
	}
	if r.Database.DSN == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}
	switch r.Database.Driver {
	case "", "mysql", "sqlite":
	default:
		return fmt.Errorf("database.driver invalid: %q (want mysql|sqlite)", r.Database.Driver)
	}
	switch r.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level invalid: %q", r.Log.Level)
	}
	return nil
}

// Get is a compatibility helper mirroring Java ServerProperties.getProperty:
// dotted key lookup used by ported code during transition.
func (r Root) Get(key, def string) string {
	switch key {
	case "ZEV.ServerMessage":
		return orDef(r.Game.ServerMessage, def)
	case "ZEV.Exp":
		return orDef(strconv.Itoa(r.Game.ExpRate), def)
	case "ZEV.Meso":
		return orDef(strconv.Itoa(r.Game.MesoRate), def)
	case "ZEV.Drop":
		return orDef(strconv.Itoa(r.Game.DropRate), def)
	case "ZEV.LPort":
		return orDef(strconv.Itoa(r.Login.Port), def)
	case "ZEV.Port":
		return orDef(strconv.Itoa(r.Channel.Port), def)
	case "ZEV.Count":
		return orDef(strconv.Itoa(r.Channel.Count), def)
	default:
		return def
	}
}

func orDef(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
