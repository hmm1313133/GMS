package database

// P3.5 drop tables (GMS-P3.5, docs/PLAN.md), on GORM.
//
// Java sources (see docs/FILETRACK.md):
//   - server/life/MapleMonsterInformationProvider.retrieveDrop
//     -> MonsterDrops       (SELECT * FROM drop_data WHERE dropperid = ?)
//   - server/life/MapleMonsterInformationProvider.retrieveGlobal
//     -> GlobalDrops        (SELECT * FROM drop_data_global WHERE chance > 0)
//
// Data provenance (important): this server's drop rows are **third-party
// seeded**, they are not derived from the wz files. The authoritative copy
// lives in the 079MAX2 MySQL database
// (K:\079MAX2服务端\mysql\MySQL\data\079@002dmax2, 14,243 drop_data rows
// over 900 monsters + 16 global rows); tools/dropexport snapshots it into
// migrations/0002_drop_data.sql, which is embedded here as seedDropData so
// the SQLite smoke backend provisions the very same drops. See
// docs/SESSION_STATE.md §P3.5.
//
// Consequence: internal/life (the drop cache) reads this data as-is; the
// wz-side generator (tools/wztosql) is only a rebuild/diff aid and must not
// overwrite it.

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
)

// seedDropData is the tools/dropexport snapshot of the authoritative
// 079-max2 drop tables (migrations/0002_drop_data.sql, kept byte-identical).
// Embedded so the SQLite smoke backend provisions real drops with no external
// file and no MySQL 5.5 running.
//
//go:embed seed_drop_data.sql
var seedDropData string

// MonsterDrop is a row of `drop_data`.
type MonsterDrop struct {
	ID        int `gorm:"primaryKey;column:id;autoIncrement"`
	DropperID int `gorm:"column:dropperid;not null;default:0;index:drop_data_dropperid"`
	ItemID    int `gorm:"column:itemid;not null;default:0"`
	Minimum   int `gorm:"column:minimum_quantity;not null;default:1"`
	Maximum   int `gorm:"column:maximum_quantity;not null;default:1"`
	QuestID   int `gorm:"column:questid;not null;default:0"`
	Chance    int `gorm:"column:chance;not null;default:0"`
}

func (MonsterDrop) TableName() string { return "drop_data" }

// GlobalDrop is a row of `drop_data_global` (continent-wide drops).
type GlobalDrop struct {
	ID        int     `gorm:"primaryKey;column:id;autoIncrement"`
	Continent int     `gorm:"column:continent;not null;default:0;index:drop_data_global_continent"`
	DropType  int8    `gorm:"column:dropType;not null;default:0"`
	ItemID    int     `gorm:"column:itemid;not null;default:0"`
	Minimum   int     `gorm:"column:minimum_quantity;not null;default:1"`
	Maximum   int     `gorm:"column:maximum_quantity;not null;default:1"`
	QuestID   int     `gorm:"column:questid;not null;default:0"`
	Chance    int     `gorm:"column:chance;not null;default:0"`
	Comments  *string `gorm:"column:comments"`
}

func (GlobalDrop) TableName() string { return "drop_data_global" }

// MonsterDropEntry is Java server/life/MonsterDropEntry: the value object
// the drop cache consumes (kept separate from the table model so the cache
// does not depend on the storage layer).
type MonsterDropEntry struct {
	ItemID  int
	Chance  int
	Minimum int
	Maximum int
	QuestID int
}

// MonsterGlobalDropEntry is Java server/life/MonsterGlobalDropEntry.
//
// Continent is the Java "continent" column (0 = everywhere, higher values
// gate by region in MapleMap.dropFrom...); DropType is the `dropType` byte
// (0 = free-for-all, non-zero = owner-only in Java's onlySelf usage).
type MonsterGlobalDropEntry struct {
	ItemID    int
	Chance    int
	Continent int
	DropType  int8
	Minimum   int
	Maximum   int
	QuestID   int
}

// MonsterDrops loads the drop list of one monster (Java
// MapleMonsterInformationProvider.retrieveDrop). An unknown monster yields an
// empty (non-nil) slice, which is what the drop loop needs: it just rolls
// nothing.
//
// Note: the EQUIP chance /= 3 quirk from the Java provider is deliberately
// NOT applied here - it belongs to the caching layer (internal/life), next to
// the monster id key, so the DAO stays a faithful row mirror.
func (db *DB) MonsterDrops(ctx context.Context, monsterID int) ([]MonsterDropEntry, error) {
	var rows []MonsterDrop
	if err := db.WithContext(ctx).Where("dropperid = ?", monsterID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("database: drops for mob %d: %w", monsterID, err)
	}
	out := make([]MonsterDropEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, MonsterDropEntry{
			ItemID:  r.ItemID,
			Chance:  r.Chance,
			Minimum: r.Minimum,
			Maximum: r.Maximum,
			QuestID: r.QuestID,
		})
	}
	return out, nil
}

// GlobalDrops loads the continent-wide drop table (Java retrieveGlobal).
// Java filters `chance > 0` in SQL; kept as-is so a disabled row stays out.
func (db *DB) GlobalDrops(ctx context.Context) ([]MonsterGlobalDropEntry, error) {
	var rows []GlobalDrop
	if err := db.WithContext(ctx).Where("chance > 0").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("database: global drops: %w", err)
	}
	out := make([]MonsterGlobalDropEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, MonsterGlobalDropEntry{
			ItemID:    r.ItemID,
			Chance:    r.Chance,
			Continent: r.Continent,
			DropType:  r.DropType,
			Minimum:   r.Minimum,
			Maximum:   r.Maximum,
			QuestID:   r.QuestID,
		})
	}
	return out, nil
}

// DropStats reports how much drop data the backend currently holds. Used by
// cmd/gms for the startup log line and by tools/dropexport -verify.
func (db *DB) DropStats(ctx context.Context) (monsters int, rows int, globals int, err error) {
	var m, r, g int64
	if err = db.WithContext(ctx).Model(&MonsterDrop{}).Distinct("dropperid").Count(&m).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("database: drop monster count: %w", err)
	}
	if err = db.WithContext(ctx).Model(&MonsterDrop{}).Count(&r).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("database: drop row count: %w", err)
	}
	if err = db.WithContext(ctx).Model(&GlobalDrop{}).Count(&g).Error; err != nil {
		return 0, 0, 0, fmt.Errorf("database: global drop count: %w", err)
	}
	return int(m), int(r), int(g), nil
}

// seedDrops replays the embedded drop snapshot into a database whose
// drop_data table is still empty. It is a no-op once rows exist, so an
// operator's own tuning is never overwritten (the shipped database is the
// source of truth, not the other way round).
//
// Only the SQLite path calls it: MySQL deployments already run against the
// authoritative 079-max2 database.
func (db *DB) seedDrops() error {
	var n int64
	if err := db.Model(&MonsterDrop{}).Count(&n).Error; err != nil {
		return fmt.Errorf("database: count drop_data: %w", err)
	}
	if n > 0 {
		return nil
	}
	return db.execScript(seedDropData)
}

// execScript runs a `;`-separated SQL script, dropping full-line `--`
// comments (the only comment form tools/dropexport emits).
//
// Statements are executed one by one rather than handed to the driver in one
// call: MySQL only accepts multi-statement strings when the DSN opts in
// (multiStatements=true), and the seed file is 14k rows long.
func (db *DB) execScript(script string) error {
	var stmt strings.Builder
	flush := func() error {
		s := strings.TrimSpace(stmt.String())
		stmt.Reset()
		if s == "" {
			return nil
		}
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("database: exec %s: %w", firstLine(s), err)
		}
		return nil
	}
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		stmt.WriteString(line)
		stmt.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	// Paranoia: the seed must actually be there, otherwise the smoke DB would
	// silently drop nothing at all.
	var n int64
	if err := db.Model(&MonsterDrop{}).Count(&n).Error; err != nil {
		return fmt.Errorf("database: verify drop seed: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("database: drop seed applied but drop_data is still empty")
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " ..."
	}
	return s
}
