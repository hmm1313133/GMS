package mapp

// P4.3b portal tests: the PortalFactory id rule (door portals are renumbered
// from 128 in document order), the attribute mapping, the empty-script
// normalisation and MapleMap.findClosestSpawnpoint.
//
// The fixture (testdata/wz/Map.wz/Map/Map9/900000002.img.xml, loaded through
// its info/link stub 900000001) holds seven portals whose wz node names are
// 0..6 and whose types are 0,1,2,6,6,7,10:
//
//	node  pn  pt  x    tm          script
//	0     sp  0   1    999999999   -
//	1     p1  1   100  100000000   -      (tn = p2)
//	2     p2  2   200  999999999   -
//	3     tp  6   30   (absent)    -      -> id 128
//	4     tp  6   31   (absent)    -      -> id 129
//	5     s1  7   40   (absent)    ""     (empty script == no script)
//	6     s2  10  50   (absent)    "r"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortals(t *testing.T) {
	d := loadFixture(t, fixtureFull)

	ps := d.Portals()
	require.Len(t, ps, 7, "every portal node loads exactly once")
	assert.Equal(t, []int{0, 1, 2, 128, 129, 5, 6}, portalIDs(ps),
		"ids are the node names, except the two pt=6 portals which become 128/129")

	// The two door portals drop their node name (3 and 4): Java's PortalFactory
	// assigns nextDoorPortal++ instead, so Portal(3)/Portal(4) do not exist.
	assert.Nil(t, d.Portal(3))
	assert.Nil(t, d.Portal(4))

	p0 := d.Portal(0)
	require.NotNil(t, p0)
	assert.Equal(t, Portal{ID: 0, Name: "sp", Type: 0, X: 1, Y: 2, TargetMap: NoTargetMap}, *p0,
		"pn/pt/x/y/tm/tn map onto the struct, with \"\" for the missing tn")

	p1 := d.Portal(1)
	require.NotNil(t, p1)
	assert.Equal(t, "p1", p1.Name)
	assert.Equal(t, 1, p1.Type)
	assert.Equal(t, 100, p1.X)
	assert.Equal(t, 100000000, p1.TargetMap)
	assert.Equal(t, "p2", p1.Target, "tn is the target portal NAME")

	p2 := d.Portal(2)
	require.NotNil(t, p2)
	assert.Equal(t, PortalMap, p2.Type)
	assert.Equal(t, NoTargetMap, p2.TargetMap)

	door1, door2 := d.Portal(128), d.Portal(129)
	require.NotNil(t, door1)
	require.NotNil(t, door2)
	assert.Equal(t, PortalDoor, door1.Type)
	assert.Equal(t, "tp", door1.Name)
	assert.Equal(t, 30, door1.X)
	assert.Equal(t, 31, door2.X, "the door counter follows document order")
	assert.Equal(t, 6, door2.Type)

	// script="" is normalised to "no script" (Java: script == null), a real
	// script survives.
	assert.Empty(t, d.Portal(5).Script, "an empty script attribute means no script")
	assert.Equal(t, "r", d.Portal(6).Script)

	// PortalByName keeps the FIRST match in document order - the two door
	// portals share pn="tp" and 128 comes first.
	assert.Equal(t, 128, d.PortalByName("tp").ID)
	assert.Equal(t, 0, d.PortalByName("sp").ID)
	assert.Nil(t, d.PortalByName("nope"))
	assert.Nil(t, d.Portal(999))

	// A nil MapData is safe (a map without data must not panic the callers).
	var nilData *MapData
	assert.Nil(t, nilData.Portal(0))
	assert.Nil(t, nilData.PortalByName("sp"))
	assert.Nil(t, nilData.FindClosestSpawnPoint(0, 0))
	assert.Empty(t, nilData.Portals())
}

func TestFindClosestSpawnPoint(t *testing.T) {
	d := loadFixture(t, fixtureFull)

	// Only type 0..2 portals that target NoTargetMap qualify; ties keep the
	// first portal in document order.
	//
	//	(0,0)   -> portal 0 at (1,2):   distanceSq 1+4 = 5
	//	           portal 2 at (200,0): distanceSq 40000  -> portal 0
	//	(30,0)  -> portal 3 at (30,0) has distanceSq 0 but is a DOOR portal
	//	           (pt=6 > PortalMap)  -> portal 0 (841+4) beats portal 2
	//	(100,0) -> portal 1 at (100,0) has distanceSq 0 but targets
	//	           100000000 != NoTargetMap -> portal 0 (9801+4) beats
	//	           portal 2 (10000)
	//	(300,0) -> portal 0 89401+4 vs portal 2 10000 -> portal 2
	assert.Equal(t, 0, d.FindClosestSpawnPoint(0, 0).ID)
	assert.Equal(t, 0, d.FindClosestSpawnPoint(30, 0).ID)
	assert.Equal(t, 0, d.FindClosestSpawnPoint(100, 0).ID)
	assert.Equal(t, 2, d.FindClosestSpawnPoint(300, 0).ID)

	// The closest portal wins even when it is farther than a rejected one.
	assert.Equal(t, 2, d.FindClosestSpawnPoint(201, 0).ID)
}

func portalIDs(ps []Portal) []int {
	out := make([]int, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}
