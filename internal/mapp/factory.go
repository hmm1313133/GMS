package mapp

// P4.3b: loading Map.wz images into MapData (Java MapleMapFactory.getMap's
// data half) and the per-channel factory + cache.
//
// Java sources (see docs/FILETRACK.md):
//   - server/maps/MapleMapFactory -> MapImagePath, LoadData, Factory
//   - server/maps/MapleMap        -> the info flags MapData carries
//   - server/life/MapleLifeFactory-> the "m"/"n" life kind

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"GMS/internal/wzs"
)

// MapImagePath is Java MapleMapFactory.getMapName: the wz-relative path of a
// map image,
//
//	StringUtil.getLeftPaddedStr(mapid, '0', 9) prefixed with "Map/Map<id/1e8>/"
//	and suffixed with ".img"
//
// so 0 -> "Map/Map0/000000000.img" and 100000000 -> "Map/Map1/100000000.img".
func MapImagePath(id int) string {
	return fmt.Sprintf("Map/Map%d/%09d.img", id/100000000, id)
}

// LoadData loads one map image into MapData. Warnings (a missing
// info/returnMap, an unparseable portal/foothold/life entry, a monster without
// a foothold below it) go to slog.Default(); use a Factory to route them to a
// server logger.
//
// It never panics: absent sections yield zero portals/footholds/life, and a
// missing image is reported as an error (Java returns null).
func LoadData(p *wzs.Provider, id int) (*MapData, error) {
	return loadData(p, id, slog.Default())
}

// loadData is LoadData with an explicit logger.
func loadData(p *wzs.Provider, id int, log *slog.Logger) (*MapData, error) {
	if p == nil {
		return nil, fmt.Errorf("mapp: map %d: no Map.wz provider", id)
	}
	path := MapImagePath(id)
	img, err := p.Data(path)
	if err != nil {
		return nil, fmt.Errorf("mapp: map %d: %w", id, err)
	}

	// info/link: 1,152 of the export's images are stubs whose only content is
	// this node. Java re-reads the linked image and builds the map from THAT
	// (MapleMapFactory.java:89-92), one hop only - it never follows a chain.
	imgID := id
	if link := img.ChildByPath("info/link"); link != nil {
		target := wzs.GetIntConvertPathDef("info/link", img, 0)
		switch {
		case target == 0 || target == id:
			// Guard against a silent empty map and a self-recursion.
			log.Warn("map link ignored", "map", id, "image", path, "link", target)
		default:
			linked, lerr := p.Data(MapImagePath(target))
			if lerr != nil {
				return nil, fmt.Errorf("mapp: map %d: link %d: %w", id, target, lerr)
			}
			if linked.ChildByPath("info/link") != nil {
				// One hop only, like Java: warn instead of following the
				// chain (real data has no chained links).
				log.Warn("map link target is itself a link, not following it",
					"map", id, "link", target)
			}
			img, imgID = linked, target
		}
	}

	d := &MapData{ID: id, ImageID: imgID}
	d.ReturnMapID = loadInfo(d, img, log)
	d.portals = loadPortals(img, log)
	d.footholds = loadFootholds(img, log)
	d.life = loadLife(img, &d.footholds, id, log)
	return d, nil
}

// loadInfo reads the info/* nodes of the (possibly linked) image and returns
// info/returnMap, which the MapleMap constructor needs at construction time.
func loadInfo(d *MapData, img *wzs.Node, log *slog.Logger) int {
	if img.ChildByPath("info/returnMap") == nil {
		// Java uses the no-default getInt overload here and throws an NPE;
		// 314/314 sampled images carry the node, so a missing one is real
		// breakage and must be visible. Note that Java would NOT fall back to
		// the map's own id.
		log.Warn("map has no info/returnMap, using 0", "map", d.ID, "image", d.ImageID)
	}
	returnMapID := wzs.GetIntPathDef("info/returnMap", img, 0)
	// MapleMap constructor quirk (MapleMap.java:160-163): the free market's
	// return map is rewritten to the map's own id.
	if returnMapID == 910000000 {
		returnMapID = d.ID
	}
	d.ForcedReturnID = wzs.GetIntPathDef("info/forcedReturn", img, NoTargetMap)
	d.FieldLimit = wzs.GetIntPathDef("info/fieldLimit", img, 0)
	d.FieldType = wzs.GetIntPathDef("info/fieldType", img, 0)
	d.TimeLimit = wzs.GetIntPathDef("info/timeLimit", img, -1)
	d.Town = wzs.GetIntPathDef("info/town", img, 0) > 0
	d.Everlast = wzs.GetIntPathDef("info/everlast", img, 0) > 0
	d.PersonalShop = wzs.GetIntPathDef("info/personalShop", img, 0) > 0
	// Java reads info/needSkillForFly, which exists nowhere in this export;
	// the data carries "fly" instead (same meaning).
	d.Soaring = wzs.GetIntPathDef("info/fly", img, 0) > 0
	d.MobRate = float64(wzs.GetFloatDef(img.ChildByPath("info/mobRate"), 0))
	d.MobCapacity = wzs.GetIntPathDef("info/fixedMobCapacity", img, 0)
	d.CreateMobInterval = wzs.GetIntPathDef("info/createMobInterval", img, 3000)
	d.OnEnter = wzs.GetStringPathDef("info/onUserEnter", img, "")
	d.OnFirstEnter = wzs.GetStringPathDef("info/onFirstUserEnter", img, "")
	// getMap reads the TOP-LEVEL clock node (CreateInstanceMap reads
	// info/clock, which no image has - a Java inconsistency).
	d.Clock = img.Child("clock") != nil
	return returnMapID
}

// loadLife ports the life loop of MapleMapFactory.getMap plus its loadLife
// helper.
//
// Java bug not ported (MapleMapFactory.java:146-147 and :329-330): it calls
// addMonsterSpawn TWICE per monster, which doubles monsterSpawn, skews
// maxRegularSpawn and halves the respawn timing. Every entry is loaded once
// here.
func loadLife(img *wzs.Node, tree *FootholdTree, mapID int, log *slog.Logger) []LifeSpawn {
	nodes := img.ChildByPath("life").Children()
	if len(nodes) == 0 {
		return nil
	}
	out := make([]LifeSpawn, 0, len(nodes))
	for _, n := range nodes {
		// Java MapleLifeFactory.getLife compares the type with
		// equalsIgnoreCase and prints "Unknown Life type" for anything else.
		var kind byte
		switch strings.ToLower(wzs.GetStringPathDef("type", n, "")) {
		case "m":
			kind = 'm'
		case "n":
			kind = 'n'
		default:
			log.Warn("map life entry has an unknown type, skipping",
				"map", mapID, "type", wzs.GetStringPathDef("type", n, ""), "node", n.Name)
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(wzs.GetStringPathDef("id", n, "")))
		if err != nil {
			log.Warn("map life entry has a non-numeric id, skipping",
				"map", mapID, "id", wzs.GetStringPathDef("id", n, ""), "node", n.Name)
			continue
		}
		// Java loadLife drops two guide NPCs from the tutorial map.
		if mapID == 910000000 && (id == 9310059 || id == 9310022) {
			continue
		}

		l := LifeSpawn{
			Kind: kind,
			ID:   id,
			X:    wzs.GetIntPathDef("x", n, 0),
			Y:    wzs.GetIntPathDef("y", n, 0),
			Fh:   wzs.GetIntPathDef("fh", n, 0),
			Cy:   wzs.GetIntPathDef("cy", n, 0),
			Rx0:  wzs.GetIntPathDef("rx0", n, 0),
			Rx1:  wzs.GetIntPathDef("rx1", n, 0),
			// Java getInt("mobTime", life, 0): seconds, with 0 for the
			// entries that carry no mobTime node (a data -1 means "spawn
			// once, never respawn" - Java SpawnPoint.shouldSpawn). The
			// struct documents -1 as "none"; that is a DATA value here, the
			// missing-node default stays Java's 0 so the spawn timing of the
			// 1857 real monster entries without the node does not change.
			MobTime: wzs.GetIntPathDef("mobTime", n, 0),
			// Java applies "hide" to NPCs only and nil-guards the "f" node.
			Hide: kind == 'n' && wzs.GetIntPathDef("hide", n, 0) == 1,
			Team: int8(wzs.GetIntPathDef("team", n, -1)),
		}
		if f := n.Child("f"); f != nil {
			l.HasF = true
			l.F = wzs.GetInt(f)
		}
		l.SpawnX, l.SpawnY = l.X, l.Y
		if kind == 'm' {
			// MapleMap.addMonsterSpawn: calcPointBelow(position) then --y.
			if px, py, ok := tree.CalcPointBelow(l.X, l.Y); ok {
				l.SpawnX, l.SpawnY = px, py-1
			} else {
				// Java NPEs here. Dropping the spawn would empty every map
				// without footholds, so keep the raw position and warn.
				log.Warn("map monster spawn has no foothold below it, keeping the raw position",
					"map", mapID, "mob", id, "x", l.X, "y", l.Y)
			}
		}
		out = append(out, l)
	}
	return out
}

// Factory loads and caches map data (Java MapleMapFactory, whose `maps` field
// is a per-channel HashMap<Integer,MapleMap> built under a fair ReentrantLock
// with a double-checked lookup).
//
// Deliberate deviations from Java:
//   - Go caches MapData (the image contents), not *Map instances: Map
//     instances stay per-channel in ChannelServer.
//   - Java has no negative cache, so a missing image is re-read on every
//     getMap call. Go remembers the failure per id and warns exactly once.
//
// A Factory is safe for concurrent use.
type Factory struct {
	wz  *wzs.Provider
	log *slog.Logger

	mu     sync.Mutex
	maps   map[int]*MapData
	failed map[int]error
}

// NewFactory creates a factory over the Map.wz provider. A nil provider is
// allowed: every Load then fails cleanly (Java's static `source` would NPE).
func NewFactory(p *wzs.Provider, log *slog.Logger) *Factory {
	if log == nil {
		log = slog.Default()
	}
	return &Factory{
		wz:     p,
		log:    log,
		maps:   map[int]*MapData{},
		failed: map[int]error{},
	}
}

// Load returns the map data of one map id, loading it on first use. Calls
// after the first failure return the same error without warning again.
func (f *Factory) Load(id int) (*MapData, error) {
	if f == nil {
		return nil, fmt.Errorf("mapp: map %d: nil factory", id)
	}
	// The lock is held across the image read, exactly like Java's fair lock
	// around the MapleMap construction: it serialises the first load of each
	// map and makes the "warn once" of the negative cache exact.
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.maps[id]; ok {
		return d, nil
	}
	if err, ok := f.failed[id]; ok {
		return nil, err
	}
	d, err := loadData(f.wz, id, f.logger())
	if err != nil {
		if f.failed == nil {
			f.failed = map[int]error{}
		}
		f.failed[id] = err
		f.logger().Warn("map image unavailable, map stays bare", "map", id, "image", MapImagePath(id), "err", err)
		return nil, err
	}
	if f.maps == nil {
		f.maps = map[int]*MapData{}
	}
	f.maps[id] = d
	return d, nil
}

// Data returns the cached data of a map id, nil when it was never loaded (or
// failed to load). It never loads and never panics - a nil *Factory returns
// nil, so callers can use it on an unwired server.
func (f *Factory) Data(id int) *MapData {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maps[id]
}

func (f *Factory) logger() *slog.Logger {
	if f == nil || f.log == nil {
		return slog.Default()
	}
	return f.log
}
