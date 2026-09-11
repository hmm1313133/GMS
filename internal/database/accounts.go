package database

// Account DAO (Java client/MapleClient login surface), on GORM.

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

// Account is the DAO model for the `accounts` table (079-max2).
//
// It only exposes the columns the login server needs; P5 will extend the
// game-side account fields. Nullable columns keep sql.NullString so the
// "never set" and "set to empty" cases stay distinguishable - GORM scans and
// writes them natively.
// NOTE: no `default:` tags on written models. GORM omits zero-valued fields
// that carry one from INSERT/UPDATE, so `gender = 0` would silently become
// the column default 10. The schema (sqliteSchema / migrations/*.sql) is the
// single source of defaults; the DAOs write every field explicitly.
type Account struct {
	ID             int            `gorm:"primaryKey;column:id;autoIncrement"`
	Name           string         `gorm:"column:name;not null"`
	Password       string         `gorm:"column:password;not null"`
	Salt           sql.NullString `gorm:"column:salt"`
	SecondPassword sql.NullString `gorm:"column:2ndpassword"` // digit-leading identifier
	Salt2          sql.NullString `gorm:"column:salt2"`
	LoggedIn       int            `gorm:"column:loggedin;not null"`
	Banned         int            `gorm:"column:banned;not null"`
	BanReason      sql.NullString `gorm:"column:banreason"`
	GM             int            `gorm:"column:gm;not null"`
	Gender         int            `gorm:"column:gender;not null"`
	SessionIP      sql.NullString `gorm:"column:SessionIP"`
	Macs           sql.NullString `gorm:"column:macs"`
	// Email is only written by auto-registration (P2.5 AutoRegister: the
	// distribution inserts a placeholder address); read for schema fidelity.
	Email sql.NullString `gorm:"column:email"`
}

// TableName pins the table name (GORM would otherwise pluralise to
// "accounts" from Account - right here, but wrong for e.g. IPBan).
func (Account) TableName() string { return "accounts" }

// Login states (Java MapleClient constants, column `loggedin`).
const (
	LoginNotLoggedIn   = 0
	LoginServerTransit = 1
	LoginLoggedIn      = 2
	LoginWaiting       = 3
	CashShopTransition = 4
	LoginCSLoggedIn    = 5
	ChangeChannel      = 6
)

// GetAccountByName loads one account by login name (nil, nil when absent).
func (db *DB) GetAccountByName(ctx context.Context, name string) (*Account, error) {
	var a Account
	err := db.WithContext(ctx).Where("name = ?", name).Take(&a).Error
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("database: get account %q: %w", name, err)
	}
	return &a, nil
}

// AccountState loads the live login state / lastlogin for an account
// (Java MapleClient.getLoginState, incl. the 20s transition timeout).
type AccountState struct {
	LoggedIn  int          `gorm:"column:loggedin"`
	LastLogin sql.NullTime `gorm:"column:lastlogin"`
}

// GetAccountState reads loggedin/lastlogin for the double-login check.
// A missing account yields the zero value (Java read it off a row that must
// exist by then, so this stays a silent "not logged in").
func (db *DB) GetAccountState(ctx context.Context, id int) (AccountState, error) {
	var st AccountState
	err := db.WithContext(ctx).Model(&Account{}).
		Select("loggedin, lastlogin").
		Where("id = ?", id).
		Scan(&st).Error
	if err != nil {
		return st, fmt.Errorf("database: get account state %d: %w", id, err)
	}
	return st, nil
}

// accountUpdates is the shared "UPDATE accounts ... WHERE id = ?" builder.
func (db *DB) accountUpdates(ctx context.Context, id int, values map[string]any) error {
	if err := db.WithContext(ctx).Model(&Account{}).
		Where("id = ?", id).Updates(values).Error; err != nil {
		return fmt.Errorf("database: update account %d: %w", id, err)
	}
	return nil
}

// UpdateLoginState ports Java MapleClient.updateLoginState(newstate, SessionID):
// loggedin + SessionIP + lastlogin=CURRENT_TIMESTAMP (SessionIP untouched
// when sessionIP is empty - the Java null branch).
//
// CURRENT_TIMESTAMP stays un-parenthesised: SQLite rejects the MySQL-only
// CURRENT_TIMESTAMP() form.
func (db *DB) UpdateLoginState(ctx context.Context, id int, newstate int, sessionIP string) error {
	values := map[string]any{
		"loggedin":  newstate,
		"lastlogin": gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if sessionIP != "" {
		values["SessionIP"] = sessionIP
	}
	return db.accountUpdates(ctx, id, values)
}

// UpdatePasswordSHA1 ports Java MapleClient.updatePasswordHashtosha1: after a
// legacy/$H$ or salted-SHA-512 login succeeds, the hash is upgraded to plain
// SHA-1 with salt cleared (password = SHA-1, salt = NULL).
//
// The nil value is what makes GORM emit `salt = NULL` (a Go empty string
// would store '').
func (db *DB) UpdatePasswordSHA1(ctx context.Context, id int, password string) error {
	return db.accountUpdates(ctx, id, map[string]any{
		"password": password,
		"salt":     nil,
	})
}

// UnbanAccount ports Java MapleClient.unban (banned == -1 auto-unban on
// login). Note the Java SQL is `SET banned = 0 and banreason = ''` (an AND
// expression - MySQL evaluates to 0); we keep the *intent* and also fix the
// column set.
func (db *DB) UnbanAccount(ctx context.Context, id int) error {
	return db.accountUpdates(ctx, id, map[string]any{
		"banned":    0,
		"banreason": "",
	})
}

// UpdateAccountMac ports Java MapleClient.updateMacs (stores the 6-byte
// machine code as a dash-separated hex string; skipped for all-zero MACs).
func (db *DB) UpdateAccountMac(ctx context.Context, id int, macData string) error {
	return db.accountUpdates(ctx, id, map[string]any{"macs": macData})
}

// UpdateAccountGender ports Java MapleClient.updateGender (SET_GENDER flow).
func (db *DB) UpdateAccountGender(ctx context.Context, id int, gender int) error {
	return db.accountUpdates(ctx, id, map[string]any{"gender": gender})
}

// ResetAccountLogin ports the MapleClient.unlockAcc fallback (Java:
// "UPDATE accounts SET loggedin = 0 WHERE name = ?"): a double login was
// detected but no live session holds the account any more (crashed or
// force-killed client), so the stale loggedin state is cleared and the next
// attempt succeeds. Only loggedin is touched - lastlogin/SessionIP stay as
// the dead session left them, like the Java statement.
func (db *DB) ResetAccountLogin(ctx context.Context, id int) error {
	return db.accountUpdates(ctx, id, map[string]any{"loggedin": LoginNotLoggedIn})
}

// UnlockAccountByName clears the stale loggedin flag of the account with that
// name (the Java statement behind MapleClient.unlockAcc is keyed by name:
// "UPDATE accounts SET loggedin = 0 WHERE name = ?"). It is the smoke-test /
// ops escape hatch for a session that was force-killed: without it the account
// answers ALREADY_LOGGED_IN (7) until the 20s transition window expires.
// Returns the number of rows touched (0 = no such account).
func (db *DB) UnlockAccountByName(ctx context.Context, name string) (int64, error) {
	res := db.WithContext(ctx).Model(&Account{}).
		Where("name = ?", name).
		Update("loggedin", LoginNotLoggedIn)
	if res.Error != nil {
		return 0, fmt.Errorf("database: unlock account %q: %w", name, res.Error)
	}
	return res.RowsAffected, nil
}

// CreateAccount inserts a login account (tools/addaccount -pass, the smoke
// helper). passwordSHA1 is plain SHA-1 hex with salt cleared, i.e. what the
// login chain expects from a fresh row. Returns the new id.
func (db *DB) CreateAccount(ctx context.Context, name, passwordSHA1 string, gender int) (int, error) {
	a := &Account{
		Name:     name,
		Password: passwordSHA1,
		Gender:   gender,
	}
	if err := db.WithContext(ctx).Create(a).Error; err != nil {
		return 0, fmt.Errorf("database: create account %q: %w", name, err)
	}
	return a.ID, nil
}

// ResetAccountPassword re-arms an existing account: new SHA-1 password, salt
// cleared, ban and login state wiped (tools/addaccount reset semantics).
//
// The nil values are what makes GORM emit `col = NULL` (an empty Go string
// would store '').
func (db *DB) ResetAccountPassword(ctx context.Context, name, passwordSHA1 string, gender int) error {
	err := db.WithContext(ctx).Model(&Account{}).Where("name = ?", name).Updates(map[string]any{
		"password":  passwordSHA1,
		"salt":      nil,
		"gender":    gender,
		"banned":    0,
		"loggedin":  0,
		"SessionIP": nil,
		"macs":      nil,
	}).Error
	if err != nil {
		return fmt.Errorf("database: reset account %q: %w", name, err)
	}
	return nil
}

// DeleteAccountByName removes an account and reports the number of deleted
// rows (0 = no such account).
func (db *DB) DeleteAccountByName(ctx context.Context, name string) (int64, error) {
	res := db.WithContext(ctx).Where("name = ?", name).Delete(&Account{})
	if res.Error != nil {
		return 0, fmt.Errorf("database: delete account %q: %w", name, res.Error)
	}
	return res.RowsAffected, nil
}

// AccountExists reports whether a login name already exists.
func (db *DB) AccountExists(ctx context.Context, name string) (bool, error) {
	var a Account
	err := db.WithContext(ctx).Select("id").Where("name = ?", name).Take(&a).Error
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("database: account exists %q: %w", name, err)
	}
	return true, nil
}
