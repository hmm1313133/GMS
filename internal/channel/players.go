// Package channel - per-channel online player registry (Java
// handling/channel/PlayerStorage).
//
// P4.1 skeleton: players register/deregister by id and (lowercased) name and
// the storage reports the live count. P4.2 adds the loaded character row, the
// per-character random stream and the CharacterTransfer pending table (the
// channel-change handoff). The 15-minute PersistingTask save sweep arrives
// with P5.
package channel

import (
	"sync"
	"time"

	"GMS/internal/database"
	"GMS/internal/mapp"
	"GMS/internal/movement"
	"GMS/internal/netw"
	"GMS/internal/packet"
	"GMS/internal/world"
)

// pendingTransferTTL mirrors the Java PlayerStorage.PersistingTask sweep that
// drops pending transfers older than 40s.
const pendingTransferTTL = 40 * time.Second

// Player is the channel-side player handle (a slice of Java MapleCharacter).
// P4.2 carries the loaded characters row; P5.1 grows it into the real
// character port (inventory, skills, buffs).
type Player struct {
	ID   int
	Name string
	Sess *netw.Session
	// Chr is the loaded characters row (Java loadCharFromDB result subset).
	// It supplies every field of the WARP_TO_MAP stat block.
	Chr *database.Character
	// Rand is the per-character CRand32 stream (Java MapleCharacter.CRand),
	// serialised into WARP_TO_MAP; P6 reuses it for damage rolls.
	Rand *packet.RandStream
	// MapIDVal is the map the character stands on (Java getMapId). The map
	// instance itself is tracked by the owning ChannelServer; P4.3 keeps this
	// current on warp (Chr.Map only holds the persisted spawn map).
	MapIDVal int

	// mu guards the live map state below (Pos/Stance/Fh/OldPos/FallCounter): it
	// is written on this player's MOVE_PLAYER goroutine and read from *other*
	// players' goroutines by mapp.Map.BroadcastRanged (P4.5 chat view-range).
	sync.RWMutex

	// Pos is the live map position (Java MapleMapObject.getPosition),
	// Stance the animation stance and Fh the foothold id - the state
	// MovementParse.updatePosition drives (P4.4). OldPos/FallCounter are the
	// Java fall-detector bookkeeping. Every field of this block is guarded by
	// the embedded mutex above: writers hold Lock, readers hold RLock (the
	// accessors below do it). The stance field is unexported only because Go
	// forbids a field and a method sharing the name Stance on one type; the
	// exported accessor is Player.Stance.
	Pos         movement.Point
	stance      int
	Fh          int
	OldPos      movement.Point
	FallCounter int
}

// newPlayer builds the channel-side handle from a loaded characters row.
func newPlayer(chr *database.Character, sess *netw.Session) *Player {
	return &Player{
		ID:       chr.ID,
		Name:     chr.Name,
		Sess:     sess,
		Chr:      chr,
		Rand:     packet.NewRandStream(),
		MapIDVal: chr.Map,
	}
}

// ObjectID returns the map object id; for characters Java defines it as the
// character id (MapleCharacter.getObjectId -> getId). Satisfies mapp.Player.
func (p *Player) ObjectID() int { return p.ID }

// Position returns the live map position (Java MapleMapObject.getPosition).
// Satisfies mapp.Player: the ranged broadcastMessage overload (P4.5 chat)
// filters on it. The read is taken under the embedded mutex, so it is safe
// from the *other* players' goroutines that run that view-range filter while
// this player's own MOVE_PLAYER goroutine writes the position.
func (p *Player) Position() (int16, int16) {
	p.RLock()
	defer p.RUnlock()
	return p.Pos.X, p.Pos.Y
}

// Stance returns the live animation stance (Java getStance) under the same
// locking pattern as Position.
func (p *Player) Stance() int {
	p.RLock()
	defer p.RUnlock()
	return p.stance
}

// IsGM ports MapleCharacter.isGM: characters.gm > 0 (Java clamps the column at
// 6 into gmLevel; for the >0 test the clamp is irrelevant).
func (p *Player) IsGM() bool { return p.Chr != nil && p.Chr.GM > 0 }

// SendPacket writes a packet body to this player's client (Java
// MapleClient.sendPacket). No-op while the session is absent (a pending
// CharacterTransfer is not connected yet) or closed.
func (p *Player) SendPacket(b []byte) {
	if p.Sess != nil {
		p.Sess.Write(b)
	}
}

// SendSpawnData ports MapleMapObject.sendSpawnData for a character: send this
// player's SPAWN_PLAYER (MaplePacketCreator.spawnPlayerMapobject) to sink,
// carrying the live position/stance (P4.4). Satisfies mapp.Player.
func (p *Player) SendSpawnData(sink mapp.PacketSink) {
	// Snapshot the guarded fields, release the lock, then write to the sink:
	// the mutex guards the state, never the network call.
	p.RLock()
	x, y, stance := p.Pos.X, p.Pos.Y, p.stance
	p.RUnlock()
	sink.SendPacket(packet.SpawnPlayerPacket(p.Chr, x, y, stance))
}

// SetPosition / SetFh / SetStance satisfy movement.Target (Java
// AnimatedMapleMapObject setters driven by MovementParse.updatePosition).
// These run on this player's own connection goroutine; taking the lock is what
// makes the writes visible to the cross-goroutine readers (Position, Stance,
// SendSpawnData).
func (p *Player) SetPosition(x, y int16) {
	p.Lock()
	p.Pos = movement.Point{X: x, Y: y}
	p.Unlock()
}

func (p *Player) SetFh(fh int) {
	p.Lock()
	p.Fh = fh
	p.Unlock()
}

func (p *Player) SetStance(s int) {
	p.Lock()
	p.stance = s
	p.Unlock()
}

// ApplyMovement ports the tail of Java PlayerHandler.MovePlayer:
// MovementParse.updatePosition(res, chr, 0) followed by
// chr.setOldPosition(chr.getPosition()). Map.movePlayer only re-sets the
// position and refreshes object visibility of the (not yet ported) map
// objects, so here it collapses into the position update.
func (p *Player) ApplyMovement(moves []movement.Fragment) {
	movement.UpdatePosition(moves, p, 0)
	p.Lock()
	p.OldPos = p.Pos
	p.Unlock()
}

// DespawnData returns this player's REMOVE_PLAYER_FROM_MAP packet (Java
// MapleMap.removePlayer -> MaplePacketCreator.removePlayerFromMap(id)).
// Satisfies mapp.Player.
func (p *Player) DespawnData() []byte {
	return packet.RemovePlayerFromMapPacket(p.ID)
}

// MapID returns the map the player stands on (Java getMapId).
func (p *Player) MapID() int {
	if p.Chr != nil {
		return p.Chr.Map
	}
	return p.MapIDVal
}

// AccountID returns the owning account id (Java getAccountID).
func (p *Player) AccountID() int {
	if p.Chr != nil {
		return p.Chr.AccountID
	}
	return 0
}

// CharacterTransfer is a pending character handed over from another channel
// (Java handling/world/CharacterTransfer). Java stores the full field set so
// the character can be rebuilt without a DB round-trip; the Go port carries
// the already-built Player and re-attaches the new session on pickup.
type CharacterTransfer struct {
	Player       *Player
	TransferTime time.Time
}

// PlayerStorage ports Java handling/channel.PlayerStorage: nameToChar +
// idToChar maps under a RW lock, plus the World.Find registration side
// effects and the pending-transfer table.
type PlayerStorage struct {
	channel int
	find    *world.Finder
	onLoad  func(channel, players int)

	mu      sync.RWMutex
	names   map[string]*Player // lowercase name -> player (Java nameToChar)
	ids     map[int]*Player    // id -> player (Java idToChar)
	pending map[int]*CharacterTransfer
}

func newPlayerStorage(channel int, find *world.Finder, onLoad func(channel, players int)) *PlayerStorage {
	return &PlayerStorage{
		channel: channel,
		find:    find,
		onLoad:  onLoad,
		names:   map[string]*Player{},
		ids:     map[int]*Player{},
		pending: map[int]*CharacterTransfer{},
	}
}

// RegisterPlayer ports PlayerStorage.registerPlayer (including the
// World.Find.register side effect).
func (ps *PlayerStorage) RegisterPlayer(p *Player) {
	ps.mu.Lock()
	ps.names[lowerName(p.Name)] = p
	ps.ids[p.ID] = p
	n := len(ps.ids)
	ps.mu.Unlock()
	if ps.find != nil {
		ps.find.Register(p.ID, p.Name, ps.channel)
	}
	ps.report(n)
}

// DeregisterPlayer ports PlayerStorage.deregisterPlayer(chr).
func (ps *PlayerStorage) DeregisterPlayer(p *Player) {
	ps.DeregisterPlayerByID(p.ID, p.Name)
}

// DeregisterPlayerByID ports PlayerStorage.deregisterPlayer(id, name)
// (including World.Find.forceDeregister).
func (ps *PlayerStorage) DeregisterPlayerByID(id int, name string) {
	ps.mu.Lock()
	if p, ok := ps.ids[id]; ok {
		delete(ps.names, lowerName(p.Name))
		delete(ps.ids, id)
	} else if p, ok := ps.names[lowerName(name)]; ok && p.ID == id {
		delete(ps.names, lowerName(name))
	}
	n := len(ps.ids)
	ps.mu.Unlock()
	if ps.find != nil {
		ps.find.ForceDeregister(id, name)
	}
	ps.report(n)
}

// RegisterPendingPlayer ports PlayerStorage.registerPendingPlayer: park a
// character for the channel it is being handed to.
func (ps *PlayerStorage) RegisterPendingPlayer(p *Player) {
	ps.mu.Lock()
	ps.pending[p.ID] = &CharacterTransfer{Player: p, TransferTime: time.Now()}
	ps.mu.Unlock()
}

// GetPendingCharacter ports PlayerStorage.getPendingCharacter: fetch the
// transfer (removing it - one-shot) or nil. Java sweeps entries older than 40s
// from its PersistingTask; the Go port checks the age on read instead of
// running that 15-minute ticker (it arrives with the P5 save loop).
func (ps *PlayerStorage) GetPendingCharacter(id int) *Player {
	ps.mu.Lock()
	t, ok := ps.pending[id]
	if ok {
		delete(ps.pending, id)
	}
	ps.mu.Unlock()
	if !ok || time.Since(t.TransferTime) > pendingTransferTTL {
		return nil
	}
	return t.Player
}

// DeregisterPendingPlayer ports PlayerStorage.deregisterPendingPlayer.
func (ps *PlayerStorage) DeregisterPendingPlayer(id int) {
	ps.mu.Lock()
	delete(ps.pending, id)
	ps.mu.Unlock()
}

// GetCharacterByName ports PlayerStorage.getCharacterByName (lowercase key).
func (ps *PlayerStorage) GetCharacterByName(name string) *Player {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.names[lowerName(name)]
}

// GetPlayerByID ports PlayerStorage.getCharacterById.
func (ps *PlayerStorage) GetPlayerByID(id int) *Player {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.ids[id]
}

// GetAllCharacters ports PlayerStorage.getAllCharacters (snapshot copy).
func (ps *PlayerStorage) GetAllCharacters() []*Player {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]*Player, 0, len(ps.ids))
	for _, p := range ps.ids {
		out = append(out, p)
	}
	return out
}

// ConnectedPlayers ports PlayerStorage.getConnectedPlayers.
func (ps *PlayerStorage) ConnectedPlayers() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.ids)
}

func (ps *PlayerStorage) report(n int) {
	if ps.onLoad != nil {
		ps.onLoad(ps.channel, n)
	}
}

func lowerName(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
