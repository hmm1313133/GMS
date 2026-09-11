// Package mapp is the map engine (Java server.maps). P4.2 brought the minimal
// instance the channel server needs to attach a player: an id and a player
// set. P4.3 adds the spawn/despawn broadcast (Java MapleMap.addPlayer /
// removePlayer -> spawnPlayerMapobject / removePlayerFromMap). Footholds,
// life/monsters, portals and the view-range filtering grow with P4.4+/P6.
//
// Java sources (see docs/FILETRACK.md):
//   - server/maps/MapleMap        -> Map
//   - server/maps/MapleMapFactory -> Factory (arrives with P4.3 map data)
package mapp

import "sync"

// PacketSink is a packet sink for one connected client (Java
// MapleClient.sendPacket). netw.Session-based players implement it.
type PacketSink interface {
	SendPacket(p []byte)
}

// Player is the map-side view of a character. Java keys the player object map
// by getObjectId(), which for characters is always the character id
// (MapleCharacter.getObjectId -> getId).
type Player interface {
	PacketSink
	// ObjectID returns the map object id (Java MapleMapObject.getObjectId).
	ObjectID() int
	// SendSpawnData ports MapleMapObject.sendSpawnData: write this player's
	// spawn packet (Java spawnPlayerMapobject) to sink.
	SendSpawnData(sink PacketSink)
	// DespawnData returns the despawn packet for this player (Java
	// MapleMap.removePlayer -> MaplePacketCreator.removePlayerFromMap(id)).
	DespawnData() []byte
}

// Map is one channel's instance of a map id (Java MapleMap). P4.3 keeps the
// player set and the spawn/despawn broadcasts; life / drops / footholds
// arrive later.
type Map struct {
	id int

	mu      sync.RWMutex
	players map[int]Player
}

// New creates an empty map instance for the given map id.
func New(id int) *Map {
	return &Map{id: id, players: map[int]Player{}}
}

// ID returns the map id (Java getMapId).
func (m *Map) ID() int { return m.id }

// AddPlayer ports the player-tracking and broadcast half of Java
// MapleMap.addPlayer:
//
//	characters/objects.insert(chr)
//	broadcastMessage(chr, spawnPlayerMapobject(chr), false)  // everyone but chr
//	sendObjectPlacement(chr)  // chr learns about every object on the map
//	chr.getClient().sendPacket(spawnPlayerMapobject(chr))    // own spawn
//
// sendObjectPlacement in Java is view-range filtered and covers every map
// object type. P4.3 has no monsters/items and no character position yet
// (movement is P4.4), so the Go port sends every other player on the map.
//
// Java also re-sends the newcomer's own spawn through sendObjectPlacement
// (chr is already in the object map when the range loop runs) and then again
// explicitly; the port sends it once.
func (m *Map) AddPlayer(p Player) {
	m.mu.Lock()
	m.players[p.ObjectID()] = p
	others := make([]Player, 0, len(m.players))
	for _, q := range m.players {
		if q.ObjectID() != p.ObjectID() {
			others = append(others, q)
		}
	}
	m.mu.Unlock()

	// The players already on the map learn about the newcomer...
	for _, q := range others {
		p.SendSpawnData(q)
	}
	// ...and the newcomer learns about each of them.
	for _, q := range others {
		q.SendSpawnData(p)
	}
	p.SendSpawnData(p)
}

// RemovePlayer ports MapleMap.removePlayer: the character leaves the object
// map, then everyone remaining is told to drop the sprite. Java removes the
// character before broadcasting, so the departing player never sees its own
// despawn. It returns whether the player was on the map.
func (m *Map) RemovePlayer(p Player) bool {
	m.mu.Lock()
	cur, ok := m.players[p.ObjectID()]
	if !ok || cur != p {
		m.mu.Unlock()
		return false
	}
	delete(m.players, p.ObjectID())
	m.mu.Unlock()

	m.Broadcast(p.DespawnData(), nil)
	return true
}

// Broadcast writes pkt to every player on the map except (when non-nil) the
// source (Java MapleMap.broadcastMessage(source, packet, repeatToSource) with
// repeatToSource=false; the ranged overloads arrive with the view range).
func (m *Map) Broadcast(pkt []byte, except Player) {
	for _, q := range m.snapshot() {
		if except != nil && q.ObjectID() == except.ObjectID() {
			continue
		}
		q.SendPacket(pkt)
	}
}

func (m *Map) snapshot() []Player {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Player, 0, len(m.players))
	for _, p := range m.players {
		out = append(out, p)
	}
	return out
}

// Players returns a snapshot of the players on this map.
func (m *Map) Players() []Player { return m.snapshot() }

// PlayerCount returns the number of players on this map.
func (m *Map) PlayerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.players)
}
