package login

// P2.5 end-to-end tests: IP/MAC ban lists + auto-registration, driven over
// the real wire (auth_test.go loginFlowN) so the packet sequence is covered
// together with the DB side effects.

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"

	"GMS/internal/config"
	"GMS/internal/database"
)

// autoRegCfg is the login config used by the auto-register tests: registration
// open, 100 accounts per machine code (Java AutoRegister.ACCOUNTS_PER_MAC).
func autoRegCfg() config.Login {
	return config.Login{
		Host:            "127.0.0.1",
		Port:            0,
		AutoRegister:    true,
		RegisterEnabled: true,
		AccountsPerMac:  100,
	}
}

func testServer(store accountStore) *Server {
	srv := New(config.Login{}, slog.New(slog.NewTextHandler(&discardWriter{}, nil)))
	if store != nil {
		srv.SetStore(store)
	}
	return srv
}

// ---- ban list unit tests (Java MapleClient.hasBannedIP / isBannedMac) ----

func TestBannedIPPrefixMatch(t *testing.T) {
	fs := newFakeStore()
	fs.ipBans = []string{"10.0.0.", "192.168.1.100", ""}
	srv := testServer(fs)
	ctx := context.Background()

	for _, tc := range []struct {
		remote string
		want   bool
	}{
		{"10.0.0.5:5555", true},      // prefix of the full "ip:port"
		{"10.0.1.5:5555", false},     // different subnet
		{"192.168.1.100:1234", true}, // exact host
		{"192.168.1.1:1234", false},  // 192.168.1.1 does not start with 192.168.1.100
		{"127.0.0.1:8484", false},    // unlisted
	} {
		if got := srv.bannedIP(ctx, tc.remote); got != tc.want {
			t.Errorf("bannedIP(%q) = %v, want %v", tc.remote, got, tc.want)
		}
	}
}

func TestBannedMacGuards(t *testing.T) {
	fs := newFakeStore()
	// Java isBannedMac: an all-zero machine code and anything that is not a
	// 17-char dash-hex string is never banned, even if listed.
	fs.macBans["00-00-00-00-00-00"] = true
	fs.macBans["11-22-33-44-55-66"] = true
	srv := testServer(fs)
	ctx := context.Background()

	for _, tc := range []struct {
		mac  string
		want bool
	}{
		{"00-00-00-00-00-00", false}, // zero MAC guard
		{"00-00-00-00-00-00", false},
		{"11-22-33", false},          // malformed length guard
		{"", false},                  // no machine code at all
		{"11-22-33-44-55-66", true},  // exact macbans hit
		{"AA-BB-CC-DD-EE-FF", false}, // unlisted
	} {
		if got := srv.bannedMac(ctx, tc.mac); got != tc.want {
			t.Errorf("bannedMac(%q) = %v, want %v", tc.mac, got, tc.want)
		}
	}
	if srv.bannedMac(ctx, "11-22-33-44-55-66") != true {
		t.Fatal("mac ban lookup must be repeatable")
	}
}

// ---- auto-registration wire tests ----

func TestAutoRegisterCreatesAccount(t *testing.T) {
	fs := newFakeStore()
	pkts := loginFlowN(t, fs, autoRegCfg(), "newguy", "pw123", 2)

	// Java: getAuthSuccess-ish branch is not used; the player gets a popup
	// plus getLoginFailed(1) so the client returns to the login dialog.
	if got := noticeText(t, pkts[0]); got != "<<GMS>>\r\n\r\n账号注册成功，请重新登录，即可进入游戏。" {
		t.Fatalf("notice = %q", got)
	}
	if got := failedReason(t, pkts[1]); got != 1 {
		t.Fatalf("LOGIN_STATUS reason = %d, want 1", got)
	}

	if len(fs.registered) != 1 {
		t.Fatalf("registered = %d accounts, want 1", len(fs.registered))
	}
	a := fs.registered[0]
	if a.Name != "newguy" {
		t.Fatalf("name = %q", a.Name)
	}
	if a.Password != gSha1("pw123") {
		t.Fatalf("password = %q, want sha1(pw123)=%q", a.Password, gSha1("pw123"))
	}
	if a.Macs.String != "11-22-33-44-55-66" || !a.Macs.Valid {
		t.Fatalf("macs = %+v", a.Macs)
	}
	if a.SessionIP.String != "127.0.0.1" {
		t.Fatalf("SessionIP = %q, want 127.0.0.1", a.SessionIP.String)
	}
}

// TestAutoRegisterThenLogin is the whole point of the feature: the account
// created by the first attempt authenticates on the second one (gender=10, so
// the reply is CHOOSE_GENDER like stock v079).
func TestAutoRegisterThenLogin(t *testing.T) {
	fs := newFakeStore()
	loginFlowN(t, fs, autoRegCfg(), "newguy", "pw123", 2)

	body := loginFlow(t, fs, "newguy", "pw123")
	if opOf(body) != 0x0004 {
		t.Fatalf("opcode = 0x%04X, want CHOOSE_GENDER (0x0004)", opOf(body))
	}
}

func TestAutoRegisterDisabled(t *testing.T) {
	fs := newFakeStore()
	cfg := autoRegCfg()
	cfg.RegisterEnabled = false

	pkts := loginFlowN(t, fs, cfg, "newguy", "pw123", 2)
	if got := noticeText(t, pkts[0]); got != "<<GMS>>\r\n\r\n管理员未开启注册功能。" {
		t.Fatalf("notice = %q", got)
	}
	if got := failedReason(t, pkts[1]); got != 1 {
		t.Fatalf("LOGIN_STATUS reason = %d, want 1", got)
	}
	if len(fs.registered) != 0 {
		t.Fatalf("no account must be created while registration is off: %+v", fs.registered)
	}
}

func TestAutoRegisterRejectsReservedPassword(t *testing.T) {
	for _, pw := range []string{"fixme", "DISCONNECT", "Disconnect"} {
		fs := newFakeStore()
		pkts := loginFlowN(t, fs, autoRegCfg(), "newguy", pw, 2)
		if got := noticeText(t, pkts[0]); got != "此密码无效。" {
			t.Fatalf("pwd %q: notice = %q", pw, got)
		}
		if got := failedReason(t, pkts[1]); got != 1 {
			t.Fatalf("pwd %q: LOGIN_STATUS reason = %d, want 1", pw, got)
		}
		if len(fs.registered) != 0 {
			t.Fatalf("pwd %q: no account must be created", pw)
		}
	}
}

func TestAutoRegisterMacLimit(t *testing.T) {
	fs := newFakeStore()
	// two accounts already share the probe's machine code
	fs.accounts["acc1"] = &database.Account{
		ID: 1, Name: "acc1", Macs: sql.NullString{String: "11-22-33-44-55-66", Valid: true},
	}
	fs.accounts["acc2"] = &database.Account{
		ID: 2, Name: "acc2", Macs: sql.NullString{String: "11-22-33-44-55-66", Valid: true},
	}
	cfg := autoRegCfg()
	cfg.AccountsPerMac = 2

	pkts := loginFlowN(t, fs, cfg, "newguy", "pw123", 2)
	if got := noticeText(t, pkts[0]); !strings.Contains(got, "已达上限") {
		t.Fatalf("notice = %q, want the per-mac cap message", got)
	}
	if len(fs.registered) != 0 {
		t.Fatalf("account must not be created past the per-mac cap: %+v", fs.registered)
	}
}

func TestAutoRegisterSkippedWhenBanned(t *testing.T) {
	fs := newFakeStore()
	fs.ipBans = []string{"127.0.0."}

	// banned + unknown account: Java skips the register branch entirely and
	// falls through to c.login -> 5 (no account).
	body := loginFlowN(t, fs, autoRegCfg(), "newguy", "pw123", 1)[0]
	if got := failedReason(t, body); got != 5 {
		t.Fatalf("reason = %d, want 5 (no account)", got)
	}
	if len(fs.registered) != 0 {
		t.Fatalf("banned peer must not register: %+v", fs.registered)
	}
}

func TestAutoRegisterSkippedForExistingAccount(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["olduser"] = &database.Account{
		ID: 9, Name: "olduser", Password: gSha1("pw123"), Gender: 1,
	}
	fs.states[9] = database.AccountState{LoggedIn: 0}

	body := loginFlow(t, fs, "olduser", "pw123")
	if got := failedReason(t, body); got != 0 {
		t.Fatalf("reason = %d, want 0 (existing account logs in, no registration)", got)
	}
	if len(fs.registered) != 0 {
		t.Fatalf("existing account must not be re-registered: %+v", fs.registered)
	}
}

// ---- ban checks inside the login chain ----

func TestLoginBannedIPReportsBanned(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["victim"] = &database.Account{
		ID: 11, Name: "victim", Password: gSha1("pw123"), Gender: 1,
	}
	fs.states[11] = database.AccountState{LoggedIn: 0}
	fs.ipBans = []string{"127.0.0."}

	// correct credentials, but the peer is on the ipbans list ->
	// Java: loginok == 0 && ipBan && !gm -> 3
	body := loginFlow(t, fs, "victim", "pw123")
	if got := failedReason(t, body); got != 3 {
		t.Fatalf("reason = %d, want 3 (banned)", got)
	}
}

func TestLoginBannedMacReportsBanned(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["victim"] = &database.Account{
		ID: 12, Name: "victim", Password: gSha1("pw123"), Gender: 1,
	}
	fs.states[12] = database.AccountState{LoggedIn: 0}
	fs.macBans["11-22-33-44-55-66"] = true // the probe's machine code

	body := loginFlow(t, fs, "victim", "pw123")
	if got := failedReason(t, body); got != 3 {
		t.Fatalf("reason = %d, want 3 (banned)", got)
	}
}

func TestLoginGMOverridesIPBan(t *testing.T) {
	fs := newFakeStore()
	fs.accounts["admin"] = &database.Account{
		ID: 13, Name: "admin", Password: gSha1("pw123"), Gender: 1, GM: 1,
	}
	fs.states[13] = database.AccountState{LoggedIn: 0}
	fs.ipBans = []string{"127.0.0."}

	body := loginFlow(t, fs, "admin", "pw123")
	if got := failedReason(t, body); got != 0 {
		t.Fatalf("reason = %d, want 0 (GM is exempt from ip/mac bans)", got)
	}
}

// TestLoginWithoutDatabaseDegrades covers cmd/gms' degraded mode: no store
// wired (DB unreachable at startup) -> the port still answers 5 instead of
// panicking on a nil store.
func TestLoginWithoutDatabaseDegrades(t *testing.T) {
	body := loginFlowN(t, nil, autoRegCfg(), "nobody", "pw123", 1)[0]
	if got := failedReason(t, body); got != 5 {
		t.Fatalf("reason = %d, want 5 (degraded: no database)", got)
	}
}
