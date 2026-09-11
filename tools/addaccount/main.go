// addaccount is the live-smoke helper (docs/SESSION_STATE.md breakpoint B'):
// it creates or resets a test account in the configured database (SHA-1
// password, salt NULL, gender=1 so login skips CHOOSE_GENDER), dumps the
// row state (-show), or deletes it (-del).
//
// Usage:
//
//	addaccount -config configs/gms.toml -name testgo -pass test123
//	addaccount -config configs/gms.toml -show testgo
//	addaccount -config configs/gms.toml -remove testgo   (-del is an alias)
//
// P2.5 ban lists (ipbans / macbans), the only way to populate them until the
// P8 ops panel exists:
//
//	addaccount -config configs/gms.toml -bans
//	addaccount -config configs/gms.toml -banip 10.0.0.
//	addaccount -config configs/gms.toml -banmac AA-BB-CC-DD-EE-FF
//	addaccount -config configs/gms.toml -unbanip 10.0.0.
//	addaccount -config configs/gms.toml -unbanmac AA-BB-CC-DD-EE-FF
//
// P4.5b admin switches (configvalues: the Java Swing console's 按键开关; >0 =
// feature OFF). The server reads them once at startup:
//
//	addaccount -config configs/gms.toml -configvalues
//	addaccount -config configs/gms.toml -configvalue "玩家聊天开关=1"
//	addaccount -config configs/gms.toml -unsetconfigvalue 玩家聊天开关
//
// -unlock clears a stale accounts.loggedin (Java MapleClient.unlockAcc's
// SQL-by-name form): a probe killed mid-session otherwise answers
// ALREADY_LOGGED_IN (reason 7) until the 20s transition window expires.
package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"GMS/internal/config"
	"GMS/internal/database"
)

func main() {
	cfgPath := flag.String("config", "configs/gms.toml", "gms.toml path")
	name := flag.String("name", "", "account name to create/reset")
	pass := flag.String("pass", "", "password (stored as SHA-1 hex, salt NULL)")
	gender := flag.Int("gender", 1, "account gender (10 = CHOOSE_GENDER on login)")
	show := flag.String("show", "", "dump account row state for this name")
	del := flag.String("remove", "", "delete this account (-del is an alias)")
	// P2.5 ban lists
	listBans := flag.Bool("bans", false, "list ipbans and macbans entries")
	banIP := flag.String("banip", "", "add an ipbans prefix rule (prefix match, e.g. 10.0.0.)")
	banMac := flag.String("banmac", "", "add a macbans entry (exact 17-char machine code)")
	unbanIP := flag.String("unbanip", "", "remove an ipbans rule")
	unbanMac := flag.String("unbanmac", "", "remove a macbans entry")
	flag.StringVar(del, "del", "", "alias of -remove")
	// P4.5b configvalues switches (admin console keys)
	listCfg := flag.Bool("configvalues", false, "list configvalues switches (>0 = feature OFF)")
	setCfg := flag.String("configvalue", "", `set a switch, e.g. "玩家聊天开关=1" (restart to apply)`)
	unsetCfg := flag.String("unsetconfigvalue", "", "remove a switch row (reads 0 = enabled)")
	unlock := flag.String("unlock", "", "clear a stale accounts.loggedin for this account")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fail(fmt.Errorf("config: %w", err))
	}
	db, err := database.Open(cfg.Database)
	if err != nil {
		fail(fmt.Errorf("database: %w", err))
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch {
	case *show != "":
		showAccount(ctx, db, *show)
	case *del != "":
		delAccount(ctx, db, *del)
	case *listBans:
		listBanRules(ctx, db)
	case *banIP != "":
		execBan(ctx, db, db.AddIPBan, *banIP, "ipban added: %q\n")
	case *banMac != "":
		execBan(ctx, db, db.AddMacBan, *banMac, "macban added: %q\n")
	case *unbanIP != "":
		execBan(ctx, db, db.RemoveIPBan, *unbanIP, "ipban removed: %q\n")
	case *unbanMac != "":
		execBan(ctx, db, db.RemoveMacBan, *unbanMac, "macban removed: %q\n")
	case *listCfg:
		listConfigValues(ctx, db)
	case *setCfg != "":
		setConfigValue(ctx, db, *setCfg)
	case *unsetCfg != "":
		unsetConfigValue(ctx, db, *unsetCfg)
	case *unlock != "":
		unlockAccount(ctx, db, *unlock)
	case *name != "" && *pass != "":
		upsert(ctx, db, *name, *pass, *gender)
	default:
		fmt.Fprintln(os.Stderr, "need -name NAME -pass PASS, -show NAME, -del NAME, "+
			"a ban-list action (-bans, -banip/-unbanip, -banmac/-unbanmac), "+
			"or a switch action (-configvalues, -configvalue NAME=VAL, -unsetconfigvalue NAME)")
		os.Exit(2)
	}
}

func sha1hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func upsert(ctx context.Context, db *database.DB, name, pass string, gender int) {
	exists, err := db.AccountExists(ctx, name)
	if err != nil {
		fail(err)
	}
	if exists {
		if err := db.ResetAccountPassword(ctx, name, sha1hex(pass), gender); err != nil {
			fail(err)
		}
		fmt.Printf("account %q reset (sha1, gender=%d)\n", name, gender)
		return
	}
	id, err := db.CreateAccount(ctx, name, sha1hex(pass), gender)
	if err != nil {
		fail(err)
	}
	fmt.Printf("account %q created (id=%d, sha1, gender=%d)\n", name, id, gender)
}

func showAccount(ctx context.Context, db *database.DB, name string) {
	a, err := db.GetAccountByName(ctx, name)
	if err != nil {
		fail(err)
	}
	if a == nil {
		fmt.Printf("account %q: not found\n", name)
		return
	}
	st, err := db.GetAccountState(ctx, a.ID)
	if err != nil {
		fail(err)
	}
	fmt.Printf("account %q id=%d\n  loggedin=%d gender=%d banned=%d gm=%d\n  SessionIP=%s macs=%s\n  lastlogin=%v\n",
		name, a.ID, st.LoggedIn, a.Gender, a.Banned, a.GM,
		nullStr(a.SessionIP), nullStr(a.Macs), nullTime(st.LastLogin))
}

func delAccount(ctx context.Context, db *database.DB, name string) {
	n, err := db.DeleteAccountByName(ctx, name)
	if err != nil {
		fail(err)
	}
	fmt.Printf("account %q deleted (%d rows)\n", name, n)
}

// listBanRules dumps the P2.5 ban lists (ipbans are prefix rules, macbans
// exact 17-char machine codes).
func listBanRules(ctx context.Context, db *database.DB) {
	ips, err := db.BannedIPs(ctx)
	if err != nil {
		fail(err)
	}
	macs, err := db.BannedMacs(ctx)
	if err != nil {
		fail(err)
	}
	fmt.Printf("ipbans: %d\n", len(ips))
	for _, ip := range ips {
		fmt.Printf("  %s\n", ip)
	}
	fmt.Printf("macbans: %d\n", len(macs))
	for _, m := range macs {
		fmt.Printf("  %s\n", m)
	}
}

// execBan runs one ban-list mutation and reports the result.
func execBan(ctx context.Context, db *database.DB, op func(context.Context, string) error, arg, okFmt string) {
	if err := op(ctx, arg); err != nil {
		fail(err)
	}
	fmt.Printf(okFmt, arg)
}

// listConfigValues dumps the ZEVMS admin switches (P4.5b). The Java console is
// the original editor; this is the CLI stand-in until the P8 ops panel.
func listConfigValues(ctx context.Context, db *database.DB) {
	vals, err := db.ConfigValues(ctx)
	if err != nil {
		fail(err)
	}
	if len(vals) == 0 {
		fmt.Println("configvalues: (empty - every switch reads 0 = enabled)")
		return
	}
	fmt.Printf("configvalues: %d switch(es) (>0 = feature OFF)\n", len(vals))
	for name, val := range vals {
		state := "on"
		if val > 0 {
			state = "OFF"
		}
		fmt.Printf("  %s = %d (%s)\n", name, val, state)
	}
}

// setConfigValue writes one switch: -configvalue "玩家聊天开关=1" turns chat
// off, "=0" turns it back on, and -unsetconfigvalue removes the row.
func setConfigValue(ctx context.Context, db *database.DB, arg string) {
	i := strings.LastIndex(arg, "=")
	if i <= 0 {
		fail(fmt.Errorf("-configvalue: want NAME=VALUE, got %q", arg))
	}
	name := strings.TrimSpace(arg[:i])
	val, err := strconv.Atoi(strings.TrimSpace(arg[i+1:]))
	if err != nil {
		fail(fmt.Errorf("-configvalue: bad value in %q: %w", arg, err))
	}
	if err := db.SetConfigValue(ctx, name, val); err != nil {
		fail(err)
	}
	state := "on"
	if val > 0 {
		state = "OFF"
	}
	fmt.Printf("configvalue %q = %d (%s) - restart the server to apply\n", name, val, state)
}

func unsetConfigValue(ctx context.Context, db *database.DB, name string) {
	if err := db.DeleteConfigValue(ctx, name); err != nil {
		fail(err)
	}
	fmt.Printf("configvalue %q removed (reads 0 = enabled)\n", name)
}

// unlockAccount clears a stale loggedin flag so the next login succeeds.
func unlockAccount(ctx context.Context, db *database.DB, name string) {
	n, err := db.UnlockAccountByName(ctx, name)
	if err != nil {
		fail(err)
	}
	if n == 0 {
		fmt.Fprintf(os.Stderr, "account %q not found\n", name)
		os.Exit(1)
	}
	fmt.Printf("account %q unlocked (loggedin=0)\n", name)
}

func nullStr(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return "NULL"
}

func nullTime(t sql.NullTime) string {
	if t.Valid {
		return t.Time.Format(time.RFC3339)
	}
	return "NULL"
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
