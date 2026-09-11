package database

// P2.5 IP/MAC ban lists + auto-registration, on GORM.
//
// Java sources (see docs/FILETRACK.md):
//   - client/MapleClient.hasBannedIP                  -> BannedIPs (ipbans, prefix match)
//   - client/MapleClient.isBannedMac                  -> IsBannedMac (macbans, exact match)
//   - handling/login/handler/AutoRegister.createAccount -> CountAccountsByMac
//     + InsertAutoRegisterAccount

import (
	"context"
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

// IPBan is a row of `ipbans` (a prefix rule, e.g. "10.0.0.").
type IPBan struct {
	IPBanID int    `gorm:"primaryKey;column:ipbanid;autoIncrement"`
	IP      string `gorm:"column:ip;not null"`
}

func (IPBan) TableName() string { return "ipbans" }

// MacBan is a row of `macbans` (an exact 17-char machine code).
type MacBan struct {
	MacBanID int    `gorm:"primaryKey;column:macbanid;autoIncrement"`
	Mac      string `gorm:"column:mac;not null"`
}

func (MacBan) TableName() string { return "macbans" }

// BannedIPs returns every `ipbans` entry. Java does the prefix match inside
// SQL (`? LIKE CONCAT(ip,'%')` against Netty's "/1.2.3.4:port" string); Go
// loads the (admin-sized) table once per login and matches in memory - one
// query for both drivers (SQLite has no CONCAT) and it tolerates rows stored
// with or without the Java leading-slash shape.
func (db *DB) BannedIPs(ctx context.Context) ([]string, error) {
	var rows []IPBan
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("database: list ip bans: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.IP)
	}
	return out, nil
}

// BannedMacs returns every `macbans` entry (the counterpart of BannedIPs,
// used by tools/addaccount -bans until the P8 ops panel exists).
func (db *DB) BannedMacs(ctx context.Context) ([]string, error) {
	var rows []MacBan
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("database: list mac bans: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Mac)
	}
	return out, nil
}

// AddIPBan adds an `ipbans` prefix rule.
func (db *DB) AddIPBan(ctx context.Context, ip string) error {
	if err := db.WithContext(ctx).Create(&IPBan{IP: ip}).Error; err != nil {
		return fmt.Errorf("database: add ip ban %q: %w", ip, err)
	}
	return nil
}

// RemoveIPBan drops every `ipbans` rule equal to ip.
func (db *DB) RemoveIPBan(ctx context.Context, ip string) error {
	if err := db.WithContext(ctx).Where("ip = ?", ip).Delete(&IPBan{}).Error; err != nil {
		return fmt.Errorf("database: remove ip ban %q: %w", ip, err)
	}
	return nil
}

// AddMacBan adds a `macbans` entry (exact 17-char machine code).
func (db *DB) AddMacBan(ctx context.Context, mac string) error {
	if err := db.WithContext(ctx).Create(&MacBan{Mac: mac}).Error; err != nil {
		return fmt.Errorf("database: add mac ban %q: %w", mac, err)
	}
	return nil
}

// RemoveMacBan drops every `macbans` entry equal to mac.
func (db *DB) RemoveMacBan(ctx context.Context, mac string) error {
	if err := db.WithContext(ctx).Where("mac = ?", mac).Delete(&MacBan{}).Error; err != nil {
		return fmt.Errorf("database: remove mac ban %q: %w", mac, err)
	}
	return nil
}

// IsBannedMac reports an exact `macbans` hit
// (Java: SELECT COUNT(*) FROM macbans WHERE mac = ?).
func (db *DB) IsBannedMac(ctx context.Context, mac string) (bool, error) {
	var n int64
	err := db.WithContext(ctx).Model(&MacBan{}).Where("mac = ?", mac).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("database: mac ban check %q: %w", mac, err)
	}
	return n > 0, nil
}

// CountAccountsByMac counts accounts already sharing a machine code
// (Java AutoRegister.createAccount: SELECT macs FROM accounts WHERE macs = ?).
func (db *DB) CountAccountsByMac(ctx context.Context, mac string) (int, error) {
	var n int64
	err := db.WithContext(ctx).Model(&Account{}).Where("macs = ?", mac).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("database: count accounts by mac: %w", err)
	}
	return int(n), nil
}

// InsertAutoRegisterAccount ports AutoRegister.createAccount's INSERT
// (name, password, email, birthday, macs, SessionIP, qq). passwordSHA1 is
// plain SHA-1 hex (Java LoginCrypto.hexSha1), which is exactly what the Go
// login chain expects on the next attempt (salt NULL -> sha1 branch).
//
// Written as a map rather than via the Account model: birthday and qq are
// placeholder columns no other Go code reads, so they do not earn a struct
// field.
//
// Deviation: Java stored SessionIP as Netty's "/1.2.3.4" form (it sliced the
// leading slash back in); Go stores the bare IP, consistent with
// UpdateLoginState (docs/SESSION_STATE.md §三.8).
func (db *DB) InsertAutoRegisterAccount(ctx context.Context, name, passwordSHA1, sessionIP, mac string) error {
	err := db.WithContext(ctx).Table("accounts").Create(map[string]any{
		"name":      name,
		"password":  passwordSHA1,
		"email":     AutoRegEmail,
		"birthday":  AutoRegBirthday,
		"macs":      mac,
		"SessionIP": sessionIP,
		"qq":        AutoRegQQ,
	}).Error
	if err != nil {
		return fmt.Errorf("database: auto-register %q: %w", name, err)
	}
	return nil
}
