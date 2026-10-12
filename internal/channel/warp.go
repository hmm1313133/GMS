// Package channel - P4.7 warp: the portal-based spawn point a character lands
// on when it enters the world, the portal transfer inside a channel and the
// three client handlers that ask for one (CHANGE_MAP, CHANGE_MAP_SPECIAL,
// USE_INNER_PORTAL).
//
// Java sources (see docs/FILETRACK.md):
//   - client/MapleCharacter.loadCharFromDB (channelserver portal block) -> resolveSpawn
//   - client/MapleCharacter.ReconstructChr (the CharacterTransfer variant)   -> resolveSpawn
//   - client/MapleCharacter.changeMapInternal    -> changeMapPortal
//   - server/maps/MapleGenericPortal.enterPortal -> enterPortal
//   - handling/channel/handler/PlayerHandler.ChangeMap / ChangeMapSpecial / InnerPortal
package channel

import (
	"GMS/internal/database"
	"GMS/internal/mapp"
	"GMS/internal/netw"
	"GMS/internal/packet"
	"GMS/internal/protocol"
)

const (
	// spawnFallbackMap is Java loadCharFromDB's fallback when the character's
	// map image does not exist (MapleCharacter:646 / :924, "char is on a map
	// that doesn't exist warp it to henesys").
	spawnFallbackMap = 100000000
	// zeroTargetID is Java ChangeMap's `if (targetid == 0) targetid = 1000000`
	// rewrite (PlayerHandler:1278-1280).
	zeroTargetID = 1000000
	// portalRangeSq is the 22500 = 150^2 the Java anti-cheat measures against a
	// portal's position (MapleGenericPortal.enterPortal:92). Java registers a
	// cheating offense and transfers anyway; the port logs it.
	portalRangeSq = 22500.0
)

// resolveSpawn ports the portal block of MapleCharacter.loadCharFromDB
// (channelserver=true) and its ReconstructChr twin:
//
//	map = mapFactory.getMap(mapid)
//	if (map == null) map = mapFactory.getMap(100000000)
//	[ReconstructChr only] if (map.getForcedReturnId() != 999999999)
//	                          map = map.getForcedReturnMap()
//	portal = map.getPortal(initialSpawnPoint)
//	if (portal == null) { portal = map.getPortal(0); initialSpawnPoint = 0 }
//	setPosition(portal.getPosition())
//
// It returns the map id and spawn point the client has to be told about plus
// the position to put the character on. forcedReturn selects the
// CharacterTransfer variant (Java keeps two copies of this block that differ
// exactly there: ReconstructChr:648-650 rewrites the map, loadCharFromDB:919-932
// does not).
//
// Deviations (documented):
//   - Java NPEs when neither the map nor the fallback map exists (or when the
//     map has no portal 0 either). The port logs and leaves the character at
//     (0,0) on whatever map id it resolved, so a server without wz still lets
//     players in.
//   - Java never guards the second lookup either, so a channel server WITHOUT
//     any wz (the DB-only degraded startup of cmd/gms) cannot load at all. The
//     port only rewrites the map when a Map.wz source is wired: with none, the
//     character stays on the map id its row carries (otherwise every character
//     of a degraded server would collapse onto 100000000).
//   - When the fallback map is used Java keeps the stale `mapid` field while
//     standing on 100000000, which desyncs the client. The port returns the
//     fallback id so both sides agree.
func (cs *ChannelServer) resolveSpawn(chr *database.Character, forcedReturn bool) (mapID, spawnPoint int, x, y int16) {
	mapID, spawnPoint = chr.Map, chr.Spawnpoint
	data := cs.srv.mapData(mapID)
	if data == nil && cs.srv.hasMapFactory() {
		cs.log.Warn("character map has no image, falling back",
			"playerID", chr.ID, "map", mapID, "fallback", spawnFallbackMap)
		mapID = spawnFallbackMap
		data = cs.srv.mapData(mapID)
	}
	if forcedReturn && data != nil && data.ForcedReturnID != mapp.NoTargetMap {
		mapID = data.ForcedReturnID
		data = cs.srv.mapData(mapID)
	}
	if data == nil {
		cs.log.Warn("spawn map has no image, keeping the raw spawn point",
			"playerID", chr.ID, "map", mapID, "spawn", spawnPoint)
		return mapID, spawnPoint, 0, 0
	}
	portal := data.Portal(spawnPoint)
	if portal == nil {
		// Java: the spawn point is stale - take the first portal instead.
		portal = data.Portal(0)
		spawnPoint = 0
	}
	if portal == nil {
		cs.log.Warn("spawn map has no portal 0, spawning at (0,0)",
			"playerID", chr.ID, "map", mapID)
		return mapID, spawnPoint, 0, 0
	}
	return mapID, spawnPoint, int16(portal.X), int16(portal.Y)
}

// mapForWarp returns the map instance a transfer is heading to. ok is false
// when the Map.wz image is unavailable, which is Java's `getMap(id) == null`
// (MapleMapFactory builds a MapleMap from the image and returns null without
// one). A channel without wz wired therefore cannot transfer anywhere, and the
// handlers answer enableActions exactly like Java does for a null target.
func (cs *ChannelServer) mapForWarp(id int) (*mapp.Map, bool) {
	if cs.srv.mapData(id) == nil {
		return nil, false
	}
	return cs.Map(id), true
}

// changeMapPortal ports MapleCharacter.changeMapPortal / changeMapInternal:
//
//	client.sendPacket(getWarpToMap(to, pto.getId(), this))
//	map.removePlayer(this)
//	if (storage.getCharacterById(getId()) != null) {
//	    map = to; setPosition(pos); to.addPlayer(this)
//	}
//
// Order matters: the client is sent to the new map first, then the old map
// drops the sprite (everyone left there gets REMOVE_PLAYER_FROM_MAP) and only
// then does the new map spawn it (the mover gets its own SPAWN_PLAYER with the
// new position, everyone already there gets it too).
//
// Skipped Java steps (documented): saveToDB(false, false) (the save loop is
// P5), eventInstance.changedMap and stats.relocHeal (P6/P9), and the party
// update block (P8). The unused `pto` parameter of changeMapInternal is why
// changeMap and changeMapPortal are one function here.
func (cs *ChannelServer) changeMapPortal(p *Player, to *mapp.Map, po *mapp.Portal) {
	if to == nil || po == nil {
		return
	}
	old := cs.lookupMap(p.MapID())
	p.SendPacket(packet.WarpToMapPacket(to.ID(), po.ID, cs.channel, p.HP()))
	if old != nil {
		old.RemovePlayer(p)
	}
	if cs.players.GetPlayerByID(p.ID) != p {
		// Java: getCharacterById(getId()) == null -> the character was already
		// evicted (duplicate-account login); it does not appear on the new map.
		cs.log.Warn("map transfer for a character that is no longer registered",
			"playerID", p.ID, "map", to.ID())
		return
	}
	p.SetMapID(to.ID())
	p.SetPosition(int16(po.X), int16(po.Y))
	to.AddPlayer(p)
	cs.log.Info("player changed map", "channel", cs.channel, "playerID", p.ID,
		"name", p.Name, "from", oldID(old), "to", to.ID(), "portal", po.ID)
}

// oldID reports the map id of a map instance (nil-safe) for logging.
func oldID(m *mapp.Map) int {
	if m == nil {
		return 0
	}
	return m.ID()
}

// handleChangeMap ports PlayerHandler.ChangeMap (recv 0x0021).
//
// Body: byte skip + int targetid + str portalName + short wheelRequest.
//
// Java's if-chain is reproduced as-is, including its dead ends:
//
//	available() == 0                          -> channel change (not ported)
//	targetid != -1 && !isAlive()               -> revive at the return map
//	targetid != -1 && isGM()                   -> warp to portal 0 of the target
//	targetid != -1 && !isGM()                  -> the hard-coded transfer table
//	portal != null                             -> portal.enterPortal
//	else                                       -> enableActions
//
// A non-GM targetid that matches no table entry is dropped SILENTLY (Java has
// no else there); only the table entries answer enableActions.
//
// Not ported (documented): the empty-body channel change
// (saveToDB + removePlayer + updateLoginState(1) + getChannelChange), which
// needs the accounts.loggedin write and the client reconnect flow; and the
// wheel revive (item 5510000) - with no inventory (P5.2) Java's haveItem is
// false, so `wheel` is false exactly like a character without the item.
func (h channelHandler) handleChangeMap(s *netw.Session, r *protocol.Reader) {
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	if r.Len() == 0 {
		h.cs.log.Debug("CHANGE_MAP without body (channel change) is not ported yet",
			"playerID", p.ID)
		return
	}
	r.Byte() // Java reads and discards one byte
	targetID := int(r.Int())
	if targetID == 0 {
		targetID = zeroTargetID
	}
	portalName := r.MapleAsciiString()
	r.Short() // wheel request flag (see the doc comment: always false here)
	if r.Err != nil {
		h.cs.log.Warn("malformed CHANGE_MAP", "err", r.Err, "playerID", p.ID)
		return
	}

	switch {
	case targetID != -1 && !p.IsAlive():
		h.reviveAtReturnMap(p)
	case targetID != -1 && p.IsGM():
		h.warpToMapPortal(p, targetID, 0)
	case targetID != -1:
		h.tableWarp(p, targetID)
	case h.enterPortalByName(p, portalName):
		// The portal took over (it answers enableActions itself when it does
		// not transfer).
	default:
		p.SendPacket(packet.EnableActionsPacket())
	}
}

// handleChangeMapSpecial ports PlayerHandler.ChangeMapSpecial (recv 0x0061).
//
// MapleServerHandler's case does its own `slea2.skip(1)` before calling the
// handler, which reads a short and then the portal name:
//
//	slea.skip(1)          // the dispatcher
//	slea.readShort()      // the handler
//	portal = map.getPortal(name)
//	if (portal != null) portal.enterPortal(c) else enableActions()
//
// The dispatcher skip is reproduced here so the body layout stays in one place.
func (h channelHandler) handleChangeMapSpecial(s *netw.Session, r *protocol.Reader) {
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	r.Skip(1) // MapleServerHandler case CHANGE_MAP_SPECIAL: slea2.skip(1)
	r.Short() // Java reads and discards a short
	name := r.MapleAsciiString()
	if r.Err != nil {
		h.cs.log.Warn("malformed CHANGE_MAP_SPECIAL", "err", r.Err, "playerID", p.ID)
		return
	}
	if !h.enterPortalByName(p, name) {
		p.SendPacket(packet.EnableActionsPacket())
	}
}

// handleInnerPortal ports PlayerHandler.InnerPortal (recv 0x0062): a teleport
// between two portals of the SAME map (the "inner portal" UI).
//
//	slea.skip(1)          // MapleServerHandler case USE_INNER_PORTAL
//	portal = map.getPortal(name)
//	toX = readShort(); toY = readShort()
//	if (portal == null) return
//	if (portal.getPosition().distanceSq(chr.getPosition()) > 22500)
//	    registerOffense(使用过远传送点)
//	map.movePlayer(chr, new Point(toX, toY))
//
// MapleMap.movePlayer only sets the position and refreshes object visibility
// (there are no monsters/items yet), so this is a position write. Java does not
// broadcast it - the moving client already drew itself. The dispatcher skip is
// reproduced here like in handleChangeMapSpecial.
func (h channelHandler) handleInnerPortal(s *netw.Session, r *protocol.Reader) {
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	r.Skip(1) // MapleServerHandler case USE_INNER_PORTAL: slea2.skip(1)
	name := r.MapleAsciiString()
	x, y := int16(r.Short()), int16(r.Short())
	if r.Err != nil {
		h.cs.log.Warn("malformed USE_INNER_PORTAL", "err", r.Err, "playerID", p.ID)
		return
	}
	m := h.cs.lookupMap(p.MapID())
	if m == nil || m.Data() == nil {
		return
	}
	po := m.Data().PortalByName(name)
	if po == nil || name == "" {
		return
	}
	px, py := p.Position()
	h.logFarPortalUse(p, po, px, py)
	p.SetPosition(x, y)
}

// enterPortalByName looks the portal up on the player's current map and enters
// it. It returns false when the map has no such portal, which is Java's
// `portal == null` -> enableActions path.
func (h channelHandler) enterPortalByName(p *Player, name string) bool {
	if name == "" {
		// Java's PortalFactory stores a missing "pn" as null, and a null name
		// never matches - Go stores "" there, so guard explicitly.
		return false
	}
	m := h.cs.lookupMap(p.MapID())
	if m == nil || m.Data() == nil {
		return false
	}
	po := m.Data().PortalByName(name)
	if po == nil {
		return false
	}
	h.enterPortal(p, po)
	return true
}

// enterPortal ports MapleGenericPortal.enterPortal:
//
//	if (portalState || isGM()) {
//	    if (scriptName != null) executePortalScript(this, c)
//	    else if (targetMapId != 999999999) {
//	        to = map(targetMapId)
//	        if (!isGM() && to == null) { enableActions(); return }
//	        if (!isGM() && to.getLevelLimit() > level) { dropMessage; enableActions(); return }
//	        changeMapPortal(to, to.getPortal(getTarget()) ?: to.getPortal(0))
//	    }
//	}
//	if (player.getMap() == currentmap) enableActions()
//
// The level limit is dead code in Java (MapleMapFactory never calls
// setLevelLimit, so getLevelLimit() is always 0) and portalState is true for
// every portal this database ships (only scripts flip it, P7), so both guards
// are documented and skipped.
//
// The script branch is the P7 script host: until it exists a scripted portal
// does nothing, which is why the trailing enableActions still fires.
func (h channelHandler) enterPortal(p *Player, po *mapp.Portal) {
	m := h.cs.lookupMap(p.MapID())
	x, y := p.Position()
	h.logFarPortalUse(p, po, x, y)

	switch {
	case po.Script != "":
		h.cs.log.Debug("portal script not ported yet",
			"playerID", p.ID, "portal", po.Name, "script", po.Script)
	case po.TargetMap != mapp.NoTargetMap:
		to, ok := h.cs.mapForWarp(po.TargetMap)
		if !ok {
			p.SendPacket(packet.EnableActionsPacket())
			return
		}
		d := to.Data()
		target := (*mapp.Portal)(nil)
		if po.Target != "" {
			target = d.PortalByName(po.Target)
		}
		if target == nil {
			// Java: an unknown or null "tn" silently falls back to the target
			// map's portal 0.
			target = d.Portal(0)
		}
		if target == nil {
			// Java NPEs here.
			h.cs.log.Warn("portal target map has no portal 0",
				"playerID", p.ID, "portal", po.Name, "target", po.TargetMap)
			p.SendPacket(packet.EnableActionsPacket())
			return
		}
		h.cs.changeMapPortal(p, to, target)
	}
	if cur := h.cs.lookupMap(p.MapID()); cur == m {
		// No transfer happened - Java's trailing check.
		p.SendPacket(packet.EnableActionsPacket())
	}
}

// warpToMapPortal ports the GM branch of ChangeMap: both the target map and its
// portal 0 must exist, otherwise nothing happens (Java sends no reply).
func (h channelHandler) warpToMapPortal(p *Player, mapID, portalID int) {
	to, ok := h.cs.mapForWarp(mapID)
	if !ok {
		return
	}
	po := to.Data().Portal(portalID)
	if po == nil {
		return
	}
	h.cs.changeMapPortal(p, to, po)
}

// tableWarp ports the hard-coded non-GM transfer table of ChangeMap
// (PlayerHandler:1315-1362). Every entry answers enableActions first and then
// warps to portal 0 of the target map; a request outside the table is dropped
// with NO reply, exactly like Java.
//
// The map ids are the quest/event instances of the original distribution; they
// are reproduced literally, including the odd `divi == 9130401` group (a map id
// whose value divided by 100 matches).
func (h channelHandler) tableWarp(p *Player, targetID int) {
	mapID := p.MapID()
	divi := mapID / 100
	sendActions, matched := true, false
	switch {
	case divi == 9130401 && (targetID == 130000000 || targetID/100 == 9130401):
		matched = true
	case divi == 9140900 && (targetID == 914090011 || targetID == 914090012 ||
		targetID == 914090013 || targetID == 140090000):
		matched = true
	case divi == 9140901 && targetID == 140000000:
		matched = true
	case divi == 9140902 && (targetID == 140030000 || targetID == 140000000):
		matched = true
	case divi == 9000900 && targetID/100 == 9000900 && targetID > mapID:
		matched = true
	case divi/1000 == 9000 && targetID/100000 == 9000:
		// Java's guard here looks inverted (it fires for targets OUTSIDE
		// 900090000..900090004) while the warp happens either way.
		sendActions = targetID < 900090000 || targetID > 900090004
		matched = true
	case mapID == 900090101 && targetID == 100030100:
		matched = true
	case mapID == 2010000 && targetID == 104000000:
		matched = true
	case (mapID == 106020001 || mapID == 106020502) && targetID == mapID-1:
		matched = true
	case mapID == 0 && targetID == 10000:
		matched = true
	}
	if !matched {
		return
	}
	if sendActions {
		p.SendPacket(packet.EnableActionsPacket())
	}
	to, ok := h.cs.mapForWarp(targetID)
	if !ok {
		return
	}
	po := to.Data().Portal(0)
	if po == nil {
		// Java NPEs here (it does not nil-check portal 0 in this branch).
		h.cs.log.Warn("transfer table target has no portal 0",
			"playerID", p.ID, "target", targetID)
		return
	}
	h.cs.changeMapPortal(p, to, po)
}

// reviveAtReturnMap ports the `targetid != -1 && !isAlive()` branch: a dead
// character is set to 50 HP and moved to the map's return map.
//
// Java's wheel branch (cash item 5510000, event instances and the pyramid
// subway) needs subsystems that do not exist yet (P5.2/P6/P9); with no
// inventory haveItem is false, so the port always takes the plain revive path.
// The null-return-map fallback also sends updateSingleStat(HP, 50) in Java,
// which arrives with the P5.1 stat system.
func (h channelHandler) reviveAtReturnMap(p *Player) {
	p.SetHP(50)
	m := h.cs.lookupMap(p.MapID())
	if m == nil || m.Data() == nil {
		p.SendPacket(packet.EnableActionsPacket())
		return
	}
	to, ok := h.cs.mapForWarp(m.Data().ReturnMapID)
	if !ok {
		p.SendPacket(packet.EnableActionsPacket())
		return
	}
	po := to.Data().Portal(0)
	if po == nil {
		p.SendPacket(packet.EnableActionsPacket())
		return
	}
	h.cs.changeMapPortal(p, to, po)
}

// logFarPortalUse ports the anti-cheat the Java portal code runs before every
// transfer: using a portal from further than 150 units away (22500 squared)
// registers an offense - Java keeps going, and so does the port (INFO log
// instead of an offense counter, which is P9).
func (h channelHandler) logFarPortalUse(p *Player, po *mapp.Portal, x, y int16) {
	dx, dy := float64(po.X)-float64(x), float64(po.Y)-float64(y)
	if dx*dx+dy*dy <= portalRangeSq {
		return
	}
	h.cs.log.Info("portal used from far away", "playerID", p.ID, "portal", po.Name,
		"portalX", po.X, "portalY", po.Y, "playerX", x, "playerY", y)
}
