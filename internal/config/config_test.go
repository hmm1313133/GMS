package config

import (
	"os"
	"path/filepath"
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

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []Root{
		func() Root { c := Defaults(); c.Login.Port = 70000; return c }(),
		func() Root { c := Defaults(); c.Channel.Count = 0; return c }(),
		func() Root { c := Defaults(); c.Database.DSN = ""; return c }(),
		func() Root { c := Defaults(); c.Log.Level = "verbose"; return c }(),
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("case %d must fail validation", i)
		}
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
