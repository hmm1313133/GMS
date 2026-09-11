package mapp

// P4.3b: the Map.wz map-instance data (Java server.maps.MapleMapFactory's data
// half). This file declares the loaded shapes: portals, footholds and the
// life spawns of one map image, plus the MapData aggregate every accessor
// hangs off. The loader itself lives in factory.go, the portal id assignment
// in portal.go and the foothold search in foothold.go.
//
// Java sources (see docs/FILETRACK.md):
//   - server/maps/MapleMap       -> MapData's fields (setters of getMap)
//   - server/MaplePortal         -> Portal (MAP_PORTAL / DOOR_PORTAL)
//   - server/maps/MapleFoothold  -> Foothold (isWall)
//   - server/maps/MapleFootholdTree -> FootholdTree
//   - server/life/SpawnPoint     -> the monster half of LifeSpawn
//   - server/maps/FieldLimitType -> FieldLimit* (the info/fieldLimit bits)

// Portal type constants (Java server.MaplePortal).
const (
	// PortalMap is MaplePortal.MAP_PORTAL: a portal linking two maps.
	PortalMap = 2
	// PortalDoor is MaplePortal.DOOR_PORTAL: a Mystic Door / "tp" portal.
	PortalDoor = 6
	// firstDoorPortal is PortalFactory.nextDoorPortal's seed. Door portals
	// ignore their wz node name and are renumbered from 128 upwards.
	firstDoorPortal = 128
	// NoTargetMap is Java's "no map" sentinel: it is both the
	// findClosestSpawnpoint filter (only portals with this target are spawn
	// points) and MapleMap.forcedReturnMap's default.
	NoTargetMap = 999999999
)

// Portal is one entry of a map's "portal" section (Java server.MaplePortal as
// built by server.PortalFactory).
type Portal struct {
	// ID is the portal id: the wz node name for every type but DOOR_PORTAL,
	// which gets firstDoorPortal+n in document order instead.
	ID int
	// Name is the "pn" attribute (Java getPortalName); "sp" marks a spawn
	// portal, "tp" a door/town portal.
	Name string
	// Type is the raw "pt" attribute (Java PortalFactory passes it straight
	// through, so out-of-range values survive).
	Type int
	// X, Y is the portal position.
	X, Y int
	// TargetMap is the "tm" attribute (Java getTargetMapId); NoTargetMap
	// means "no target map".
	TargetMap int
	// Target is the "tn" attribute: the NAME of the destination portal.
	Target string
	// Script is the optional "script" attribute. Java normalises the empty
	// string to null, so Go uses "" for "no script" as well.
	Script string
}

// Foothold is one line segment of a map's walkable ground (Java
// server.maps.MapleFoothold). Coordinates use the client's screen space:
// x grows right, y grows DOWN.
type Foothold struct {
	// ID is the wz node name of the foothold ("fhId"), which is the id the
	// life entries reference through "fh".
	ID int
	// X1, Y1 is the first end point, X2, Y2 the second.
	X1, Y1, X2, Y2 int
	// Prev, Next are the ids of the neighbouring footholds on the same
	// ledge (Java getPrev/getNext; 0 = none).
	Prev, Next int
}

// IsWall ports MapleFoothold.isWall: a vertical segment cannot be walked on
// and never wins findBelow.
func (f Foothold) IsWall() bool { return f.X1 == f.X2 }

// FootholdTree is the map's foothold set. Java builds a quadtree
// (MapleFootholdTree), but that tree never subdivides: getMap seeds lBound
// with (0,0) and only ever widens it to the min/max envelope of every
// foothold, so the containment test `x1 >= p1.x && x2 <= p2.x && y1 >= p1.y &&
// y2 <= p2.y` holds for every foothold and all of them land in the root node.
// A flat slice in document order is therefore the faithful structure - the
// quadtree's only observable effect (getRelevants) is "return everything".
type FootholdTree struct {
	all []Foothold
}

// All returns the footholds in document order. The slice is shared with the
// map data and must not be modified.
func (t *FootholdTree) All() []Foothold {
	if t == nil {
		return nil
	}
	return t.all
}

// Len returns the number of footholds.
func (t *FootholdTree) Len() int {
	if t == nil {
		return 0
	}
	return len(t.all)
}

// LifeSpawn is one entry of a map's "life" section: the map-side position of a
// monster or NPC (Java server.life.SpawnPoint for monsters plus the
// AbstractLoadedMapleLife fields the factory copies onto it).
type LifeSpawn struct {
	// Kind is 'm' for a monster, 'n' for an NPC (Java
	// MapleLifeFactory.getLife's case-insensitive "m"/"n").
	Kind byte

	// ID is the mob/NPC id (the wz "id" string parsed as an int).
	ID int
	// X, Y is the raw position from the image.
	X, Y int
	// F is the facing direction ("f"); HasF records whether the node
	// existed, because 55 of 2445 sampled entries omit it and Java only
	// overwrites the default when it is present.
	F    int
	HasF bool
	// Fh is the foothold id the life stands on, Cy the "cy" value.
	Fh, Cy int
	// Rx0, Rx1 delimit the roaming range.
	Rx0, Rx1 int

	// MobTime is the respawn time in SECONDS, straight from the image
	// (-1 = "spawn once, never respawn", 0 = instant; the milliseconds
	// conversion of Java SpawnPoint is P6's job). Java reads it with a
	// default of 0 for the entries that carry no mobTime node.
	MobTime int
	// Hide is Java's `hide == 1` flag, which it applies to NPCs only.
	Hide bool
	// Team is the carnival team byte (Java's default -1; the wz data has
	// almost no "team" nodes).
	Team int8

	// SpawnX, SpawnY is the position the spawn point actually uses. Java's
	// MapleMap.addMonsterSpawn runs calcPointBelow(position) and then
	// subtracts 1 from y, so a monster sits one pixel above the foothold
	// surface; NPCs keep the raw position (Java addMapObject).
	SpawnX, SpawnY int
}

// MapData is one loaded Map.wz image (Java: the MapleMap fields
// MapleMapFactory.getMap fills in from the image). It is immutable once Load
// returned it, so it may be shared between goroutines.
type MapData struct {
	// ID is the REQUESTED map id; ImageID is the image the data really came
	// from. They differ for the 1152 link stubs of the 079MAX2 export,
	// where info/link points at the image Java builds the map from.
	ID, ImageID int
	// ReturnMapID is info/returnMap (0 when the node is missing - Java
	// throws there) after the MapleMap constructor quirk: a returnMap of
	// 910000000 is replaced by the map's own id.
	ReturnMapID int
	// ForcedReturnID is info/forcedReturn (Java default NoTargetMap).
	ForcedReturnID int
	// FieldLimit is the info/fieldLimit bit set (see FieldLimit* below).
	FieldLimit int
	// FieldType is info/fieldType.
	FieldType int
	// TimeLimit is info/timeLimit in minutes (Java default -1 = none).
	TimeLimit int
	// Town, Everlast, PersonalShop, Soaring are the Java >0 flags of
	// info/town, info/everlast, info/personalShop and info/fly. Java reads
	// info/needSkillForFly for soaring, which exists nowhere in this
	// export; "fly" is the node the data actually carries (same meaning:
	// the map allows flying mounts).
	Town, Everlast, PersonalShop, Soaring bool
	// Clock is true when the image has a TOP-LEVEL "clock" node (Java
	// getMap: mapData.getChildByPath("clock") != null). Note that
	// MapleMapFactory.CreateInstanceMap reads info/clock instead - a Java
	// inconsistency, since no image carries info/clock.
	Clock bool
	// MobRate is the info/mobRate spawn multiplier (Java default 0 when the
	// node is missing).
	MobRate float64
	// MobCapacity is info/fixedMobCapacity (Java setFixedMob).
	MobCapacity int
	// CreateMobInterval is info/createMobInterval in ms (Java default 3000).
	CreateMobInterval int
	// OnEnter is info/onUserEnter, OnFirstEnter info/onFirstUserEnter.
	OnEnter, OnFirstEnter string

	portals   []Portal
	footholds FootholdTree
	life      []LifeSpawn
}

// Portals returns the map's portals in document order. The slice is shared
// with the map data and must not be modified.
func (d *MapData) Portals() []Portal {
	if d == nil {
		return nil
	}
	return d.portals
}

// Footholds returns the map's foothold tree (never nil for a loaded map).
func (d *MapData) Footholds() *FootholdTree {
	if d == nil {
		return nil
	}
	return &d.footholds
}

// Life returns every life entry in document order (monsters and NPCs). The
// slice is shared with the map data and must not be modified.
func (d *MapData) Life() []LifeSpawn {
	if d == nil {
		return nil
	}
	return d.life
}

// Mobs returns the monster entries of Life (Java MapleMap.monsterSpawn).
func (d *MapData) Mobs() []LifeSpawn { return d.filter('m') }

// NPCs returns the NPC entries of Life (Java MapleMap.addMapObject of the
// MapleNPC half).
func (d *MapData) NPCs() []LifeSpawn { return d.filter('n') }

func (d *MapData) filter(kind byte) []LifeSpawn {
	if d == nil {
		return nil
	}
	var out []LifeSpawn
	for _, l := range d.life {
		if l.Kind == kind {
			out = append(out, l)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// server.maps.FieldLimitType
// ---------------------------------------------------------------------------

// FieldLimit is one bit of MapleMap.getFieldLimit (Java server.maps.
// FieldLimitType). The bits that matter in game are Jump (no jumping),
// MovementSkills (no teleport/dash), MysticDoor (no Mystic Door) and Mount.
type FieldLimit int

// The FieldLimitType enum values (Java :7-21).
const (
	FieldLimitJump           FieldLimit = 1
	FieldLimitMovementSkills FieldLimit = 2
	FieldLimitSummoningBag   FieldLimit = 4
	FieldLimitMysticDoor     FieldLimit = 8
	FieldLimitChannelSwitch  FieldLimit = 16
	FieldLimitRegularExpLoss FieldLimit = 32
	FieldLimitVipRock        FieldLimit = 64
	FieldLimitMinigames      FieldLimit = 128
	FieldLimitNoClue1        FieldLimit = 256
	FieldLimitMount          FieldLimit = 512
	FieldLimitPotionUse      FieldLimit = 1024
	FieldLimitEvent          FieldLimit = 8192
	FieldLimitPet            FieldLimit = 32768
	FieldLimitEvent2         FieldLimit = 65536
	FieldLimitDropDown       FieldLimit = 131072
)

// Check ports FieldLimitType.check: the bit is set in the map's fieldLimit.
func (f FieldLimit) Check(fieldLimit int) bool {
	return fieldLimit&int(f) == int(f)
}
