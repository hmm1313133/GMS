// addaccount is the live-smoke helper (docs/SESSION_STATE.md breakpoint B'):
// it creates or resets a test account in the configured database (SHA-1
// password, salt NULL, gender=1 so login skips CHOOSE_GENDER), dumps the
// row state (-show), or deletes it (-del).
//
// Usage:
//
//	addaccount -config configs/gms.toml -name testgo -pass test123
//	addaccount -config configs/gms.toml -show testgo
//	addaccount -config configs/gms.toml -del testgo
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
	del := flag.String("del", "", "delete this account")
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
	case *name != "" && *pass != "":
		upsert(ctx, db, *name, *pass, *gender)
	default:
		fmt.Fprintln(os.Stderr, "need -name NAME -pass PASS, -show NAME, or -del NAME")
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
		if _, err := db.ExecContext(ctx,
			"UPDATE accounts SET password = ?, salt = NULL, gender = ?, banned = 0, "+
				"loggedin = 0, SessionIP = NULL, macs = NULL WHERE name = ?",
			sha1hex(pass), gender, name); err != nil {
			fail(err)
		}
		fmt.Printf("account %q reset (sha1, gender=%d)\n", name, gender)
		return
	}
	res, err := db.ExecContext(ctx,
		"INSERT INTO accounts (name, password, salt, gender, banned, gm, loggedin) "+
			"VALUES (?, ?, NULL, ?, 0, 0, 0)",
		name, sha1hex(pass), gender)
	if err != nil {
		fail(err)
	}
	id, _ := res.LastInsertId()
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
	res, err := db.ExecContext(ctx, "DELETE FROM accounts WHERE name = ?", name)
	if err != nil {
		fail(err)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("account %q deleted (%d rows)\n", name, n)
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
