// Package life is the Go home of the server.life tree (docs/FILETRACK.md).
// P3.5 lands the first piece: the monster drop cache, i.e. the runtime
// consumer of the drop tables.
package life

// Port of Java server/life/MapleMonsterInformationProvider (GMS-P3.5).
//
// Two things this file is careful about:
//
//  1. Data provenance. The 079MAX2 distribution ships hand-tuned drop_data
//     rows (14,243 rows over 900 monsters) that are NOT derived from the wz
//     files. The database is the source of truth; tools/wztosql can rebuild
//     a wz-derived drop list for comparison but must not overwrite it. See
//     internal/database/drops.go and docs/SESSION_STATE.md §P3.5.
//
//  2. The EQUIP chance quirk. Java divides an EQUIP row's chance by 3 while
//     loading it (MapleMonsterInformationProvider.retrieveDrop), so the
//     stored value is not the effective one. Kept faithful - the /3 is what
//     the live server has always rolled with.

import (
	"context"
	"sync"

	"GMS/internal/database"
)

// DropEntry is Java server/life/MonsterDropEntry.
type DropEntry struct {
	ItemID  int
	Chance  int
	Minimum int
	Maximum int
	QuestID int
}

// GlobalDropEntry is Java server/life/MonsterGlobalDropEntry as loaded by
// retrieveGlobal: the 7-arg constructor leaves onlySelf=false, so the field
// is not modelled here (MapleMap uses `de.onlySelf ? 0 : droptype`, i.e.
// always the map's own drop type for database-driven globals).
type GlobalDropEntry struct {
	ItemID    int
	Chance    int
	Continent int
	DropType  int8
	Minimum   int
	Maximum   int
	QuestID   int
}

// dropStore is the slice of *database.DB the provider needs; kept as an
// interface so tests can run without a database (see drops_test.go).
type dropStore interface {
	MonsterDrops(ctx context.Context, monsterID int) ([]database.MonsterDropEntry, error)
	GlobalDrops(ctx context.Context) ([]database.MonsterGlobalDropEntry, error)
}

// MonsterInformationProvider caches per-monster drop lists, keyed by monster
// id, plus the continent-wide global drop list.
//
// Deviation (intentional): Java used a plain HashMap/LinkedList from multiple
// netty worker threads with no synchronisation - a real data race on first
// sight of each monster. Go guards both with a RWMutex; behaviour is
// identical, minus the race.
type MonsterInformationProvider struct {
	store dropStore

	mu     sync.RWMutex
	drops  map[int][]DropEntry
	global []GlobalDropEntry
}

// NewMonsterInformationProvider builds the provider and, like the Java
// constructor, eagerly loads the global drop list. Errors are returned
// (Java swallowed them into an empty list + stderr print) so the caller can
// decide; the provider stays usable either way.
func NewMonsterInformationProvider(store dropStore) (*MonsterInformationProvider, error) {
	p := &MonsterInformationProvider{
		store:  store,
		drops:  make(map[int][]DropEntry),
		global: []GlobalDropEntry{},
	}
	return p, p.reloadGlobal(context.Background())
}

// RetrieveDrop returns the (possibly empty) drop list of one monster,
// caching it on success. On a database error it returns an empty slice
// without caching - same as Java, which skips drops.put on SQLException so
// the next kill retries the query.
func (p *MonsterInformationProvider) RetrieveDrop(ctx context.Context, monsterID int) []DropEntry {
	p.mu.RLock()
	if e, ok := p.drops[monsterID]; ok {
		p.mu.RUnlock()
		return e
	}
	p.mu.RUnlock()

	rows, err := p.store.MonsterDrops(ctx, monsterID)
	if err != nil {
		return []DropEntry{}
	}
	out := make([]DropEntry, 0, len(rows))
	for _, r := range rows {
		chance := r.Chance
		if isEquip(r.ItemID) {
			chance /= 3 // Java: GameConstants.getInventoryType(itemid) == EQUIP
		}
		out = append(out, DropEntry{
			ItemID:  r.ItemID,
			Chance:  chance,
			Minimum: r.Minimum,
			Maximum: r.Maximum,
			QuestID: r.QuestID,
		})
	}

	p.mu.Lock()
	p.drops[monsterID] = out
	p.mu.Unlock()
	return out
}

// GlobalDrop returns the continent-wide drop list.
func (p *MonsterInformationProvider) GlobalDrop() []GlobalDropEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.global
}

// ClearDrops drops both caches and reloads the global list (Java
// clearDrops()). Called from the ops reload path so drop edits take effect
// without a restart.
func (p *MonsterInformationProvider) ClearDrops(ctx context.Context) error {
	p.mu.Lock()
	p.drops = make(map[int][]DropEntry)
	p.mu.Unlock()
	return p.reloadGlobal(ctx)
}

func (p *MonsterInformationProvider) reloadGlobal(ctx context.Context) error {
	rows, err := p.store.GlobalDrops(ctx)
	out := make([]GlobalDropEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, GlobalDropEntry{
			ItemID:    r.ItemID,
			Chance:    r.Chance,
			Continent: r.Continent,
			DropType:  r.DropType,
			Minimum:   r.Minimum,
			Maximum:   r.Maximum,
			QuestID:   r.QuestID,
		})
	}
	p.mu.Lock()
	p.global = out
	p.mu.Unlock()
	return err
}

// isEquip is GameConstants.getInventoryType(itemId) == MapleInventoryType.EQUIP:
// the leading digit of the item id is 1 (1=EQUIP, 2=USE, 3=SETUP, 4=ETC,
// 5=CASH; anything else is UNDEFINED).
func isEquip(itemID int) bool {
	return itemID/1000000 == 1
}
