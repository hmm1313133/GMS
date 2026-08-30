package database

// P2.5 IP/MAC ban lists + auto-registration.
//
// Java sources (see docs/FILETRACK.md):
//   - client/MapleClient.hasBannedIP                  -> BannedIPs (ipbans, prefix match)
//   - client/MapleClient.isBannedMac                  -> IsBannedMac (macbans, exact match)
//   - handling/login/handler/AutoRegister.createAccount -> CountAccountsByMac
//     + InsertAutoRegisterAccount

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Auto-register literal defaults (Java AutoRegister.createAccount): the
// 079MAX2 distribution hard-codes a placeholder mail / birthday / qq for
// freshly registered accounts. They only land in columns nothing else in the
// login chain reads; kept for schema fidelity.
const (
	AutoRegEmail    = "71447500@qq.com"
	AutoRegBirthday = "2017-08-10"
	AutoRegQQ       = "123456789"
)

// BannedIPs returns every `ipbans` entry. Java does the prefix match inside
// SQL (`? LIKE CONCAT(ip,'%')` against Netty's "/1.2.3.4:port" string); Go
// loads the (admin-sized) table once per login and matches in memory - one
// query for both drivers (SQLite has no CONCAT) and it tolerates rows stored
// with or without the Java leading-slash shape.
func (db *DB) BannedIPs(ctx context.Context) ([]string, error) {
	var out []string
	if err := db.SelectContext(ctx, &out, `SELECT ip FROM ipbans`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("database: list ip bans: %w", err)
	}
	return out, nil
}

// IsBannedMac reports an exact `macbans` hit
// (Java: SELECT COUNT(*) FROM macbans WHERE mac = ?).
func (db *DB) IsBannedMac(ctx context.Context, mac string) (bool, error) {
	var n int
	err := db.GetContext(ctx, &n, `SELECT COUNT(*) FROM macbans WHERE mac = ?`, mac)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("database: mac ban check %q: %w", mac, err)
	}
	return n > 0, nil
}

// CountAccountsByMac counts accounts already sharing a machine code
// (Java AutoRegister.createAccount: SELECT macs FROM accounts WHERE macs = ?).
func (db *DB) CountAccountsByMac(ctx context.Context, mac string) (int, error) {
	var n int
	err := db.GetContext(ctx, &n, `SELECT COUNT(*) FROM accounts WHERE macs = ?`, mac)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("database: count accounts by mac %q: %w", mac, err)
	}
	return n, nil
}

// InsertAutoRegisterAccount ports AutoRegister.createAccount's INSERT
// (name, password, email, birthday, macs, SessionIP, qq). passwordSHA1 is
// plain SHA-1 hex (Java LoginCrypto.hexSha1), which is exactly what the Go
// login chain expects on the next attempt (salt NULL -> sha1 branch).
//
// Deviation: Java stored SessionIP as Netty's "/1.2.3.4" form (it sliced the
// leading slash back in); Go stores the bare IP, consistent with
// UpdateLoginState (docs/SESSION_STATE.md §三.8).
func (db *DB) InsertAutoRegisterAccount(ctx context.Context, name, passwordSHA1, sessionIP, mac string) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO accounts (name, password, email, birthday, macs, SessionIP, qq) `+
			`VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, passwordSHA1, AutoRegEmail, AutoRegBirthday, mac, sessionIP, AutoRegQQ); err != nil {
		return fmt.Errorf("database: auto-register %q: %w", name, err)
	}
	return nil
}
