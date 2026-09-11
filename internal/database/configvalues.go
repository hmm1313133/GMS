package database

// P4.5b configvalues: the ZEVMS admin-console gameplay switches, on GORM.
//
// Java sources (see docs/FILETRACK.md):
//   - gui/Start.GetConfigValues (called once from Start.startServer)
//     SELECT name, val FROM ConfigValues -> Start.ConfigValuesMap
//   - gui/控制台/控制台1号.java writes the rows (玩家聊天开关 id 2024,
//     聊天记录开关 id 2031, 游戏找人开关 id 2127)
//
// Convention, uniform across the Java tree: **val > 0 means the feature is
// OFF**, 0 means enabled. A key that is absent from the map therefore reads
// 0 = enabled, which is exactly what an empty table has to mean here: the Go
// SQLite dev backend provisions the table empty on purpose (see sqliteSchema
// in db.go), and the production 079-max2 database ships it populated (322
// rows).

import (
	"context"
	"fmt"
)

// ConfigValues ports gui/Start.GetConfigValues: SELECT name, val FROM ConfigValues.
//
// The whole (admin-sized) table is loaded into a map, which is what the Java
// static ConfigValuesMap is. An empty table yields an empty, non-nil map - the
// caller reads "no switch configured" as "every feature enabled".
//
// The table name is quoted with backticks. MySQL accepts them natively and
// SQLite (the dev backend) accepts them for MySQL compatibility, so one
// statement serves both drivers; `configvalues` is close enough to the
// reserved VALUES/`values` family that the quoting is kept even though the
// current MySQL grammar would parse it bare.
func (db *DB) ConfigValues(ctx context.Context) (map[string]int, error) {
	rows, err := db.WithContext(ctx).Raw("SELECT name, val FROM `configvalues`").Rows()
	if err != nil {
		return nil, fmt.Errorf("database: load configvalues: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var (
			name string
			val  int
		)
		if err := rows.Scan(&name, &val); err != nil {
			return nil, fmt.Errorf("database: scan configvalues row: %w", err)
		}
		out[name] = val
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: read configvalues: %w", err)
	}
	return out, nil
}

// SetConfigValue writes one admin switch (the Java console's 按键开关: it updates
// the row with that id, or inserts one when the key is new). Switch names are
// operator-facing Chinese keys, so callers pass them through unchanged.
//
// The row id is auto-assigned by the column, and an existing row is matched by
// name - the production schema declares id as the primary key and the console
// looks rows up by name, so name is the effective key here too.
func (db *DB) SetConfigValue(ctx context.Context, name string, val int) error {
	var count int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM `configvalues` WHERE name = ?", name).
		Row().Scan(&count); err != nil {
		return fmt.Errorf("database: count configvalue %q: %w", name, err)
	}
	var sql string
	if count > 0 {
		sql = "UPDATE `configvalues` SET val = ? WHERE name = ?"
	} else {
		sql = "INSERT INTO `configvalues` (name, val) VALUES (?, ?)"
	}
	args := []any{val, name}
	if count == 0 {
		args = []any{name, val}
	}
	if err := db.WithContext(ctx).Exec(sql, args...).Error; err != nil {
		return fmt.Errorf("database: write configvalue %q: %w", name, err)
	}
	return nil
}

// DeleteConfigValue removes a switch row so the key falls back to "absent"
// (which every reader treats as 0 = enabled).
func (db *DB) DeleteConfigValue(ctx context.Context, name string) error {
	if err := db.WithContext(ctx).Exec("DELETE FROM `configvalues` WHERE name = ?", name).Error; err != nil {
		return fmt.Errorf("database: delete configvalue %q: %w", name, err)
	}
	return nil
}
