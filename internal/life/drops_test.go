package life

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"GMS/internal/database"
)

// fakeStore is the dropStore the provider is tested against: it counts
// queries so the cache behaviour (and the "no caching on error" rule) can be
// asserted without a database.
type fakeStore struct {
	mobs    map[int][]database.MonsterDropEntry
	globals []database.MonsterGlobalDropEntry
	err     error

	monsterCalls int32
	globalCalls  int32
}

func (f *fakeStore) MonsterDrops(ctx context.Context, monsterID int) ([]database.MonsterDropEntry, error) {
	atomic.AddInt32(&f.monsterCalls, 1)
	if f.err != nil {
		return nil, f.err
	}
	return f.mobs[monsterID], nil
}

func (f *fakeStore) GlobalDrops(ctx context.Context) ([]database.MonsterGlobalDropEntry, error) {
	atomic.AddInt32(&f.globalCalls, 1)
	if f.err != nil {
		return nil, f.err
	}
	return f.globals, nil
}

func newProvider(t *testing.T, s *fakeStore) *MonsterInformationProvider {
	t.Helper()
	p, err := NewMonsterInformationProvider(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRetrieveDropCaches(t *testing.T) {
	s := &fakeStore{mobs: map[int][]database.MonsterDropEntry{
		100100: {{ItemID: 4000019, Chance: 360000, Minimum: 1, Maximum: 1}},
	}}
	p := newProvider(t, s)
	ctx := context.Background()

	a := p.RetrieveDrop(ctx, 100100)
	if len(a) != 1 || a[0].ItemID != 4000019 || a[0].Chance != 360000 {
		t.Fatalf("first retrieve = %+v", a)
	}
	b := p.RetrieveDrop(ctx, 100100)
	if len(b) != 1 || b[0].Chance != 360000 {
		t.Fatalf("second retrieve = %+v", b)
	}
	if n := atomic.LoadInt32(&s.monsterCalls); n != 1 {
		t.Errorf("MonsterDrops called %d times, want 1 (cached)", n)
	}
}

// TestRetrieveDropEquipThird: Java divides an EQUIP row's chance by 3 on
// load (GameConstants.getInventoryType(itemid) == EQUIP). 1002067 is an
// equip, 4000019 is etc.
func TestRetrieveDropEquipThird(t *testing.T) {
	s := &fakeStore{mobs: map[int][]database.MonsterDropEntry{
		100100: {
			{ItemID: 4000019, Chance: 360000, Minimum: 1, Maximum: 1}, // etc: unchanged
			{ItemID: 1002067, Chance: 900, Minimum: 1, Maximum: 1},    // equip: 900/3
			{ItemID: 2000000, Chance: 20000, Minimum: 1, Maximum: 2},  // use: unchanged
			{ItemID: 0, Chance: 100, Minimum: 1, Maximum: 1},          // undefined: unchanged
		},
	}}
	p := newProvider(t, s)
	drops := p.RetrieveDrop(context.Background(), 100100)
	want := map[int]int{4000019: 360000, 1002067: 300, 2000000: 20000, 0: 100}
	for _, d := range drops {
		if d.Chance != want[d.ItemID] {
			t.Errorf("item %d chance = %d, want %d", d.ItemID, d.Chance, want[d.ItemID])
		}
	}
	if len(drops) != 4 {
		t.Fatalf("got %d drops, want 4", len(drops))
	}
}

// TestRetrieveDropErrorNotCached: Java skips drops.put on SQLException, so
// the next kill retries the query. An empty result for an unconfigured
// monster, on the other hand, IS cached.
func TestRetrieveDropErrorNotCached(t *testing.T) {
	s := &fakeStore{
		mobs: map[int][]database.MonsterDropEntry{999: {}},
		err:  errors.New("boom"),
	}
	p, err := NewMonsterInformationProvider(s)
	if err == nil {
		t.Fatal("NewMonsterInformationProvider should surface the global load error")
	}
	if p == nil {
		t.Fatal("provider must stay usable after a failed global load")
	}

	s.err = errors.New("db down")
	if got := p.RetrieveDrop(context.Background(), 100100); len(got) != 0 {
		t.Fatalf("retrieve after error = %+v, want empty", got)
	}
	if got := p.RetrieveDrop(context.Background(), 100100); len(got) != 0 {
		t.Fatalf("second retrieve = %+v, want empty", got)
	}
	if n := atomic.LoadInt32(&s.monsterCalls); n != 2 {
		t.Errorf("MonsterDrops called %d times, want 2 (errors are not cached)", n)
	}

	s.err = nil
	if got := p.RetrieveDrop(context.Background(), 999); len(got) != 0 {
		t.Fatalf("configured-but-empty monster = %+v, want empty", got)
	}
	p.RetrieveDrop(context.Background(), 999)
	if n := atomic.LoadInt32(&s.monsterCalls); n != 3 {
		t.Errorf("MonsterDrops called %d times, want 3 (empty list is cached)", n)
	}
}

func TestGlobalDropAndClear(t *testing.T) {
	s := &fakeStore{globals: []database.MonsterGlobalDropEntry{
		{ItemID: 4310149, Chance: 20000, Continent: 1, DropType: 1, Minimum: 1, Maximum: 1},
		{ItemID: 4006000, Chance: 50000, Continent: 1, DropType: 1, Minimum: 1, Maximum: 1},
	}}
	p := newProvider(t, s)
	if n := atomic.LoadInt32(&s.globalCalls); n != 1 {
		t.Fatalf("global drops loaded %d times during construction, want 1", n)
	}

	got := p.GlobalDrop()
	if len(got) != 2 || got[0].ItemID != 4310149 || got[0].DropType != 1 || got[0].Continent != 1 {
		t.Fatalf("GlobalDrop = %+v", got)
	}

	// Simulate an ops edit, then the reload path.
	s.globals = s.globals[:1]
	if err := p.ClearDrops(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := p.GlobalDrop(); len(got) != 1 {
		t.Fatalf("GlobalDrop after ClearDrops = %d rows, want 1", len(got))
	}
	if n := atomic.LoadInt32(&s.globalCalls); n != 2 {
		t.Errorf("global drops loaded %d times, want 2 after ClearDrops", n)
	}

	// ClearDrops must also forget the per-monster cache.
	s.mobs = map[int][]database.MonsterDropEntry{
		100100: {{ItemID: 1, Chance: 1, Minimum: 1, Maximum: 1}},
	}
	p.RetrieveDrop(context.Background(), 100100)
	p.RetrieveDrop(context.Background(), 100100)
	before := atomic.LoadInt32(&s.monsterCalls)
	if err := p.ClearDrops(context.Background()); err != nil {
		t.Fatal(err)
	}
	p.RetrieveDrop(context.Background(), 100100)
	if n := atomic.LoadInt32(&s.monsterCalls); n != before+1 {
		t.Errorf("MonsterDrops calls after ClearDrops = %d, want %d (cache not cleared)", n, before+1)
	}
}

// TestRetrieveDropConcurrent is the regression test for the Java data race
// (unsynchronised HashMap across netty workers): N goroutines hitting
// distinct monsters must each see exactly their own list.
func TestRetrieveDropConcurrent(t *testing.T) {
	const n = 64
	s := &fakeStore{mobs: map[int][]database.MonsterDropEntry{}}
	for i := 0; i < n; i++ {
		id := 100100 + i
		s.mobs[id] = []database.MonsterDropEntry{{ItemID: id, Chance: id, Minimum: 1, Maximum: 1}}
	}
	p := newProvider(t, s)

	done := make(chan error, n)
	for i := 0; i < n; i++ {
		id := 100100 + i
		go func() {
			for j := 0; j < 8; j++ {
				drops := p.RetrieveDrop(context.Background(), id)
				if len(drops) != 1 || drops[0].ItemID != id {
					done <- errors.New("wrong drop list for monster")
					return
				}
			}
			done <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestIsEquip(t *testing.T) {
	cases := map[int]bool{
		1002067: true,  // 帽子
		1402000: true,  // 武器
		4000019: false, // 其它
		2000000: false, // 消耗
		0:       false,
	}
	for id, want := range cases {
		if got := isEquip(id); got != want {
			t.Errorf("isEquip(%d) = %v, want %v", id, got, want)
		}
	}
}
