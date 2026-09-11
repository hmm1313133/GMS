package mapp

// P4.3b: portal loading and lookup (Java MapleMapFactory's portal loop +
// server.PortalFactory.loadPortal + MapleMap.findClosestSpawnpoint).

import (
	"log/slog"
	"math"
	"strconv"

	"GMS/internal/wzs"
)

// loadPortals ports the portal half of MapleMapFactory.getMap:
//
//	PortalFactory portalFactory = new PortalFactory();
//	for (MapleData portal : mapData.getChildByPath("portal")) {
//	    map.addPortal(portalFactory.makePortal(getInt(pt), portal));
//	}
//
// plus PortalFactory.loadPortal's attribute reads and its id rule:
//
//	if (myPortal.getType() == 6) { myPortal.setId(nextDoorPortal++); }
//	else { myPortal.setId(Integer.parseInt(portal.getName())); }
//
// The door counter starts at 128 and runs in DOCUMENT ORDER, so on map
// 100000000 the six pt=6 portals (wz node names 28..33) really do become ids
// 128..133 and there is no portal 28..33.
//
// Java reads pn/tn/tm/x/y/pt without a default (a missing node is an NPE) and
// Integer.parseInt throws NumberFormatException on a non-numeric node name,
// which loses the whole map; Go keeps the MapleDataTool default-0 semantics
// and skips just the offending element.
func loadPortals(root *wzs.Node, log *slog.Logger) []Portal {
	nodes := root.ChildByPath("portal").Children()
	if len(nodes) == 0 {
		return nil
	}
	out := make([]Portal, 0, len(nodes))
	nextDoor := firstDoorPortal
	for _, n := range nodes {
		p := Portal{
			Name:      wzs.GetStringPathDef("pn", n, ""),
			Type:      wzs.GetIntPathDef("pt", n, 0),
			X:         wzs.GetIntPathDef("x", n, 0),
			Y:         wzs.GetIntPathDef("y", n, 0),
			TargetMap: wzs.GetIntPathDef("tm", n, 0),
			Target:    wzs.GetStringPathDef("tn", n, ""),
			Script:    wzs.GetStringPathDef("script", n, ""),
		}
		if p.Type == PortalDoor {
			p.ID = nextDoor
			nextDoor++
		} else {
			id, err := strconv.Atoi(n.Name)
			if err != nil {
				// Java throws NumberFormatException and drops the map.
				log.Warn("map portal has a non-numeric node name, skipping",
					"node", n.Name, "name", p.Name, "type", p.Type)
				continue
			}
			p.ID = id
		}
		out = append(out, p)
	}
	return out
}

// Portal returns the portal with that id, nil when the map has none (Java
// MapleMap.getPortal).
func (d *MapData) Portal(id int) *Portal {
	if d == nil {
		return nil
	}
	for i := range d.portals {
		if d.portals[i].ID == id {
			return &d.portals[i]
		}
	}
	return nil
}

// PortalByName returns the first portal with that name, nil when absent (Java
// MapleMap.getPortal(String), which the portal script/target lookups use).
//
// Java iterates a HashMap, so with duplicate names its winner depends on hash
// order; Go uses document order, which is deterministic and agrees with the
// real data (a map's duplicate names are its "sp"/"tp" portals, which are
// interchangeable for every caller).
func (d *MapData) PortalByName(name string) *Portal {
	if d == nil {
		return nil
	}
	for i := range d.portals {
		if d.portals[i].Name == name {
			return &d.portals[i]
		}
	}
	return nil
}

// FindClosestSpawnPoint ports MapleMap.findClosestSpawnpoint: the nearest
// portal of type 0..2 whose target map is NoTargetMap (i.e. a portal that does
// not leave the map - the "sp" spawn points). The comparison is java.awt.Point
// distanceSq, i.e. the squared distance, and it keeps the FIRST portal of a
// tie.
func (d *MapData) FindClosestSpawnPoint(x, y int) *Portal {
	if d == nil {
		return nil
	}
	var closest *Portal
	shortest := math.Inf(1) // Java: Double.POSITIVE_INFINITY
	for i := range d.portals {
		p := &d.portals[i]
		if p.Type < 0 || p.Type > PortalMap || p.TargetMap != NoTargetMap {
			continue
		}
		dx, dy := float64(p.X-x), float64(p.Y-y)
		distance := dx*dx + dy*dy
		if distance >= shortest {
			continue
		}
		closest, shortest = p, distance
	}
	return closest
}
