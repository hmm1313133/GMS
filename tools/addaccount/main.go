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
package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
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
	case *name != "" && *pass != "":
		upsert(ctx, db, *name, *pass, *gender)
	default:
		fmt.Fprintln(os.Stderr, "need -name NAME -pass PASS, -show NAME, -del NAME, "+
			"or a ban-list action (-bans, -banip/-unbanip, -banmac/-unbanmac)")
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
