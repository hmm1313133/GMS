package mapp

// P4.3b foothold tests: the document-order flattening of
// foothold/<group>/<layer>/<fhId>, and MapleFootholdTree.findBelow /
// MapleMap.calcPointBelow.
//
// Every expectation below is derived BY HAND from the fixture (see
// testdata/wz/Map.wz/Map/Map9/900000002.img.xml), which holds four footholds
// in three (group, layer) buckets:
//
//	fh 1  wall   (-100,100)-(-100,50)      group 1 layer 0
//	fh 2  slope  (0,0)-(100,100)           group 1 layer 0   (next=3)
//	fh 3  flat   (0,200)-(100,200)         group 1 layer 0   (prev=2)
//	fh 4  flat   (50,200)-(150,200)        group 2 layer 0
//
// Input / candidate set / winner for each case is spelled out in the test.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFootholdsParsed(t *testing.T) {
	d := loadFixture(t, fixtureFull)
	tree := d.Footholds()
	require.NotNil(t, tree)

	all := tree.All()
	require.Len(t, all, 4, "group/layer nesting flattens to one flat list")
	assert.Equal(t, []int{1, 2, 3, 4}, footholdIDs(all), "document order: group 1 layer 0, then group 2 layer 0")

	assert.Equal(t, Foothold{ID: 1, X1: -100, Y1: 100, X2: -100, Y2: 50}, all[0])
	assert.Equal(t, Foothold{ID: 2, X1: 0, Y1: 0, X2: 100, Y2: 100, Next: 3}, all[1])
	assert.Equal(t, Foothold{ID: 3, X1: 0, Y1: 200, X2: 100, Y2: 200, Prev: 2}, all[2])
	assert.Equal(t, Foothold{ID: 4, X1: 50, Y1: 200, X2: 150, Y2: 200}, all[3])
	assert.Equal(t, 4, tree.Len())

	// isWall is X1 == X2: the vertical segment is not walkable.
	assert.True(t, all[0].IsWall())
	assert.False(t, all[1].IsWall())
	assert.False(t, all[3].IsWall())

	// A nil tree is safe.
	var nilTree *FootholdTree
	assert.Equal(t, 0, nilTree.Len())
	assert.Empty(t, nilTree.All())
	_, ok := nilTree.FindBelow(0, 0)
	assert.False(t, ok)
}

func TestFootholdFindBelow(t *testing.T) {
	tree := loadFixture(t, fixtureFull).Footholds()

	// (1) The wall is filtered out: X1 == X2 never matches, and at x = -100
	//     nothing else has -100 inside its span (fh 2/3 start at 0, fh 4 at
	//     50), so the candidate set is EMPTY and there is no floor.
	_, ok := tree.FindBelow(-100, 0)
	assert.False(t, ok, "a query at the wall's own x finds nothing")

	// (2) x=50, y=0: candidates {fh2 (0..100), fh3 (0..100), fh4 (50..150)}.
	//     Ordering by MapleFoothold.compareTo: fh2.y2=100 < fh3.y1=200 -> -1
	//     and fh2.y2 < fh4.y1 -> -1, so fh2 first; fh3 vs fh4 both span y=200
	//     (neither y2 < y1 nor y1 > y2) -> 0 -> stable sort keeps fh3 first.
	//     fh2 is the slope: s1=|100-0|=100, s2=|100-0|=100, s4=|50-0|=50, so
	//     s5 = cos(atan(1)) * (50/cos(atan(1))) - which in IEEE-754 doubles is
	//     49.99999999999999289, NOT 50: the 45-degree case always lands one
	//     ulp below the ideal. Java's (int) cast truncates towards zero, so
	//     calcY = y1 + 49 = 49. 49 >= 0 -> fh2 wins. (This pixel-sized
	//     quirk is Java's; the Go port reproduces the same expression.)
	f, ok := tree.FindBelow(50, 0)
	require.True(t, ok)
	assert.Equal(t, 2, f.ID, "the slope under the query wins")
	x, y, ok := tree.CalcPointBelow(50, 0)
	require.True(t, ok)
	assert.Equal(t, 50, x)
	assert.Equal(t, 49, y, "interpolated slope y at half of the 100x100 slope, truncated like Java")

	// (3) x=50, y=60: same order, but the slope's surface (49) is ABOVE the
	//     query point (49 < 60), so it is skipped and the flat fh3 at y=200
	//     becomes the floor.
	f, ok = tree.FindBelow(50, 60)
	require.True(t, ok)
	assert.Equal(t, 3, f.ID, "a foothold above the query point is not a floor")
	x, y, ok = tree.CalcPointBelow(50, 60)
	require.True(t, ok)
	assert.Equal(t, 50, x)
	assert.Equal(t, 200, y, "a flat foothold's drop y is y1")

	// (4) x=75, y=200: candidates {fh3, fh4}, both flat at y=200 and the
	//     comparator returns 0 both ways, so the stable sort keeps document
	//     order and fh3 wins. At x=120 fh3 (x2=100) is out of range and fh4
	//     wins instead - the x span is inclusive on both ends.
	f, ok = tree.FindBelow(75, 200)
	require.True(t, ok)
	assert.Equal(t, 3, f.ID, "a tie keeps document order")
	f, ok = tree.FindBelow(120, 200)
	require.True(t, ok)
	assert.Equal(t, 4, f.ID, "x1 <= x <= x2 is inclusive at both ends")

	// (5) x=100, y=250: candidates {fh3, fh4} are both flat at y=200, i.e.
	//     above the query point (y1 < y) -> skipped, no floor.
	_, ok = tree.FindBelow(100, 250)
	assert.False(t, ok, "both flats are above the query point")

	// (6) No foothold spans x=1000 at all.
	_, ok = tree.FindBelow(1000, 0)
	assert.False(t, ok)
	_, _, ok = tree.CalcPointBelow(1000, 0)
	assert.False(t, ok, "calcPointBelow reports a missing floor instead of panicking")
}

// TestFootholdUpSlope covers the y2 < y1 branch of the interpolation (a slope
// rising to the right) plus the "wall only" and "flat only" degenerate maps,
// which the shared fixture does not exercise.
func TestFootholdUpSlope(t *testing.T) {
	p := tempProvider(t, map[string]string{
		// fh 1: (0,200) -> (100,100): 100 wide, 100 tall, rising to the right.
		// fh 2: a wall at x=500 (never a floor).
		// fh 3: a flat segment at y=400, below the slope.
		"Map/Map9/900000010.img": `<imgdir name="900000010.img"><imgdir name="foothold"><imgdir name="1"><imgdir name="0">` +
			`<imgdir name="1"><int name="x1" value="0"/><int name="y1" value="200"/><int name="x2" value="100"/><int name="y2" value="100"/></imgdir>` +
			`<imgdir name="2"><int name="x1" value="500"/><int name="y1" value="0"/><int name="x2" value="500"/><int name="y2" value="300"/></imgdir>` +
			`<imgdir name="3"><int name="x1" value="0"/><int name="y1" value="400"/><int name="x2" value="100"/><int name="y2" value="400"/></imgdir>` +
			`</imgdir></imgdir></imgdir></imgdir>`,
	})
	d, err := LoadData(p, 900000010)
	require.NoError(t, err)
	tree := d.Footholds()
	require.Equal(t, 3, tree.Len())

	// x=50, y=0: s1=|100-200|=100, s2=|100-0|=100, s4=|50-0|=50, so s5 is the
	// same 49.99999999999999289 as the 45-degree case above, the (int) cast
	// truncates it to 49 and - because y2(100) < y1(200) - calcY = y1 - 49 =
	// 151 >= 0, so fh 1 wins and calcPointBelow drops to (50,151).
	f, ok := tree.FindBelow(50, 0)
	require.True(t, ok)
	assert.Equal(t, 1, f.ID)
	_, y, ok := tree.CalcPointBelow(50, 0)
	require.True(t, ok)
	assert.Equal(t, 151, y, "an up-slope interpolates upward from y1")

	// x=50, y=170: the slope surface (151) is above the query -> skipped, and
	// the flat fh 3 at y=400 becomes the floor.
	f, ok = tree.FindBelow(50, 170)
	require.True(t, ok)
	assert.Equal(t, 3, f.ID)

	// x=50, y=500: every candidate is above the query (151 and 400).
	_, ok = tree.FindBelow(50, 500)
	assert.False(t, ok)

	// x=500 hits the wall only: the wall is filtered out, so no floor even
	// though the segment is right there.
	_, ok = tree.FindBelow(500, 0)
	assert.False(t, ok, "a wall is never a floor")
}

func footholdIDs(fs []Foothold) []int {
	out := make([]int, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}
