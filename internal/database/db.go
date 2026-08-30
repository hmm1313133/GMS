// Package database is the Go replacement for Java
// database/DatabaseConnection(.1).java: a MySQL connection pool over
// database/sql + sqlx, configured from [database] in gms.toml (no more
// hardcoded root/root).
//
// The driver is switchable via [database].driver: "mysql" is the faithful
// original backend; "sqlite" (pure-Go modernc.org/sqlite, no CGO) auto-
// provisions the `accounts` table from the 079-max2 dump so dev/smoke runs
// do not need the MySQL 5.5 distribution running.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"GMS/internal/config"

	_ "github.com/go-sql-driver/mysql" // mysql driver
	_ "modernc.org/sqlite"             // pure-Go sqlite driver (no CGO)
)

// sqlNoRows re-exports sql.ErrNoRows for the DAO files.
var sqlNoRows = sql.ErrNoRows

// DB wraps sqlx.DB and adds the session defaults the Java code applied on
// every new connection (SET time_zone = '+08:00', MySQL only).
type DB struct {
	*sqlx.DB
	driver string // "mysql" | "sqlite" (dialect marker for shared queries)
}

// dialectInt quotes the `int` column (reserved-ish name) per dialect:
// MySQL backticks, SQLite double quotes.
func (db *DB) dialectInt() string {
	if db.driver == "sqlite" {
		return `"int"`
	}
	return "`int`"
}

// Open creates the pool and verifies connectivity. The DSN is taken from
// config; defaults reproduce the original 079MAX2 values (root/root@.../079-max2).
func Open(cfg config.Database) (*DB, error) {
	switch cfg.Driver {
	case "", "mysql":
		return openMySQL(cfg)
	case "sqlite":
		return openSQLite(cfg)
	default:
		return nil, fmt.Errorf("database: unknown driver %q (want mysql|sqlite)", cfg.Driver)
	}
}

func openMySQL(cfg config.Database) (*DB, error) {
	db, err := sqlx.Connect("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "SET time_zone = '+08:00'"); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: set time_zone: %w", err)
	}
	return &DB{DB: db, driver: "mysql"}, nil
}

// openSQLite provisions a local SQLite DB: creates the parent directory and
// applies the accounts DDL (SQLite translation of CREATE TABLE `accounts`
// in migrations/0001_base.sql:224) so a fresh file is immediately usable
// for login smoke tests.
//
// Caveat: SQLite CURRENT_TIMESTAMP stores UTC (MySQL path stores +08:00 per
// the Java session tz); only the 20s server-transition staleness check
// reads lastlogin, so the skew is inconsequential.
func openSQLite(cfg config.Database) (*DB, error) {
	if p := sqliteFilePath(cfg.DSN); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("database: mkdir %s: %w", dir, err)
			}
		}
	}
	db, err := sqlx.Connect("sqlite", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: connect sqlite: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)

	for _, stmt := range sqliteSchema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: sqlite schema: %w", err)
		}
	}
	return &DB{DB: db, driver: "sqlite"}, nil
}

// sqliteFilePath extracts the file path of a sqlite DSN (drops a `file:`
// prefix and any `?_pragma=...` query part).
func sqliteFilePath(dsn string) string {
	p := dsn
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	return strings.TrimPrefix(p, "file:")
}

// sqliteSchema mirrors CREATE TABLE `accounts` from migrations/0001_base.sql
// column-for-column (MySQL -> SQLite type translation; "2ndpassword" is
// quoted because SQLite identifiers may not start with a digit).
var sqliteSchema = []string{
	`CREATE TABLE IF NOT EXISTS accounts (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  name TEXT NOT NULL DEFAULT '',
	  password TEXT NOT NULL DEFAULT '',
	  salt TEXT DEFAULT NULL,
	  "2ndpassword" TEXT DEFAULT NULL,
	  salt2 TEXT DEFAULT NULL,
	  loggedin INTEGER NOT NULL DEFAULT 0,
	  lastlogin TIMESTAMP NULL DEFAULT NULL,
	  createdat TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	  birthday TEXT NOT NULL DEFAULT '0000-00-00',
	  banned INTEGER NOT NULL DEFAULT 0,
	  banreason TEXT,
	  gm INTEGER NOT NULL DEFAULT 0,
	  email TEXT,
	  macs TEXT,
	  tempban TIMESTAMP NOT NULL DEFAULT '0000-00-00 00:00:00',
	  greason INTEGER DEFAULT NULL,
	  ACash INTEGER DEFAULT 0,
	  mPoints INTEGER DEFAULT 0,
	  gender INTEGER NOT NULL DEFAULT 10,
	  SessionIP TEXT DEFAULT NULL,
	  points INTEGER NOT NULL DEFAULT 0,
	  vpoints INTEGER NOT NULL DEFAULT 0,
	  lastlogon TIMESTAMP NULL DEFAULT NULL,
	  qq TEXT DEFAULT NULL,
	  access_token TEXT DEFAULT '',
	  password_otp TEXT DEFAULT '',
	  expiration TIMESTAMP NULL DEFAULT NULL,
	  VIP INTEGER DEFAULT NULL,
	  money INTEGER NOT NULL DEFAULT 0,
	  moneyb INTEGER NOT NULL DEFAULT 0,
	  lastGainHM INTEGER NOT NULL DEFAULT 0,
	  md5pass TEXT DEFAULT NULL,
	  df_tired_point INTEGER NOT NULL DEFAULT 0,
	  df_cheating_player INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS accounts_name ON accounts(name)`,
	`CREATE INDEX IF NOT EXISTS accounts_ranking1 ON accounts(id, banned, gm)`,
	// characters: full 079-max2 column set (migrations/0001_base.sql:904),
	// MySQL->SQLite type translation; `int` quoted (SQLite identifiers may
	// not collide with type names unquoted; MySQL backticks in shared DAO
	// queries via dialectInt()).
	`CREATE TABLE IF NOT EXISTS characters (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  accountid INTEGER NOT NULL DEFAULT 0,
	  world INTEGER NOT NULL DEFAULT 0,
	  name TEXT NOT NULL DEFAULT '',
	  level INTEGER NOT NULL DEFAULT 0,
	  exp INTEGER NOT NULL DEFAULT 0,
	  str INTEGER NOT NULL DEFAULT 0,
	  dex INTEGER NOT NULL DEFAULT 0,
	  luk INTEGER NOT NULL DEFAULT 0,
	  "int" INTEGER NOT NULL DEFAULT 0,
	  hp INTEGER NOT NULL DEFAULT 0,
	  mp INTEGER NOT NULL DEFAULT 0,
	  maxhp INTEGER NOT NULL DEFAULT 0,
	  maxmp INTEGER NOT NULL DEFAULT 0,
	  meso INTEGER NOT NULL DEFAULT 20000,
	  hpApUsed INTEGER NOT NULL DEFAULT 0,
	  job INTEGER NOT NULL DEFAULT 0,
	  skincolor INTEGER NOT NULL DEFAULT 0,
	  gender INTEGER NOT NULL DEFAULT 0,
	  fame INTEGER NOT NULL DEFAULT 0,
	  hair INTEGER NOT NULL DEFAULT 0,
	  face INTEGER NOT NULL DEFAULT 0,
	  ap INTEGER NOT NULL DEFAULT 0,
	  map INTEGER NOT NULL DEFAULT 0,
	  spawnpoint INTEGER NOT NULL DEFAULT 0,
	  gm INTEGER NOT NULL DEFAULT 6,
	  party INTEGER NOT NULL DEFAULT 0,
	  buddyCapacity INTEGER NOT NULL DEFAULT 25,
	  createdate TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	  guildid INTEGER NOT NULL DEFAULT 0,
	  guildrank INTEGER NOT NULL DEFAULT 5,
	  allianceRank INTEGER NOT NULL DEFAULT 5,
	  monsterbookcover INTEGER NOT NULL DEFAULT 0,
	  dojo_pts INTEGER NOT NULL DEFAULT 0,
	  dojoRecord INTEGER NOT NULL DEFAULT 0,
	  pets TEXT NOT NULL DEFAULT '-1,-1,-1',
	  sp TEXT NOT NULL DEFAULT '0,0,0,0,0,0,0,0,0,0',
	  subcategory INTEGER NOT NULL DEFAULT 0,
	  Jaguar INTEGER NOT NULL DEFAULT 0,
	  rank INTEGER NOT NULL DEFAULT 1,
	  rankMove INTEGER NOT NULL DEFAULT 0,
	  jobRank INTEGER NOT NULL DEFAULT 1,
	  jobRankMove INTEGER NOT NULL DEFAULT 0,
	  marriageId INTEGER NOT NULL DEFAULT 0,
	  familyid INTEGER NOT NULL DEFAULT 0,
	  seniorid INTEGER NOT NULL DEFAULT 0,
	  junior1 INTEGER NOT NULL DEFAULT 0,
	  junior2 INTEGER NOT NULL DEFAULT 0,
	  currentrep INTEGER NOT NULL DEFAULT 0,
	  totalrep INTEGER NOT NULL DEFAULT 0,
	  charmessage TEXT NOT NULL DEFAULT 'ZEV',
	  expression INTEGER NOT NULL DEFAULT 0,
	  constellation INTEGER NOT NULL DEFAULT 0,
	  blood INTEGER NOT NULL DEFAULT 0,
	  month INTEGER NOT NULL DEFAULT 0,
	  day INTEGER NOT NULL DEFAULT 0,
	  beans INTEGER NOT NULL DEFAULT 0,
	  prefix TEXT DEFAULT '',
	  gachexp INTEGER NOT NULL DEFAULT 0,
	  pvpkills INTEGER DEFAULT NULL,
	  pvpdeaths INTEGER DEFAULT NULL,
	  mountid INTEGER NOT NULL DEFAULT 0,
	  todayOnlineTime INTEGER DEFAULT 0,
	  totalOnlineTime INTEGER DEFAULT 0,
	  autohp INTEGER NOT NULL DEFAULT 0,
	  automp INTEGER NOT NULL DEFAULT 0,
	  mxmxd_dakong_qianneng TEXT DEFAULT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS characters_accountid ON characters(accountid)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS characters_name ON characters(name)`,
	`CREATE INDEX IF NOT EXISTS characters_ranking1 ON characters(level, exp)`,
	`CREATE INDEX IF NOT EXISTS characters_ranking2 ON characters(gm, job)`,
	// character_slots: Java MapleClient.getCharacterSlots lazy provisioning.
	`CREATE TABLE IF NOT EXISTS character_slots (
	  accid INTEGER NOT NULL,
	  worldid INTEGER NOT NULL,
	  charslots INTEGER NOT NULL DEFAULT 6,
	  PRIMARY KEY (accid, worldid)
	)`,
}

// Ping checks the pool with a bounded context (Java: Connection.isValid).
func (db *DB) Ping(ctx context.Context) error {
	return db.DB.PingContext(ctx)
}
