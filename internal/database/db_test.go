package database

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"GMS/internal/config"
)

// TestOpenIntegration is opt-in: set GMS_TEST_DB_DSN to a running MySQL
// (e.g. root:root@tcp(127.0.0.1:3306)/079-max2) to exercise the real pool.
func TestOpenIntegration(t *testing.T) {
	dsn := os.Getenv("GMS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("GMS_TEST_DB_DSN not set; skipping live MySQL test")
	}
	db, err := Open(config.Database{
		Driver:             "mysql",
		DSN:                dsn,
		MaxOpenConns:       2,
		MaxIdleConns:       1,
		ConnMaxLifetimeSec: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func sha1hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestSQLiteAccountChain exercises the whole login SQL surface on the
// auto-provisioned SQLite backend (the dev/smoke replacement for MySQL 5.5,
// docs/SESSION_STATE.md breakpoint B'): insert -> read -> state updates.
func TestSQLiteAccountChain(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(config.Database{
		Driver:             "sqlite",
		DSN:                filepath.Join(dir, "test.db"),
		MaxOpenConns:       2,
		MaxIdleConns:       1,
		ConnMaxLifetimeSec: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// smoke-test account (breakpoint B' step 3: gender=1 to skip CHOOSE_GENDER)
	if _, err := db.ExecContext(ctx,
		"INSERT INTO accounts (name, password, salt, gender, banned, gm, loggedin) "+
			"VALUES ('testgo', ?, NULL, 1, 0, 0, 0)",
		sha1hex("test123")); err != nil {
		t.Fatal(err)
	}

	// unknown name -> nil, nil (the auth layer's "no account" branch)
	if a, err := db.GetAccountByName(ctx, "nobody"); err != nil || a != nil {
		t.Fatalf("GetAccountByName(nobody) = %v, %v", a, err)
	}

	// known name -> full row incl. the digit-leading column and NULLs
	a, err := db.GetAccountByName(ctx, "testgo")
	if err != nil || a == nil {
		t.Fatalf("GetAccountByName: %v, %v", a, err)
	}
	if a.Password != sha1hex("test123") {
		t.Fatalf("password = %q", a.Password)
	}
	if a.Salt.Valid || a.SessionIP.Valid || a.Macs.Valid {
		t.Fatalf("expected NULL salt/SessionIP/macs, got %+v", a)
	}
	if a.Gender != 1 {
		t.Fatalf("gender = %d, want 1", a.Gender)
	}

	exists, err := db.AccountExists(ctx, "testgo")
	if err != nil || !exists {
		t.Fatalf("AccountExists = %v, %v", exists, err)
	}

	// login-state update + readback (what the live probe asserts after login)
	if err := db.UpdateLoginState(ctx, a.ID, 2, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	st, err := db.GetAccountState(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.LoggedIn != 2 {
		t.Fatalf("loggedin = %d, want 2", st.LoggedIn)
	}
	if !st.LastLogin.Valid {
		t.Fatal("lastlogin not set by UpdateLoginState")
	}

	// mac + password upgrade + unban side effects
	if err := db.UpdateAccountMac(ctx, a.ID, "11-22-33-44-55-66"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdatePasswordSHA1(ctx, a.ID, sha1hex("newpass")); err != nil {
		t.Fatal(err)
	}
	a2, err := db.GetAccountByName(ctx, "testgo")
	if err != nil {
		t.Fatal(err)
	}
	if a2.Macs.String != "11-22-33-44-55-66" || !a2.Macs.Valid {
		t.Fatalf("macs = %+v", a2.Macs)
	}
	if a2.Password != sha1hex("newpass") || a2.Salt.Valid {
		t.Fatalf("password upgrade wrong: pw=%q salt=%+v", a2.Password, a2.Salt)
	}

	if _, err := db.ExecContext(ctx, "UPDATE accounts SET banned = 1 WHERE id = ?", a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.UnbanAccount(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	a3, _ := db.GetAccountByName(ctx, "testgo")
	if a3.Banned != 0 {
		t.Fatalf("banned = %d, want 0 after unban", a3.Banned)
	}
}

// TestSQLiteBanAndAutoRegister exercises the P2.5 ban tables and the
// auto-register INSERT on the auto-provisioned SQLite backend (ipbans /
// macbans are created by Open, and empty = nobody is banned).
func TestSQLiteBanAndAutoRegister(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(config.Database{
		Driver:             "sqlite",
		DSN:                filepath.Join(dir, "bans.db"),
		MaxOpenConns:       2,
		MaxIdleConns:       1,
		ConnMaxLifetimeSec: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// fresh DB: no bans at all
	ips, err := db.BannedIPs(ctx)
	if err != nil || len(ips) != 0 {
		t.Fatalf("BannedIPs on a fresh db = %v, %v", ips, err)
	}
	if yes, err := db.IsBannedMac(ctx, "11-22-33-44-55-66"); err != nil || yes {
		t.Fatalf("IsBannedMac on a fresh db = %v, %v", yes, err)
	}

	if _, err := db.ExecContext(ctx, "INSERT INTO ipbans (ip) VALUES ('10.0.0.'), ('192.168.0.7')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO macbans (mac) VALUES ('AA-BB-CC-DD-EE-FF')"); err != nil {
		t.Fatal(err)
	}
	ips, err = db.BannedIPs(ctx)
	if err != nil || len(ips) != 2 {
		t.Fatalf("BannedIPs = %v, %v", ips, err)
	}
	if yes, err := db.IsBannedMac(ctx, "AA-BB-CC-DD-EE-FF"); err != nil || !yes {
		t.Fatalf("IsBannedMac(AA-BB-CC-DD-EE-FF) = %v, %v", yes, err)
	}
	if yes, err := db.IsBannedMac(ctx, "11-22-33-44-55-66"); err != nil || yes {
		t.Fatalf("IsBannedMac(unlisted) = %v, %v", yes, err)
	}

	// auto-register: insert + the per-mac counter that gates it
	if err := db.InsertAutoRegisterAccount(ctx, "autoreg", sha1hex("pw123"), "127.0.0.1", "11-22-33-44-55-66"); err != nil {
		t.Fatal(err)
	}
	n, err := db.CountAccountsByMac(ctx, "11-22-33-44-55-66")
	if err != nil || n != 1 {
		t.Fatalf("CountAccountsByMac = %d, %v", n, err)
	}
	if n, err := db.CountAccountsByMac(ctx, "FF-FF-FF-FF-FF-FF"); err != nil || n != 0 {
		t.Fatalf("CountAccountsByMac(other) = %d, %v", n, err)
	}
	acc, err := db.GetAccountByName(ctx, "autoreg")
	if err != nil || acc == nil {
		t.Fatalf("GetAccountByName(autoreg) = %v, %v", acc, err)
	}
	if acc.Password != sha1hex("pw123") || acc.Salt.Valid {
		t.Fatalf("auto-registered password = %q, salt = %+v (want plain sha1 + NULL salt)", acc.Password, acc.Salt)
	}
	if acc.Gender != 10 {
		t.Fatalf("gender = %d, want 10 (accounts DEFAULT -> CHOOSE_GENDER on first login)", acc.Gender)
	}
	if acc.SessionIP.String != "127.0.0.1" || acc.Macs.String != "11-22-33-44-55-66" {
		t.Fatalf("SessionIP = %+v macs = %+v", acc.SessionIP, acc.Macs)
	}
	if acc.Email.String != AutoRegEmail {
		t.Fatalf("email = %q, want the Java placeholder %q", acc.Email.String, AutoRegEmail)
	}
}
