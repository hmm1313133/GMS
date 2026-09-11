// Package database is the Go replacement for Java
// database/DatabaseConnection(.1).java: a GORM-backed connection pool over
// database/sql, configured from [database] in gms.toml (no more hardcoded
// root/root).
//
// The driver is switchable via [database].driver: "mysql" is the faithful
// original backend (the 079-max2 schema); "sqlite" is a pure-Go dev/smoke
// backend that provisions the same schema so no MySQL 5.5 distribution is
// needed.
//
// Two deliberate choices (see docs/SESSION_STATE.md §GORM):
//
//   - The SQLite driver is github.com/glebarez/sqlite, not the official
//     gorm.io/driver/sqlite: the latter pulls mattn/go-sqlite3, which needs
//     CGO (gcc) and this box has none. glebarez is a fork of modernc's
//     pure-Go engine. Consequence: modernc.org/sqlite must NOT be imported
//     anywhere in this binary, because both register a driver named
//     "sqlite" and database/sql panics with "Register called twice".
//
//   - The schema is NOT auto-migrated. GORM's AutoMigrate is only ever
//     pointed at the throwaway SQLite file; the production MySQL database
//     (225 tables of live data) is migrated by migrations/*.sql, and the
//     models here are business-facing subsets (Character carries 27 of
//     characters' 62 columns), so AutoMigrate would silently narrow them.
package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GMS/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrNotFound is GORM's "no row" sentinel, re-exported so DAO callers do not
// have to import gorm only for the comparison.
var ErrNotFound = gorm.ErrRecordNotFound

// DB wraps *gorm.DB and keeps the driver name (the SQLite path provisions
// schema and seeds that MySQL already has).
type DB struct {
	*gorm.DB
	driver string // "mysql" | "sqlite"
}

// Driver reports the backend in use ("mysql" | "sqlite").
func (db *DB) Driver() string { return db.driver }

// gormConfig is shared by both backends.
//
// Logger runs at Warn: GORM's default logs every statement at Info, which
// would drown the server log (the drop seed alone is 14k rows).
// TranslateError turns driver-specific failures (duplicate key, ...) into
// gorm's sentinels so the DAO layer can branch on them.
func gormConfig() *gorm.Config {
	return &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		TranslateError: true,
		// The 079-max2 dump declares no foreign keys we rely on, and the
		// SQLite dev backend would otherwise need them declared in order.
		DisableForeignKeyConstraintWhenMigrating: true,
	}
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
	db, err := gorm.Open(mysql.Open(forceParseTime(cfg.DSN)), gormConfig())
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	if err := tunePool(db, cfg); err != nil {
		closeGORM(db)
		return nil, err
	}
	// Java applied this on every new connection (DatabaseConnection).
	if err := db.Exec("SET time_zone = '+08:00'").Error; err != nil {
		closeGORM(db)
		return nil, fmt.Errorf("database: set time_zone: %w", err)
	}
	return &DB{DB: db, driver: "mysql"}, nil
}

// tunePool applies the configured pool limits (GORM wraps database/sql, so
// the underlying pool is still the same one).
func tunePool(db *gorm.DB, cfg config.Database) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: pool handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)
	return nil
}

// isNotFound reports a "no row" result, covering both the translated and the
// raw driver error shapes.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// forceParseTime appends parseTime=true when the DSN omits it: without it
// the MySQL driver hands back []byte for DATETIME columns and every
// sql.NullTime scan (GetAccountState's lastlogin) fails at runtime with a
// confusing "unsupported Scan" error.
func forceParseTime(dsn string) string {
	if strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}

// openSQLite provisions a local SQLite DB: creates the parent directory and
// applies the schema DDL (SQLite translation of migrations/0001_base.sql) so
// a fresh file is immediately usable for login smoke tests.
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
	db, err := gorm.Open(sqlite.Open(cfg.DSN), gormConfig())
	if err != nil {
		return nil, fmt.Errorf("database: connect sqlite: %w", err)
	}
	if err := tunePool(db, cfg); err != nil {
		closeGORM(db)
		return nil, err
	}
	for _, stmt := range sqliteSchema {
		if err := db.Exec(stmt).Error; err != nil {
			closeGORM(db)
			return nil, fmt.Errorf("database: sqlite schema: %w", err)
		}
	}
	gdb := &DB{DB: db, driver: "sqlite"}
	// P3.5: replay the drop snapshot shipped with the 079MAX2 database so the
	// smoke backend drops the same items as production. No-op when the table
	// already has rows (operator tuning wins).
	if err := gdb.seedDrops(); err != nil {
		closeGORM(db)
		return nil, fmt.Errorf("database: sqlite drop seed: %w", err)
	}
	return gdb, nil
}

// closeGORM releases the underlying database/sql pool on an open failure.
func closeGORM(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
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
	// ipbans / macbans: P2.5 ban lists (migrations/0001_base.sql:1822/1911),
	// MySQL->SQLite translation. Empty by default = nobody is banned.
	`CREATE TABLE IF NOT EXISTS ipbans (
	  ipbanid INTEGER PRIMARY KEY AUTOINCREMENT,
	  ip TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS macbans (
	  macbanid INTEGER PRIMARY KEY AUTOINCREMENT,
	  mac TEXT NOT NULL
	)`,
	// drop_data / drop_data_global: P3.5 drop tables
	// (migrations/0001_base.sql:1114/1129), MySQL->SQLite translation.
	// Provisioned empty here, then filled from the embedded snapshot of the
	// authoritative 079-max2 database (see seedDrops in drops.go).
	`CREATE TABLE IF NOT EXISTS drop_data (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  dropperid INTEGER NOT NULL DEFAULT 0,
	  itemid INTEGER NOT NULL DEFAULT 0,
	  minimum_quantity INTEGER NOT NULL DEFAULT 1,
	  maximum_quantity INTEGER NOT NULL DEFAULT 1,
	  questid INTEGER NOT NULL DEFAULT 0,
	  chance INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS drop_data_dropperid ON drop_data(dropperid)`,
	`CREATE TABLE IF NOT EXISTS drop_data_global (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  continent INTEGER NOT NULL DEFAULT 0,
	  dropType INTEGER NOT NULL DEFAULT 0,
	  itemid INTEGER NOT NULL DEFAULT 0,
	  minimum_quantity INTEGER NOT NULL DEFAULT 1,
	  maximum_quantity INTEGER NOT NULL DEFAULT 1,
	  questid INTEGER NOT NULL DEFAULT 0,
	  chance INTEGER NOT NULL DEFAULT 0,
	  comments TEXT DEFAULT NULL
	)`,
	// configvalues: P4.5b ZEVMS admin-console switches
	// (migrations/0001_base.sql:1005 `CREATE TABLE configvalues (id, Name,
	// Val)`), MySQL->SQLite type translation. The column names keep the
	// dump's `Name`/`Val` capitalisation (both engines match them
	// case-insensitively).
	//
	// Deliberately NOT seeded: the production 079-max2 table carries the
	// operator's rows, while in dev an empty table has to mean "no switch
	// configured" - every switch reads 0, i.e. every feature is ON (the Java
	// convention is val > 0 = off). Seeding anything here would silently
	// change gameplay in the smoke backend.
	`CREATE TABLE IF NOT EXISTS configvalues (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  Name TEXT NOT NULL DEFAULT '',
	  Val INTEGER NOT NULL DEFAULT 0
	)`,
}

// Close releases the underlying pool. GORM's *gorm.DB has no Close of its
// own - it borrows database/sql's pool - so closing goes through db.DB().
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("database: pool handle: %w", err)
	}
	return sqlDB.Close()
}

// Ping checks the pool with a bounded context (Java: Connection.isValid).
func (db *DB) Ping(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("database: pool handle: %w", err)
	}
	return sqlDB.PingContext(ctx)
}
