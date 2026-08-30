package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Account is the DAO model for the A-class table `accounts` (079-max2).
// It only exposes the columns the login server needs; P2.2 will add login
// crypto helpers, and P5 will extend the game-side account fields.
type Account struct {
	ID             int            `db:"id"`
	Name           string         `db:"name"`
	Password       string         `db:"password"`
	Salt           sql.NullString `db:"salt"`
	SecondPassword sql.NullString `db:"2ndpassword"`
	Salt2          sql.NullString `db:"salt2"`
	LoggedIn       int            `db:"loggedin"`
	Banned         int            `db:"banned"`
	BanReason      sql.NullString `db:"banreason"`
	GM             int            `db:"gm"`
	Gender         int            `db:"gender"`
	SessionIP      sql.NullString `db:"SessionIP"`
	Macs           sql.NullString `db:"macs"`
	// Email is only written by auto-registration (P2.5 AutoRegister: the
	// distribution inserts a placeholder address); read for schema fidelity.
	Email          sql.NullString `db:"email"`
}

// Login states (Java MapleClient constants, column `loggedin`).
const (
	LoginNotLoggedIn     = 0
	LoginServerTransit   = 1
	LoginLoggedIn        = 2
	LoginWaiting         = 3
	CashShopTransition   = 4
	LoginCSLoggedIn      = 5
	ChangeChannel        = 6
)

// GetAccountByName loads one account by login name.
func (db *DB) GetAccountByName(ctx context.Context, name string) (*Account, error) {
	var a Account
	// `2ndpassword` backticked: SQLite identifiers may not start with a digit
	// and MySQL natively accepts backticks - one query serves both drivers.
	q := "SELECT id, name, password, salt, `2ndpassword`, salt2, loggedin, " +
		"banned, banreason, gm, gender, SessionIP, macs, email " +
		"FROM accounts WHERE name = ?"
	if err := db.GetContext(ctx, &a, q, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("database: get account %q: %w", name, err)
	}
	return &a, nil
}

// GetAccountState loads the live login state / lastlogin for an account
// (Java MapleClient.getLoginState, incl. the 20s transition timeout).
type AccountState struct {
	LoggedIn  int        `db:"loggedin"`
	LastLogin sql.NullTime `db:"lastlogin"`
}

// GetAccountState reads loggedin/lastlogin for the double-login check.
func (db *DB) GetAccountState(ctx context.Context, id int) (AccountState, error) {
	var st AccountState
	err := db.GetContext(ctx, &st,
		`SELECT loggedin, lastlogin FROM accounts WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("database: get account state %d: %w", id, err)
	}
	return st, nil
}

// UpdateLoginState ports Java MapleClient.updateLoginState(newstate, SessionID):
// loggedin + SessionIP + lastlogin=CURRENT_TIMESTAMP (SessionIP untouched
// when sessionIP is empty - the Java null branch).
func (db *DB) UpdateLoginState(ctx context.Context, id int, newstate int, sessionIP string) error {
	var err error
	if sessionIP != "" {
		_, err = db.ExecContext(ctx,
			`UPDATE accounts SET loggedin = ?, SessionIP = ?, lastlogin = CURRENT_TIMESTAMP WHERE id = ?`,
			newstate, sessionIP, id)
	} else {
		_, err = db.ExecContext(ctx,
			`UPDATE accounts SET loggedin = ?, lastlogin = CURRENT_TIMESTAMP WHERE id = ?`,
			newstate, id)
	}
	if err != nil {
		return fmt.Errorf("database: update login state %d: %w", id, err)
	}
	return nil
}

// UpdatePasswordSHA1 ports Java MapleClient.updatePasswordHashtosha1: after a
// legacy/$H$ or salted-SHA-512 login succeeds, the hash is upgraded to plain
// SHA-1 with salt cleared (password = SHA-1, salt = NULL).
func (db *DB) UpdatePasswordSHA1(ctx context.Context, id int, password string) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE accounts SET password = ?, salt = NULL WHERE id = ?`,
		password, id); err != nil {
		return fmt.Errorf("database: update password %d: %w", id, err)
	}
	return nil
}

// UnbanAccount ports Java MapleClient.unban (banned == -1 auto-unban on
// login). Note the Java SQL is `SET banned = 0 and banreason = ''` (an AND
// expression - MySQL evaluates to 0); we keep the *intent* and also fix the
// column set.
func (db *DB) UnbanAccount(ctx context.Context, id int) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE accounts SET banned = 0, banreason = '' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("database: unban %d: %w", id, err)
	}
	return nil
}

// UpdateAccountMac ports Java MapleClient.updateMacs (stores the 6-byte
// machine code as a dash-separated hex string; skipped for all-zero MACs).
func (db *DB) UpdateAccountMac(ctx context.Context, id int, macData string) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE accounts SET macs = ? WHERE id = ?`, macData, id); err != nil {
		return fmt.Errorf("database: update mac %d: %w", id, err)
	}
	return nil
}

// UpdateAccountGender ports Java MapleClient.updateGender (SET_GENDER flow).
func (db *DB) UpdateAccountGender(ctx context.Context, id int, gender int) error {
	if _, err := db.ExecContext(ctx,
		`UPDATE accounts SET gender = ? WHERE id = ?`, gender, id); err != nil {
		return fmt.Errorf("database: update gender %d: %w", id, err)
	}
	return nil
}

// AccountExists reports whether a login name already exists.
func (db *DB) AccountExists(ctx context.Context, name string) (bool, error) {
	var one int
	err := db.GetContext(ctx, &one, `SELECT 1 FROM accounts WHERE name = ? LIMIT 1`, name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("database: account exists %q: %w", name, err)
	}
	return true, nil
}
