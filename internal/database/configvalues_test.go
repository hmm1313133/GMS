package database

// P4.5b configvalues: the ZEVMS admin-console switch table (Java
// gui/Start.GetConfigValues). The SQLite dev backend provisions it empty on
// purpose - "no row" and "val = 0" both mean "feature enabled".

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSQLiteConfigValuesEmpty: a fresh SQLite database has the table but no
// rows, and that must read as an empty, non-nil map - the caller's lookups
// then all return 0 = every switch on.
func TestSQLiteConfigValuesEmpty(t *testing.T) {
	db, cancel := openTestSQLite(t)
	defer cancel()
	defer db.Close()

	ctx, to := context.WithTimeout(context.Background(), 10*time.Second)
	defer to()

	vals, err := db.ConfigValues(ctx)
	require.NoError(t, err)
	require.NotNil(t, vals, "empty table must yield a non-nil map (len 0)")
	require.Empty(t, vals)
}

// TestSQLiteConfigValuesRoundTrip checks the backticked `configvalues` query
// and the `Name`/`Val` column mapping, including the three chat-related keys
// the admin console writes (玩家聊天开关 id 2024, 聊天记录开关 id 2031,
// 游戏找人开关 id 2127) and a zero value.
func TestSQLiteConfigValuesRoundTrip(t *testing.T) {
	db, cancel := openTestSQLite(t)
	defer cancel()
	defer db.Close()

	ctx, to := context.WithTimeout(context.Background(), 10*time.Second)
	defer to()

	fixtures := []struct {
		id  int
		key string
		val int
	}{
		{2024, "玩家聊天开关", 1},
		{2031, "聊天记录开关", 0},
		{2127, "游戏找人开关", 1},
	}
	for _, f := range fixtures {
		require.NoError(t, db.WithContext(ctx).Exec(
			"INSERT INTO `configvalues` (id, Name, Val) VALUES (?, ?, ?)",
			f.id, f.key, f.val).Error)
	}

	vals, err := db.ConfigValues(ctx)
	require.NoError(t, err)
	require.Len(t, vals, len(fixtures))
	for _, f := range fixtures {
		got, ok := vals[f.key]
		require.True(t, ok, "key %q missing from the loaded map", f.key)
		require.Equal(t, f.val, got, "value of %q", f.key)
	}
	// The map is keyed by name and holds the raw value: the `val > 0 = off`
	// interpretation belongs to the caller (channel.Server.switchOn), not here.
	require.Equal(t, 0, vals["聊天记录开关"])
	require.Equal(t, 1, vals["玩家聊天开关"])
}
