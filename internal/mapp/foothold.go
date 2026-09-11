package mapp

// P4.3b: foothold loading and the findBelow / calcPointBelow search (Java
// MapleFootholdTree.findBelow + MapleMap.calcPointBelow).
//
// The Java tree is built from a min/max envelope seeded at (0,0)
// (MapleMapFactory.getMap):
//
//	Point lBound = new Point(); Point uBound = new Point();
//	... widen with every foothold's x1/x2/y1/y2 ...
//	MapleFootholdTree fTree = new MapleFootholdTree(lBound, uBound);
//
// so every foothold satisfies the root's containment test `x1 >= p1.x &&
// x2 <= p2.x && y1 >= p1.y && y2 <= p2.y` and the tree never subdivides:
// getRelevants(p) always returns the whole list, in insertion order. The Go
// tree is that flat list (document order) - see FootholdTree.

import (
	"log/slog"
	"math"
	"sort"
	"strconv"

	"GMS/internal/wzs"
)

// loadFootholds ports the foothold loop of MapleMapFactory.getMap:
//
//	for (MapleData footRoot : mapData.getChildByPath("foothold"))
//	    for (MapleData footCat : footRoot)
//	        for (MapleData footHold : footCat)
//	            new MapleFoothold(... , Integer.parseInt(footHold.getName()))
//
// i.e. foothold/<group>/<layer>/<fhId>, flattened in document order. The
// group/layer names are a spatial hint only - the factory never uses them, so
// neither does this walk. The "forbidFallDown" and "force" attributes exist in
// the data but Java ignores them, so they are ignored here too.
func loadFootholds(root *wzs.Node, log *slog.Logger) FootholdTree {
	var t FootholdTree
	for _, group := range root.ChildByPath("foothold").Children() {
		for _, layer := range group.Children() {
			for _, n := range layer.Children() {
				id, err := strconv.Atoi(n.Name)
				if err != nil {
					// Java throws NumberFormatException and loses the map.
					log.Warn("map foothold has a non-numeric node name, skipping", "node", n.Name)
					continue
				}
				t.all = append(t.all, Foothold{
					ID:   id,
					X1:   wzs.GetIntPathDef("x1", n, 0),
					Y1:   wzs.GetIntPathDef("y1", n, 0),
					X2:   wzs.GetIntPathDef("x2", n, 0),
					Y2:   wzs.GetIntPathDef("y2", n, 0),
					Prev: wzs.GetIntPathDef("prev", n, 0),
					Next: wzs.GetIntPathDef("next", n, 0),
				})
			}
		}
	}
	return t
}

// FindBelow ports MapleFootholdTree.findBelow: the foothold a point falls onto.
//
//	getRelevants(p)                       -> every foothold (see above)
//	keep x1 <= x <= x2 && x1 != x2        -> the flat list is filtered
//	Collections.sort(xMatches)            -> by MapleFoothold.compareTo
//	first survivor wins
//
// The comparator is the one-sided
//
//	if (this.y2 < other.y1) return -1;
//	if (this.y1 > other.y2) return 1;
//	return 0;
//
// which only orders footholds that are clearly above/below each other.
// Collections.sort is a stable TimSort, so sort.SliceStable reproduces the tie
// order (document order). Java's sort and Go's stable sort can disagree when
// the comparator is inconsistent (0 is not transitive here); the real data
// sorts the same way in every case exercised by the tests.
//
// The survivor tests are the Java ones verbatim:
//
//	slope (not a wall, y1 != y2): interpolate the y at x and skip when the
//	    surface is ABOVE the query point (calcY < p.y, remember y grows down)
//	wall or flat:                 skip when y1 < p.y, else it is the floor
//
// The x filter is not normalised either: Java asks `x1 <= x <= x2`, so a
// segment stored right-to-left can never match. The export really contains
// such segments (410 of its 359,848 footholds have x1 > x2) and Java silently
// ignores them; Go keeps that.
func (t *FootholdTree) FindBelow(x, y int) (Foothold, bool) {
	if t == nil {
		return Foothold{}, false
	}
	matches := make([]Foothold, 0, len(t.all))
	for _, f := range t.all {
		if f.X1 > x || f.X2 < x || f.X1 == f.X2 {
			continue
		}
		matches = append(matches, f)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return compareFoothold(matches[i], matches[j]) < 0
	})
	for _, f := range matches {
		if !f.IsWall() && f.Y1 != f.Y2 {
			if interpolatedY(f, x) < y {
				continue
			}
			return f, true
		}
		if f.IsWall() || f.Y1 < y {
			continue
		}
		return f, true
	}
	return Foothold{}, false
}

// CalcPointBelow ports MapleMap.calcPointBelow: the point (x, dropY) where
// dropY is the foothold surface under (x, y), interpolated over a slope. It
// reports false when no foothold is below the point (Java returns null, and
// MapleMap.addMonsterSpawn then throws an NPE).
func (t *FootholdTree) CalcPointBelow(x, y int) (int, int, bool) {
	f, ok := t.FindBelow(x, y)
	if !ok {
		return 0, 0, false
	}
	if !f.IsWall() && f.Y1 != f.Y2 {
		return x, interpolatedY(f, x), true
	}
	return x, f.Y1, true
}

// compareFoothold ports MapleFoothold.compareTo (-1 = this foothold lies above
// other, 1 = below, 0 = the y ranges overlap).
func compareFoothold(f, other Foothold) int {
	switch {
	case f.Y2 < other.Y1:
		return -1
	case f.Y1 > other.Y2:
		return 1
	}
	return 0
}

// interpolatedY is the y of the slope foothold f at x, as computed by both
// MapleFootholdTree.findBelow and MapleMap.calcPointBelow:
//
//	s1 = |y2 - y1|; s2 = |x2 - x1|; s4 = |x - x1|
//	s5 = cos(atan(s2/s1)) * (s4 / cos(atan(s1/s2)))
//	calcY = y2 < y1 ? y1 - (int)s5 : y1 + (int)s5
//
// (the Java locals really are named that way round - s1 is the y span and s2
// the x span, see MapleFootholdTree.java:153-155).
// cos(atan(s2/s1)) = s1/hypot and cos(atan(s1/s2)) = s2/hypot, so s5 is the
// linear interpolation |dy/dx| * |x - x1|; the double arithmetic and the
// truncating (int) cast are kept exactly as Java writes them.
//
// The truncation is observable: for a 45-degree segment (s1 == s2) the double
// result is one ulp BELOW the ideal value (s4 = 50 gives
// 49.99999999999999289), so (int)s5 is 49 and the surface sits one pixel
// above the geometric line. Java behaves the same way, so the port does too.
func interpolatedY(f Foothold, x int) int {
	s1 := math.Abs(float64(f.Y2 - f.Y1))
	s2 := math.Abs(float64(f.X2 - f.X1))
	s4 := math.Abs(float64(x - f.X1))
	alpha := math.Atan(s2 / s1)
	beta := math.Atan(s1 / s2)
	s5 := math.Cos(alpha) * (s4 / math.Cos(beta))
	if f.Y2 < f.Y1 {
		return f.Y1 - int(s5)
	}
	return f.Y1 + int(s5)
}
