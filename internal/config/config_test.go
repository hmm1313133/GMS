package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDefaultsValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("no-such-file.toml"); err == nil {
		t.Fatal("missing file must error")
	}
}

func TestLoadEmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("empty path: %v", err)
	}
	// ZEV.* baseline from K:\079MAX2服务端\Load\服务端配置加载项.ini
	if cfg.Game.ExpRate != 20 || cfg.Game.MesoRate != 20 || cfg.Game.DropRate != 5 {
		t.Fatalf("rates not matching original ini: %+v", cfg.Game)
	}
	if cfg.Login.Port != 8484 || cfg.Channel.Port != 7575 || cfg.Channel.Count != 5 {
		t.Fatalf("ports not matching original ini: %+v", cfg)
	}
}

func TestWZDefaults(t *testing.T) {
	cfg := Defaults()
	// Java: System.getProperty("net.sf.odinms.wzpath", "wz")
	if cfg.WZ.Path != "wz" {
		t.Fatalf("wz path default = %q, want wz", cfg.WZ.Path)
	}
	if !cfg.WZ.LoadNames {
		t.Fatal("wz name tables should load by default (Java factories do)")
	}
}

func TestLoadWZSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gms.toml")
	body := `
[wz]
path = "data/wz"
load_names = false
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.WZ.Path != "data/wz" || cfg.WZ.LoadNames {
		t.Fatalf("wz section not decoded: %+v", cfg.WZ)
	}
}

func TestLoadAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gms.toml")
	body := `
[login]
port = 9999
[game]
exp_rate = 50
[database]
dsn = "u:p@tcp(1.2.3.4:3306)/x"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Login.Port != 9999 {
		t.Fatalf("login.port override failed: %d", cfg.Login.Port)
	}
	if cfg.Game.ExpRate != 50 {
		t.Fatalf("exp_rate override failed: %d", cfg.Game.ExpRate)
	}
	// defaults survive partial override
	if cfg.Channel.Port != 7575 {
		t.Fatalf("channel.port default lost: %d", cfg.Channel.Port)
	}
}

// TestLoginRegistrationDefaults pins the P2.5 auto-register switches to the
// Java originals: ZEV.自动注册 on, 账号注册开关 = 0 (open), ACCOUNTS_PER_MAC = 100.
func TestLoginRegistrationDefaults(t *testing.T) {
	c := Defaults()
	if !c.Login.AutoRegister || !c.Login.RegisterEnabled {
		t.Fatalf("defaults must auto-register: %+v", c.Login)
	}
	if c.Login.AccountsPerMac != 100 {
		t.Fatalf("accounts_per_mac = %d, want 100 (AutoRegister.ACCOUNTS_PER_MAC)", c.Login.AccountsPerMac)
	}
}

// TestLoadResolvesSQLiteDSN pins the P2.5 fix: a relative sqlite path in
// gms.toml is resolved against the config file's directory, so the server and
// the CLI tools always agree on one database file whatever the cwd was.
func TestLoadResolvesSQLiteDSN(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "configs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "gms.toml")
	body := `
[database]
driver = "sqlite"
dsn = "../data/gms.db?_pragma=busy_timeout(5000)"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := filepath.Join(dir, "data", "gms.db") + "?_pragma=busy_timeout(5000)"
	if cfg.Database.DSN != want {
		t.Fatalf("DSN = %q, want %q", cfg.Database.DSN, want)
	}

	// absolute paths and MySQL DSNs are untouched
	abs := filepath.Join(dir, "abs.db")
	body = "[database]\ndriver = \"sqlite\"\ndsn = " + strconv.Quote(abs) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Database.DSN != abs {
		t.Fatalf("absolute DSN rewritten: %q", cfg.Database.DSN)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []Root{
		func() Root { c := Defaults(); c.Login.Port = 70000; return c }(),
		func() Root { c := Defaults(); c.Channel.Count = 0; return c }(),
		func() Root { c := Defaults(); c.Database.DSN = ""; return c }(),
		func() Root { c := Defaults(); c.Log.Level = "verbose"; return c }(),
		func() Root { c := Defaults(); c.Login.AccountsPerMac = -1; return c }(),
		func() Root { c := Defaults(); c.Server.ExternalIP = "not-an-ip"; return c }(),
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("case %d must fail validation", i)
		}
	}
}

func TestExternalIPDefault(t *testing.T) {
	// Java MapleParty.IP地址 defaults to the unusable 0.0.0.0; Go hands
	// clients a loopback address for local smoke tests instead.
	cfg := Defaults()
	if cfg.Server.ExternalIP != "127.0.0.1" {
		t.Fatalf("external ip = %q, want 127.0.0.1", cfg.Server.ExternalIP)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default external ip must validate: %v", err)
	}
	// an empty value is allowed (the server falls back to loopback)
	cfg.Server.ExternalIP = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty external ip must validate: %v", err)
	}
}

func TestGetCompatibilityKeys(t *testing.T) {
	cfg := Defaults()
	if got := cfg.Get("ZEV.Exp", "1"); got != "20" {
		t.Fatalf("ZEV.Exp = %q", got)
	}
	if got := cfg.Get("ZEV.LPort", "1"); got != "8484" {
		t.Fatalf("ZEV.LPort = %q", got)
	}
	if got := cfg.Get("ZEV.Missing", "fallback"); got != "fallback" {
		t.Fatalf("unknown key must return default, got %q", got)
	}
}
