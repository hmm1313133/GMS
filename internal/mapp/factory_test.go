package mapp

// P4.3b factory/loader tests: MapImagePath, info/link resolution, the info
// flags, the life section, the factory cache/degradation contract and a smoke
// run over the real Map.wz export.
//
// Fixtures (internal/mapp/testdata/wz, hand written, 3.0 KB total):
//
//	Map/Map9/900000001.img  a link stub: info/returnMap=1, info/link="900000002"
//	Map/Map9/900000002.img  the real content: info flags, a top-level clock
//	                        node, seven portals, four footholds, two life
//	                        entries (see portal_test.go / foothold_test.go)
//	Map/Map9/900000003.img  info with nothing in it and no portal/foothold/
//	                        life sections at all
//
// The optional pn/tn/y/tm attributes and some values are shortened where the
// byte budget allowed (the Java defaults for them are asserted elsewhere).

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/wzs"
)

const (
	fixtureRoot = "testdata/wz"
	fixtureStub = 900000001
	fixtureFull = 900000002
	fixtureBare = 900000003
)

func TestMapImagePath(t *testing.T) {
	// Java getMapName: "Map/Map" + mapid/100000000 + "/" + leftPad(mapid, 9, '0') + ".img"
	cases := map[int]string{
		0:          "Map/Map0/000000000.img",
		10000:      "Map/Map0/000010000.img",
		100000000:  "Map/Map1/100000000.img",
		900000002:  "Map/Map9/900000002.img",
		910000000:  "Map/Map9/910000000.img",
		999999999:  "Map/Map9/999999999.img",
		1000000000: "Map/Map10/1000000000.img",
	}
	for id, want := range cases {
		assert.Equal(t, want, MapImagePath(id), "MapImagePath(%d)", id)
	}
}

func TestInfoFlags(t *testing.T) {
	d := loadFixture(t, fixtureFull)
	assert.Equal(t, fixtureFull, d.ID)
	assert.Equal(t, fixtureFull, d.ImageID)
	assert.Equal(t, 10000, d.ReturnMapID)
	assert.Equal(t, 1, d.ForcedReturnID)
	assert.Equal(t, 3, d.FieldLimit)
	assert.True(t, FieldLimitJump.Check(d.FieldLimit))
	assert.True(t, FieldLimitMovementSkills.Check(d.FieldLimit))
	assert.False(t, FieldLimitMysticDoor.Check(d.FieldLimit), "3 = Jump|MovementSkills only")
	assert.Equal(t, 5, d.FieldType)
	assert.Equal(t, 3, d.TimeLimit)
	assert.True(t, d.Town)
	assert.True(t, d.Everlast)
	assert.True(t, d.PersonalShop)
	assert.True(t, d.Soaring, "read from info/fly (this export has no info/needSkillForFly)")
	assert.True(t, d.Clock, "the TOP-LEVEL clock node")
	assert.Equal(t, 1.5, d.MobRate)
	assert.Equal(t, 20, d.MobCapacity, "info/fixedMobCapacity")
	assert.Equal(t, 1500, d.CreateMobInterval)
	assert.Equal(t, "e", d.OnEnter)
	assert.Equal(t, "f", d.OnFirstEnter)
}

// TestBareImage covers the degradation defaults: an image whose info node is
// empty and which has no portal/foothold/life section at all.
func TestBareImage(t *testing.T) {
	p := fixtureProvider(t)
	f, warns := newTestFactory(t, p)

	d, err := f.Load(fixtureBare)
	require.NoError(t, err, "a map without sections is not an error")

	assert.Equal(t, 0, d.ReturnMapID, "a missing info/returnMap yields 0 (Java NPEs)")
	assert.Equal(t, 1, warns.count("no info/returnMap"), "and warns once")
	assert.Equal(t, NoTargetMap, d.ForcedReturnID, "Java default 999999999")
	assert.Equal(t, -1, d.TimeLimit, "Java default -1")
	assert.Equal(t, 3000, d.CreateMobInterval, "Java default 3000")
	assert.Equal(t, 0.0, d.MobRate)
	assert.False(t, d.Clock)
	assert.Empty(t, d.Portals())
	assert.Empty(t, d.Life())
	assert.Empty(t, d.Mobs())
	assert.Empty(t, d.NPCs())
	assert.Equal(t, 0, d.Footholds().Len())
	_, ok := d.Footholds().FindBelow(0, 0)
	assert.False(t, ok, "no footholds means no floor, not a panic")
}

// TestLinkResolution asserts that a link stub is resolved to its target image
// while MapData.ID keeps the requested id.
func TestLinkResolution(t *testing.T) {
	p := fixtureProvider(t)
	d, err := LoadData(p, fixtureStub)
	require.NoError(t, err)

	assert.Equal(t, fixtureStub, d.ID, "ID stays the requested map id")
	assert.Equal(t, fixtureFull, d.ImageID, "ImageID records the image used")
	assert.Equal(t, 10000, d.ReturnMapID, "the TARGET's info is used, not the stub's 1")
	assert.Equal(t, 1.5, d.MobRate, "the target's mobRate")
	assert.Len(t, d.Portals(), 7, "the target's portals")
	assert.Equal(t, 4, d.Footholds().Len(), "the target's footholds")
	assert.Len(t, d.Life(), 2, "the target's life")
}

// TestLinkGuards covers the guards the loader needs for the guards Java does
// not have: link == 0, link == the map itself and a chained link (real data
// has no chains, so these are hand-written here).
func TestLinkGuards(t *testing.T) {
	p := tempProvider(t, map[string]string{
		"Map/Map9/900000020.img": `<imgdir name="900000020.img"><imgdir name="info"><int name="returnMap" value="7"/>` +
			`<string name="link" value="0"/></imgdir><imgdir name="portal"><imgdir name="0"><string name="pn" value="sp"/>` +
			`<int name="pt" value="0"/><int name="x" value="0"/><int name="y" value="0"/><int name="tm" value="999999999"/></imgdir></imgdir></imgdir>`,
		"Map/Map9/900000021.img": `<imgdir name="900000021.img"><imgdir name="info"><int name="returnMap" value="8"/>` +
			`<string name="link" value="900000021"/></imgdir></imgdir>`,
		"Map/Map9/900000022.img": `<imgdir name="900000022.img"><imgdir name="info"><int name="returnMap" value="9"/>` +
			`<string name="link" value="900000023"/></imgdir></imgdir>`,
		"Map/Map9/900000023.img": `<imgdir name="900000023.img"><imgdir name="info"><int name="returnMap" value="10"/>` +
			`<string name="link" value="900000024"/></imgdir></imgdir>`,
	})
	f, warns := newTestFactory(t, p)

	// link == 0: the link is ignored, the stub's own (tiny) image is used.
	d, err := f.Load(900000020)
	require.NoError(t, err)
	assert.Equal(t, 900000020, d.ImageID)
	assert.Equal(t, 7, d.ReturnMapID)
	assert.Len(t, d.Portals(), 1)
	assert.Equal(t, 1, warns.count("map link ignored"))

	// link == self: same guard, no recursion.
	d, err = f.Load(900000021)
	require.NoError(t, err)
	assert.Equal(t, 900000021, d.ImageID)
	assert.Equal(t, 8, d.ReturnMapID)
	assert.Equal(t, 2, warns.count("map link ignored"))

	// A chained link is followed exactly once (Java one-hop behaviour) and
	// reported instead of recursing.
	d, err = f.Load(900000022)
	require.NoError(t, err)
	assert.Equal(t, 900000023, d.ImageID, "one hop only")
	assert.Equal(t, 10, d.ReturnMapID, "the first target's info")
	assert.Equal(t, 1, warns.count("link target is itself a link"))

	// A link pointing at an image that does not exist is an error, not a panic.
	_, err = f.Load(900000099)
	require.Error(t, err)
}

func TestLifeParsing(t *testing.T) {
	d := loadFixture(t, fixtureFull)
	life := d.Life()
	require.Len(t, life, 2)

	mob := life[0]
	assert.Equal(t, byte('m'), mob.Kind)
	assert.Equal(t, 100, mob.ID)
	assert.Equal(t, 25, mob.X)
	assert.Equal(t, 150, mob.Y)
	assert.Equal(t, 1800, mob.MobTime, "mobTime stays in SECONDS")
	assert.True(t, mob.HasF, "the f node is present")
	assert.Equal(t, 0, mob.F)
	assert.Equal(t, 3, mob.Fh)
	assert.Equal(t, 20, mob.Cy)
	assert.Equal(t, -2, mob.Rx0)
	assert.Equal(t, 7, mob.Rx1)
	assert.False(t, mob.Hide, "hide applies to NPCs only")
	assert.Equal(t, int8(-1), mob.Team, "no team node -> Java's default -1")

	// Spawn position: Java addMonsterSpawn -> calcPointBelow(position), then
	// --y. For (25,150): fh2 (the slope 0..100) interpolates to int(24.99999)
	// = 24, which is above the query point, so fh3 (flat, y=200) is the floor
	// and the spawn sits at 200-1.
	x, y, ok := d.Footholds().CalcPointBelow(mob.X, mob.Y)
	require.True(t, ok)
	assert.Equal(t, 25, x)
	assert.Equal(t, 200, y)
	assert.Equal(t, 25, mob.SpawnX)
	assert.Equal(t, 199, mob.SpawnY, "one pixel above the foothold surface")

	npc := life[1]
	assert.Equal(t, byte('n'), npc.Kind)
	assert.Equal(t, 9010, npc.ID)
	assert.True(t, npc.Hide, "hide=1 on an NPC")
	assert.False(t, npc.HasF, "no f node -> HasF stays false (Java nil-guards it)")
	assert.Equal(t, 0, npc.MobTime, "no mobTime node -> Java's getInt default 0")
	assert.Equal(t, 25, npc.SpawnX, "an NPC keeps the raw position (Java addMapObject)")
	assert.Equal(t, 250, npc.SpawnY)

	require.Len(t, d.Mobs(), 1)
	require.Len(t, d.NPCs(), 1)
	assert.Equal(t, 100, d.Mobs()[0].ID)
	assert.Equal(t, 9010, d.NPCs()[0].ID)
}

// TestLifeDegradation covers the three life cases the real data cannot pin
// down: a monster with no foothold below it (Java NPEs), a life entry with an
// unknown type (Java prints and drops it) and the two tutorial NPCs Java
// removes from map 910000000.
func TestLifeDegradation(t *testing.T) {
	p := tempProvider(t, map[string]string{
		"Map/Map9/900000030.img": `<imgdir name="900000030.img"><imgdir name="info"><int name="returnMap" value="1"/></imgdir>` +
			`<imgdir name="life">` +
			`<imgdir name="0"><string name="type" value="M"/><string name="id" value="100"/><int name="x" value="900"/><int name="y" value="0"/>` +
			`<int name="mobTime" value="1800"/></imgdir>` +
			`<imgdir name="1"><string name="type" value="x"/><string name="id" value="200"/><int name="x" value="0"/><int name="y" value="0"/></imgdir>` +
			`<imgdir name="2"><string name="type" value="n"/><string name="id" value="?"/><int name="x" value="0"/><int name="y" value="0"/></imgdir>` +
			`</imgdir></imgdir>`,
		"Map/Map9/910000000.img": `<imgdir name="910000000.img"><imgdir name="info"><int name="returnMap" value="910000000"/></imgdir>` +
			`<imgdir name="life">` +
			`<imgdir name="0"><string name="type" value="n"/><string name="id" value="9310059"/><int name="x" value="0"/><int name="y" value="0"/></imgdir>` +
			`<imgdir name="1"><string name="type" value="n"/><string name="id" value="9310022"/><int name="x" value="0"/><int name="y" value="0"/></imgdir>` +
			`<imgdir name="2"><string name="type" value="n"/><string name="id" value="9010"/><int name="x" value="0"/><int name="y" value="0"/></imgdir>` +
			`</imgdir></imgdir>`,
	})
	f, warns := newTestFactory(t, p)

	d, err := f.Load(900000030)
	require.NoError(t, err)
	require.Len(t, d.Life(), 1, "the unknown type and the bad id are skipped")
	mob := d.Life()[0]
	assert.Equal(t, byte('m'), mob.Kind, "the life type compares case-insensitively")
	assert.Equal(t, 900, mob.SpawnX, "no foothold below -> the raw position is kept")
	assert.Equal(t, 0, mob.SpawnY)
	assert.Equal(t, 1, warns.count("unknown type"))
	assert.Equal(t, 1, warns.count("non-numeric id"))
	assert.Equal(t, 1, warns.count("no foothold below"))

	// map 910000000 drops exactly the two blocked NPCs.
	tut, err := f.Load(910000000)
	require.NoError(t, err)
	require.Len(t, tut.Life(), 1)
	assert.Equal(t, 9010, tut.Life()[0].ID)
}

// TestMonsterSpawnOnSlope pins the spawn y of a monster standing on a slope to
// the INTERPOLATED foothold y minus one (Java calcPointBelow, not y1).
func TestMonsterSpawnOnSlope(t *testing.T) {
	p := tempProvider(t, map[string]string{
		"Map/Map9/900000040.img": `<imgdir name="900000040.img"><imgdir name="info"><int name="returnMap" value="1"/></imgdir>` +
			`<imgdir name="foothold"><imgdir name="1"><imgdir name="0">` +
			`<imgdir name="1"><int name="x1" value="0"/><int name="y1" value="0"/><int name="x2" value="100"/><int name="y2" value="100"/></imgdir>` +
			`</imgdir></imgdir></imgdir>` +
			`<imgdir name="life"><imgdir name="0"><string name="type" value="m"/><string name="id" value="100"/>` +
			`<int name="x" value="50"/><int name="y" value="0"/><int name="mobTime" value="1800"/></imgdir></imgdir></imgdir>`,
	})
	d, err := LoadData(p, 900000040)
	require.NoError(t, err)
	require.Len(t, d.Mobs(), 1)
	// s1=s2=100, s4=50 -> s5 = 49.99999999999999289 -> (int) 49 -> calcY =
	// 0+49 = 49 -> the spawn sits one pixel above it, at 48. The ideal
	// geometric value (50) is one pixel lower; Java's (int) cast truncates.
	assert.Equal(t, 50, d.Mobs()[0].SpawnX)
	assert.Equal(t, 48, d.Mobs()[0].SpawnY)
}

func TestFactoryCachesAndDegrades(t *testing.T) {
	p := fixtureProvider(t)
	f, warns := newTestFactory(t, p)

	// Same pointer on every Load, and Data sees it afterwards.
	d1, err := f.Load(fixtureFull)
	require.NoError(t, err)
	d2, err := f.Load(fixtureFull)
	require.NoError(t, err)
	assert.Same(t, d1, d2, "the factory caches the loaded data")
	assert.Same(t, d1, f.Data(fixtureFull))

	// A never-loaded id has no data.
	assert.Nil(t, f.Data(123456789))
	assert.Nil(t, f.Data(fixtureStub))
	// ...but Data never loads: loading the stub afterwards works and both ids
	// then have their own (different) data.
	stub, err := f.Load(fixtureStub)
	require.NoError(t, err)
	assert.NotSame(t, d1, stub, "a link stub gets its own MapData (different ID/ImageID)")
	assert.Same(t, stub, f.Data(fixtureStub))

	// A missing image: error, nil data, one warning, cached error.
	_, err1 := f.Load(424242)
	require.Error(t, err1)
	_, err2 := f.Load(424242)
	require.Error(t, err2)
	assert.Equal(t, err1, err2, "the negative cache returns the same error value")
	assert.Contains(t, err1.Error(), "Map/Map0/000424242.img")
	assert.Nil(t, f.Data(424242), "a failed load caches no data")
	assert.Equal(t, 1, warns.count("map image unavailable"), "warned exactly once")

	// A factory without a provider never panics.
	nilProvider := NewFactory(nil, warns.logger())
	_, err = nilProvider.Load(fixtureFull)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Map.wz provider")
	assert.Nil(t, nilProvider.Data(fixtureFull))

	// A nil logger must not panic either.
	noLog := NewFactory(p, nil)
	_, err = noLog.Load(fixtureFull)
	require.NoError(t, err)

	// Nil factory: Load errors, Data and Data-on-unknown stay nil.
	var nilFactory *Factory
	assert.Nil(t, nilFactory.Data(fixtureFull))
	_, err = nilFactory.Load(fixtureFull)
	require.Error(t, err)

	// Concurrent first loads hand out one shared instance.
	f2, _ := newTestFactory(t, p)
	var wg sync.WaitGroup
	got := make([]*MapData, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], _ = f2.Load(fixtureFull)
		}(i)
	}
	wg.Wait()
	for _, g := range got {
		require.NotNil(t, g)
		assert.Same(t, got[0], g)
	}
}

// TestMap0Fixture loads map 0 from the shared wz test fixture
// (internal/wzs/testdata/wz) instead of the mapp one, so both packages assert
// against the same bytes.
func TestMap0Fixture(t *testing.T) {
	root, err := wzs.OpenRoot("../wzs/testdata/wz")
	require.NoError(t, err)
	p, err := root.WZ("Map.wz")
	require.NoError(t, err)
	d, err := LoadData(p, 0)
	require.NoError(t, err)

	assert.Equal(t, 0, d.ID)
	assert.Equal(t, 0, d.ImageID)
	assert.Equal(t, 10000, d.ReturnMapID)
	assert.Equal(t, 10000, d.ForcedReturnID)
	assert.Equal(t, 0, d.FieldLimit)
	assert.Equal(t, 1.0, d.MobRate)
	assert.False(t, d.Clock, "map 0 has no clock node")
	assert.False(t, d.Town)
	assert.False(t, d.Soaring)

	require.Len(t, d.Portals(), 1)
	assert.Equal(t, Portal{ID: 0, Name: "sp", Type: 0, X: 0, Y: 0, TargetMap: NoTargetMap}, d.Portals()[0])
	assert.Equal(t, 0, d.PortalByName("sp").ID)

	assert.Empty(t, d.Life(), "the life section is empty")
	assert.Len(t, d.Mobs(), 0)
	assert.Len(t, d.NPCs(), 0)

	// 13 footholds: two walls (ids 1 and 13, x1 == x2 == -399 / 399) and eleven
	// flat segments at y=95 covering -399..399.
	tree := d.Footholds()
	require.Equal(t, 13, tree.Len())
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}, footholdIDs(tree.All()))
	assert.True(t, tree.All()[0].IsWall())
	assert.True(t, tree.All()[12].IsWall())

	// Hand-derived: at x=0 the candidates are fh6 (0..90, y=95) and fh7
	// (-90..0, y=95); both are flat at y=95, compareTo returns 0 both ways and
	// the stable sort keeps document order, so fh6 (the earlier node) wins.
	f, ok := tree.FindBelow(0, 0)
	require.True(t, ok)
	assert.Equal(t, 6, f.ID)
	_, y, ok := tree.CalcPointBelow(0, 0)
	require.True(t, ok)
	assert.Equal(t, 95, y)
}

// TestRealMapTree loads real Map.wz images (the gitignored 079MAX2 export at
// ../../wz) and asserts the loader never fails and the structural invariants
// hold. Skipped when the export is not present.
func TestRealMapTree(t *testing.T) {
	const realRoot = "../../wz"
	if _, err := os.Stat(realRoot); err != nil {
		t.Skipf("full wz export not present at %s", realRoot)
	}
	root, err := wzs.OpenRoot(realRoot)
	require.NoError(t, err)
	p, err := root.WZ("Map.wz")
	require.NoError(t, err)
	f, _ := newTestFactory(t, p)

	// A deterministic sample: the first images of the provider's tree (Map0
	// first, then Map1, ...). Parsed images are purged periodically so the
	// smoke run does not hold every XML tree in memory.
	var ids []int
	var walk func(dir *wzs.DirEntry)
	walk = func(dir *wzs.DirEntry) {
		for _, file := range dir.Files() {
			id, err := strconv.Atoi(strings.TrimSuffix(file.Name, ".img"))
			if err != nil {
				continue // e.g. the top-level Physics.img
			}
			ids = append(ids, id)
			if len(ids) >= 60 {
				return
			}
		}
		for _, sub := range dir.Subdirectories() {
			walk(sub)
			if len(ids) >= 60 {
				return
			}
		}
	}
	walk(p.Root())
	require.GreaterOrEqual(t, len(ids), 20, "the sample should cover several map folders")
	t.Logf("sampling %d real map images", len(ids))

	var footholds, mobs, resolved int
	for i, id := range ids {
		if i > 0 && i%10 == 0 {
			p.Purge()
		}
		d, err := f.Load(id)
		require.NoErrorf(t, err, "map %d (%s)", id, MapImagePath(id))
		require.NotNil(t, d)
		assert.Equalf(t, id, d.ID, "the requested id is kept")

		tree := d.Footholds()
		footholds += tree.Len()
		for _, fh := range tree.All() {
			if fh.X1 >= fh.X2 {
				// walls (x1 == x2) and the right-to-left segments Java's
				// findBelow can never match (it requires x1 <= x <= x2).
				continue
			}
			// At a foothold's own left end the surface is exactly y1 (s4 = 0),
			// so that foothold is a candidate that cannot be skipped: the
			// search must return something spanning x and never a wall.
			got, ok := tree.FindBelow(fh.X1, fh.Y1)
			require.Truef(t, ok, "map %d: no foothold below (%d,%d) despite fh %d spanning it", id, fh.X1, fh.Y1, fh.ID)
			assert.Truef(t, got.X1 <= fh.X1 && fh.X1 <= got.X2, "map %d: winner %d does not span x=%d", id, got.ID, fh.X1)
			assert.Falsef(t, got.IsWall(), "map %d: a wall won findBelow", id)
			resolved++
		}

		for _, m := range d.Mobs() {
			mobs++
			// addMonsterSpawn: calcPointBelow(position) - 1, or the raw
			// position when Java would have thrown an NPE.
			if _, y, ok := tree.CalcPointBelow(m.X, m.Y); ok {
				assert.Equalf(t, y-1, m.SpawnY, "map %d mob %d", id, m.ID)
				assert.Equalf(t, m.X, m.SpawnX, "map %d mob %d", id, m.ID)
			} else {
				assert.Equalf(t, m.Y, m.SpawnY, "map %d mob %d (no foothold below)", id, m.ID)
			}
		}
	}
	assert.Positive(t, footholds)
	assert.Positive(t, resolved)

	// Map 100000000 (Henesys) is the spec's door-portal example: six pt=6
	// portals whose wz node names are 28..33 become ids 128..133, so 28..33
	// are empty.
	henesys, err := f.Load(100000000)
	require.NoError(t, err)
	doors := 0
	for _, portal := range henesys.Portals() {
		if portal.Type == PortalDoor {
			doors++
		}
	}
	assert.Equal(t, 6, doors)
	for id := 28; id <= 33; id++ {
		assert.Nilf(t, henesys.Portal(id), "portal %d must not exist on 100000000", id)
	}
	for id := 128; id <= 133; id++ {
		require.NotNilf(t, henesys.Portal(id), "portal %d must exist on 100000000", id)
		assert.Equal(t, PortalDoor, henesys.Portal(id).Type)
	}
	assert.Nil(t, henesys.Portal(134))

	// A real link stub: 108000709 only carries info/link=108000700.
	stub, err := f.Load(108000709)
	require.NoError(t, err)
	assert.Equal(t, 108000709, stub.ID)
	assert.Equal(t, 108000700, stub.ImageID)
	assert.Equal(t, 140030000, stub.ReturnMapID, "the linked image's returnMap")
	assert.NotEmpty(t, stub.Portals())
	assert.Positive(t, stub.Footholds().Len())

	// A real map with monsters: every sample resolves and the spawns sit on a
	// foothold.
	d, err := f.Load(103000804)
	require.NoError(t, err)
	require.NotEmpty(t, d.Mobs())
	for _, m := range d.Mobs() {
		_, y, ok := d.Footholds().CalcPointBelow(m.X, m.Y)
		require.Truef(t, ok, "mob %d at (%d,%d) has no foothold below", m.ID, m.X, m.Y)
		assert.Equal(t, y-1, m.SpawnY)
	}
}

// ---------------------------------------------------------------------------
// test helpers
// ---------------------------------------------------------------------------

// loadFixture loads one of the committed mapp fixtures.
func loadFixture(t *testing.T, id int) *MapData {
	t.Helper()
	d, err := LoadData(fixtureProvider(t), id)
	require.NoError(t, err)
	return d
}

func fixtureProvider(t *testing.T) *wzs.Provider {
	t.Helper()
	root, err := wzs.OpenRoot(fixtureRoot)
	require.NoError(t, err)
	p, err := root.WZ("Map.wz")
	require.NoError(t, err)
	return p
}

// tempProvider writes a Map.wz tree into a temporary directory (files maps a
// wz-relative image path to its XML) and opens it.
func tempProvider(t *testing.T, files map[string]string) *wzs.Provider {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(dir, "Map.wz", filepath.FromSlash(rel)+".xml")
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}
	root, err := wzs.OpenRoot(dir)
	require.NoError(t, err)
	p, err := root.WZ("Map.wz")
	require.NoError(t, err)
	return p
}

// warnRecorder is a slog.Handler that keeps every record so tests can assert
// the warning contract (one warning per degradation, never a panic).
type warnRecorder struct {
	mu   sync.Mutex
	recs []string
}

func (h *warnRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (h *warnRecorder) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recs = append(h.recs, r.Message)
	return nil
}

func (h *warnRecorder) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *warnRecorder) WithGroup(string) slog.Handler      { return h }

// count returns how many records carry substr in their message.
func (h *warnRecorder) count(substr string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, m := range h.recs {
		if strings.Contains(m, substr) {
			n++
		}
	}
	return n
}

func (h *warnRecorder) logger() *slog.Logger { return slog.New(h) }

func newTestFactory(t *testing.T, p *wzs.Provider) (*Factory, *warnRecorder) {
	t.Helper()
	rec := &warnRecorder{}
	return NewFactory(p, rec.logger()), rec
}
