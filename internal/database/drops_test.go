package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"GMS/internal/config"
)

// TestSeedMatchesMigration guards the two copies of the drop snapshot: the
// one embedded here (SQLite provisioning) and migrations/0002_drop_data.sql
// (MySQL provisioning). tools/dropexport writes both from the same run, so
// any divergence means somebody edited one by hand or re-ran the tool for
// only one of them.
func TestSeedMatchesMigration(t *testing.T) {
	disk, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0002_drop_data.sql"))
	if err != nil {
		t.Skipf("migrations/0002_drop_data.sql not readable: %v", err)
	}
	if string(disk) != seedDropData {
		t.Fatal("seed_drop_data.sql and migrations/0002_drop_data.sql diverged; " +
			"re-run tools/dropexport for both (dropexport -out <path>)")
	}
}

// TestSQLiteDropSeed verifies the embedded 079-max2 drop snapshot is
// replayed into a fresh SQLite database (GMS-P3.5). The counts below are the
// authoritative database's own (tools/dropexport -stats).
func TestSQLiteDropSeed(t *testing.T) {
	db, cancel := openTestSQLite(t)
	defer cancel()
	defer db.Close()

	ctx, to := context.WithTimeout(context.Background(), 30*time.Second)
	defer to()

	monsters, rows, globals, err := db.DropStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if monsters != 900 || rows != 14243 || globals != 16 {
		t.Fatalf("drop stats = %d monsters / %d rows / %d globals, want 900 / 14243 / 16",
			monsters, rows, globals)
	}

	// 100100 (蜗牛) rows straight out of the authoritative database.
	got := map[int]int{}
	for _, e := range mustDrops(t, db, ctx, 100100) {
		got[e.ItemID] = e.Chance
	}
	for itemID, chance := range map[int]int{
		4000019: 360000,
		2000000: 20000,
		4010000: 9000,
		1002067: 900,
		2061009: 30,
	} {
		if got[itemID] != chance {
			t.Errorf("mob 100100 item %d chance = %d, want %d (absent=%v)",
				itemID, got[itemID], chance, got[itemID] == 0)
		}
	}
}

// TestSQLiteDropsUnknownMonster: a monster with no drop_data rows must yield
// an empty, non-nil slice (the drop loop ranges over it).
func TestSQLiteDropsUnknownMonster(t *testing.T) {
	db, cancel := openTestSQLite(t)
	defer cancel()
	defer db.Close()

	ctx, to := context.WithTimeout(context.Background(), 10*time.Second)
	defer to()

	drops := mustDrops(t, db, ctx, 999999999)
	if drops == nil {
		t.Fatal("MonsterDrops returned nil for an unknown monster, want empty slice")
	}
	if len(drops) != 0 {
		t.Fatalf("unknown monster has %d drops, want 0", len(drops))
	}
}

// TestSQLiteGlobalDrops checks the continent-wide table, including the
// `chance > 0` filter Java applied in SQL.
func TestSQLiteGlobalDrops(t *testing.T) {
	db, cancel := openTestSQLite(t)
	defer cancel()
	defer db.Close()

	ctx, to := context.WithTimeout(context.Background(), 10*time.Second)
	defer to()

	rows, err := db.GlobalDrops(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16 {
		t.Fatalf("global drops = %d, want 16", len(rows))
	}
	var saw4310149 bool
	for _, e := range rows {
		if e.Chance <= 0 {
			t.Errorf("global drop %d has chance %d, query must filter chance > 0", e.ItemID, e.Chance)
		}
		if e.ItemID == 4310149 {
			saw4310149 = true
			if e.Minimum != 1 || e.Maximum != 1 || e.QuestID != 0 {
				t.Errorf("global drop 4310149 = min %d max %d quest %d, want 1/1/0",
					e.Minimum, e.Maximum, e.QuestID)
			}
		}
	}
	if !saw4310149 {
		t.Error("global drop 4310149 missing from the snapshot")
	}

	// A zero-chance row must be filtered out (Java's WHERE clause).
	require.NoError(t, db.WithContext(ctx).Exec(
		"INSERT INTO drop_data_global (continent, dropType, itemid, chance) VALUES (9, 0, 9999999, 0)").Error)
	rows, err = db.GlobalDrops(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16 {
		t.Fatalf("global drops after inserting a chance=0 row = %d, want 16", len(rows))
	}
}

// TestSQLiteDropSeedIdempotent: re-opening an existing database must not
// duplicate or overwrite rows (operator tuning wins over the snapshot).
func TestSQLiteDropSeedIdempotent(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "drops.db")
	open := func() *DB {
		db, err := Open(config.Database{
			Driver: "sqlite", DSN: dsn,
			MaxOpenConns: 2, MaxIdleConns: 1, ConnMaxLifetimeSec: 60,
		})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}

	db := open()
	ctx := context.Background()
	require.NoError(t, db.WithContext(ctx).Exec(
		"INSERT INTO drop_data (dropperid, itemid, chance) VALUES (100100, 4000019, 1)").Error)
	db.Close()

	db = open()
	defer db.Close()
	_, rows, _, err := db.DropStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 14244 {
		t.Fatalf("drop_data rows after reopen = %d, want 14244 (seed not replayed twice)", rows)
	}
}

func mustDrops(t *testing.T, db *DB, ctx context.Context, mob int) []MonsterDropEntry {
	t.Helper()
	drops, err := db.MonsterDrops(ctx, mob)
	if err != nil {
		t.Fatal(err)
	}
	return drops
}

func openTestSQLite(t *testing.T) (*DB, context.CancelFunc) {
	t.Helper()
	db, err := Open(config.Database{
		Driver:             "sqlite",
		DSN:                filepath.Join(t.TempDir(), "test.db"),
		MaxOpenConns:       2,
		MaxIdleConns:       1,
		ConnMaxLifetimeSec: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, context.CancelFunc(func() {})
}
