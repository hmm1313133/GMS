package channel

// P4.7 e2e over the wire: the portal-based spawn point at login (both the
// loadCharFromDB and the CharacterTransfer variants), the CHANGE_MAP transfer
// table, the portal transfers (CHANGE_MAP / CHANGE_MAP_SPECIAL), the revive
// path and USE_INNER_PORTAL.

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
	"GMS/internal/wzs"
)

// startChannelMaps starts a channel server whose Map.wz comes from the package
// testdata (Java MapleMapFactory's provider). The fixtures are:
//
//	910000100  start map:    portal 0 "sp" (-5,-10); "out" -> 910000101/"in";
//	                         "scripted" (script=warp); "notarget" (tm=no map);
//	                         "badtn" (tn=nosuch); returnMap 910000101
//	910000101  target map:   portal 0 "sp" (-93,450); "in" -> 910000100/"sp"
//	910000102  forced return map: portal 0 "sp" (11,22)
//	910000103  map with forcedReturn 910000102, portal 0 "sp" (7,8)
//	0 / 10000  the CHANGE_MAP transfer-table pair (0 -> 10000)
func startChannelMaps(t *testing.T, cfg Config, store characterStore) *Server {
	t.Helper()
	s := startChannelWith(t, cfg, store)
	root, err := wzs.OpenRoot("testdata/wz")
	require.NoError(t, err)
	s.SetWZ(root)
	return s
}

// mapChar is testChar with the fields the warp tests steer: the spawn point,
// the HP (0 = dead, which takes the revive path) and the GM level.
func mapChar(id, accID, mapID, spawn, hp, gm int) *database.Character {
	c := testChar(id, accID, mapID)
	c.Spawnpoint = spawn
	c.HP = hp
	c.GM = gm
	return c
}

// sendChangeMap builds a CHANGE_MAP body: byte skip + int targetid + str portal
// + short wheel.
func sendChangeMap(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, targetID int, portal string) {
	t.Helper()
	w := protocol.NewWriter(32 + len(portal))
	w.Short(int(protocol.RecvCHANGE_MAP))
	w.Byte(0)
	w.Int(int32(targetID))
	w.MapleAsciiString(portal)
	w.Short(0)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

// sendChangeMapSpecial builds a CHANGE_MAP_SPECIAL body: byte skip + short +
// str portal (the dispatcher skips its own byte, the handler reads the short).
func sendChangeMapSpecial(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, portal string) {
	t.Helper()
	w := protocol.NewWriter(32 + len(portal))
	w.Short(int(protocol.RecvCHANGE_MAP_SPECIAL))
	w.Byte(0)
	w.Short(0)
	w.MapleAsciiString(portal)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

// sendInnerPortal builds a USE_INNER_PORTAL body: byte skip + str portal +
// short x + short y.
func sendInnerPortal(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, portal string, x, y int16) {
	t.Helper()
	w := protocol.NewWriter(32 + len(portal))
	w.Short(int(protocol.RecvUSE_INNER_PORTAL))
	w.Byte(0)
	w.MapleAsciiString(portal)
	w.Pos(x, y)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

// readCharInfoSpawn walks the getCharInfo body (the login packet) up to the
// map id and spawn point of its addCharStats block.
func readCharInfoSpawn(t *testing.T, body []byte) (mapID, spawn int) {
	t.Helper()
	r := protocol.NewReader(body)
	r.Short() // WARP_TO_MAP
	r.Int()   // channel - 1
	r.Byte()  // 0
	r.Byte()  // 1
	r.Byte()  // 1
	r.Short() // 0
	r.Int()
	r.Int()
	r.Int() // CRand connectData
	r.Long()
	r.Byte()
	r.Int()    // id
	r.Skip(13) // name
	r.Byte()   // gender
	r.Byte()   // skin
	r.Int()    // face
	r.Int()    // hair
	r.Skip(24)
	r.Byte()  // level
	r.Short() // job
	r.Skip(20)
	r.Int()   // exp
	r.Short() // fame
	r.Int()   // gachapon
	r.Long()  // time
	mapID = int(r.Int())
	spawn = int(r.Byte())
	require.NoError(t, r.Err)
	return mapID, spawn
}

// readTransferSpawn walks the short getWarpToMap body (the in-channel transfer)
// up to its map id and spawn point: those are written directly after the
// marker block instead of inside addCharStats.
func readTransferSpawn(t *testing.T, body []byte) (mapID, spawn int) {
	t.Helper()
	r := protocol.NewReader(body)
	r.Short() // WARP_TO_MAP
	r.Int()   // channel - 1
	r.Byte()  // 0
	r.Byte()  // 3
	r.Short() // 0
	r.Byte()  // 0
	mapID = int(r.Int())
	spawn = int(r.Byte())
	hp := r.Short()
	require.NoError(t, r.Err)
	require.NotZero(t, hp, "the transfer packet carries the HP")
	return mapID, spawn
}

// spawnTailPos decodes the position/stance of a SPAWN_PLAYER body. The packet
// tail is fixed (see packet.SpawnPlayerPacket): pos(4) + stance(1) + foothold(2)
// + pets(1) + mount level/exp/fatigue(12) + announce box(1) + chalkboard(1)
// + addRingInfo x2 (10) + marriage ring(1) + short(2), i.e. 35 bytes with the
// position on top.
func spawnTailPos(t *testing.T, body []byte) (x, y int16, stance int) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 35)
	r := protocol.NewReader(body[len(body)-35:])
	x, y = r.Pos()
	stance = int(r.Byte())
	require.NoError(t, r.Err)
	return x, y, stance
}

func TestLoginSpawnComesFromPortal(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 0, 50, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, _, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()

	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readCharInfoSpawn(t, warp)
	assert.Equal(t, 910000100, mapID)
	assert.Equal(t, 0, spawn, "the spawn byte is the portal id the character lands on")
	require.Equal(t, uint16(protocol.SendTEMP_STATS_RESET), opOf(readTestReply(t, conn, recv)))

	// The own spawn carries the portal position instead of (0,0).
	self := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(self))
	x, y, _ := spawnTailPos(t, self)
	assert.Equal(t, int16(-5), x)
	assert.Equal(t, int16(-10), y)

	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)
	px, py := p.Position()
	assert.Equal(t, int16(-5), px)
	assert.Equal(t, int16(-10), py)
	waitFor(t, func() bool { return cs.Map(910000100).PlayerCount() == 1 })
}

func TestLoginSpawnFallsBackToPortalZero(t *testing.T) {
	// The characters row points at portal 7, which the map does not have:
	// Java falls back to portal 0 and rewrites initialSpawnPoint, and the
	// WARP_TO_MAP spawn byte must reflect the rewrite (not the stale 7).
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 7, 50, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, _, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()

	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readCharInfoSpawn(t, warp)
	assert.Equal(t, 910000100, mapID)
	assert.Equal(t, 0, spawn, "a stale spawn point falls back to portal 0")

	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)
	assert.Equal(t, 0, p.Chr.Spawnpoint, "the loaded row is rewritten like the Java character")
	x, y := p.Position()
	assert.Equal(t, int16(-5), x)
	assert.Equal(t, int16(-10), y)
}

func TestLoginSpawnFallsBackToHenesysWhenImageIsMissing(t *testing.T) {
	// Java loadCharFromDB:646: a character whose map image is gone is moved to
	// 100000000 ("warp it to henesys") and spawns on that map's portal 0.
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000999, 0, 50, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, _, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()

	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readCharInfoSpawn(t, warp)
	assert.Equal(t, 100000000, mapID)
	assert.Equal(t, 0, spawn)

	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)
	x, y := p.Position()
	assert.Equal(t, int16(12), x)
	assert.Equal(t, int16(34), y)
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })
}

func TestForcedReturnOnlyAppliesToTransfer(t *testing.T) {
	// MapleCharacter keeps two copies of the spawn block: loadCharFromDB
	// (:919-932, the DB path) does NOT rewrite the map, ReconstructChr
	// (:648-650, the CharacterTransfer path) rewrites it when the map has a
	// forced return. Map 910000103 has forcedReturn = 910000102.
	row := func() *database.Character { return mapChar(31, 7, 910000103, 0, 50, 0) }

	// DB path: stay on 910000103, on its own portal 0.
	{
		fs := &fakeStore{chars: map[int]*database.Character{31: row()}}
		s := startChannelMaps(t, Config{Count: 1}, fs)
		conn, _, recv := sendLoggedIn(t, s.Channel(1), 31)
		defer conn.Close()
		warp := readTestReply(t, conn, recv)
		mapID, spawn := readCharInfoSpawn(t, warp)
		assert.Equal(t, 910000103, mapID)
		assert.Equal(t, 0, spawn)
		p := s.Channel(1).Players().GetPlayerByID(31)
		require.NotNil(t, p)
		x, y := p.Position()
		assert.Equal(t, int16(7), x)
		assert.Equal(t, int16(8), y)
	}

	// Transfer path: the forced return map takes over (portal 0 of 910000102).
	{
		fs := &fakeStore{chars: map[int]*database.Character{31: row()}}
		s := startChannelMaps(t, Config{Count: 1}, fs)
		cs := s.Channel(1)
		pending := newPlayer(row(), nil)
		cs.Players().RegisterPendingPlayer(pending)

		conn, _, recv := sendLoggedIn(t, cs, 31)
		defer conn.Close()
		warp := readTestReply(t, conn, recv)
		mapID, spawn := readCharInfoSpawn(t, warp)
		assert.Equal(t, 910000102, mapID, "the transfer path applies the forced return")
		assert.Equal(t, 0, spawn)
		assert.Same(t, pending, cs.Players().GetPlayerByID(31))
		x, y := pending.Position()
		assert.Equal(t, int16(11), x)
		assert.Equal(t, int16(22), y)
	}
}

func TestChangeMapTableTransfer(t *testing.T) {
	// The non-GM transfer table entry `mapId == 0 && targetid == 10000`:
	// enableActions, then the warp packet, then the old map drops the sprite
	// and the new map spawns it.
	fs := &fakeStore{chars: map[int]*database.Character{
		31: mapChar(31, 7, 0, 0, 50, 0),
		32: mapChar(32, 8, 0, 0, 50, 0),
	}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)
	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1) // A learns about B
	waitFor(t, func() bool { return cs.Map(0).PlayerCount() == 2 })

	allowReads(t, connA)
	sendChangeMap(t, connA, sendA, 10000, "")

	// A: enableActions first (Java sends it inside the table entry)...
	actions := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendUPDATE_STATS), opOf(actions))
	assert.Equal(t, []byte{0x22, 0x00, 0x01, 0, 0, 0, 0, 0, 0}, actions)

	// ...then the transfer packet with the target map and its portal 0...
	warp := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readTransferSpawn(t, warp)
	assert.Equal(t, 10000, mapID)
	assert.Equal(t, 0, spawn)
	assert.Equal(t, int16(50), protocol.NewReader(warp[len(warp)-10:]).Short(),
		"the packet carries the character's HP before the trailing timestamp")

	// ...and the new map spawns A at that portal.
	self := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(self))
	x, y, _ := spawnTailPos(t, self)
	assert.Equal(t, int16(-93), x)
	assert.Equal(t, int16(450), y)

	// B (still on map 0) is told to drop A's sprite.
	allowReads(t, connB)
	despawn := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendREMOVE_PLAYER_FROM_MAP), opOf(despawn))
	assert.Equal(t, int32(31), protocol.NewReader(despawn[2:]).Int())

	pA := cs.Players().GetPlayerByID(31)
	require.NotNil(t, pA)
	assert.Equal(t, 10000, pA.MapID())
	px, py := pA.Position()
	assert.Equal(t, int16(-93), px)
	assert.Equal(t, int16(450), py)
	waitFor(t, func() bool { return cs.Map(10000).PlayerCount() == 1 })
	assert.Equal(t, 1, cs.Map(0).PlayerCount(), "only B is left behind")
}

func TestChangeMapPortalTransferAndFallbacks(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 0, 50, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, send, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()
	drain(t, conn, recv, 3)
	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)

	// A portal whose tm is the "no map" sentinel does not transfer: the
	// trailing enableActions fires.
	allowReads(t, conn)
	sendChangeMap(t, conn, send, -1, "notarget")
	require.Equal(t, uint16(protocol.SendUPDATE_STATS), opOf(readTestReply(t, conn, recv)))
	assert.Equal(t, 910000100, p.MapID())

	// A scripted portal is the P7 script host: still no transfer.
	sendChangeMap(t, conn, send, -1, "scripted")
	require.Equal(t, uint16(protocol.SendUPDATE_STATS), opOf(readTestReply(t, conn, recv)))
	assert.Equal(t, 910000100, p.MapID())

	// An unknown portal name -> caller answers enableActions.
	sendChangeMap(t, conn, send, -1, "nope")
	require.Equal(t, uint16(protocol.SendUPDATE_STATS), opOf(readTestReply(t, conn, recv)))
	assert.Equal(t, 910000100, p.MapID())

	// An unknown "tn" falls back to the target map's portal 0 (spawn 0).
	sendChangeMap(t, conn, send, -1, "badtn")
	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readTransferSpawn(t, warp)
	assert.Equal(t, 910000101, mapID)
	assert.Equal(t, 0, spawn, "unknown tn -> portal 0 of the target map")
	self := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(self))
	x, y, _ := spawnTailPos(t, self)
	assert.Equal(t, int16(-93), x)
	assert.Equal(t, int16(450), y)
	waitFor(t, func() bool { return p.MapID() == 910000101 })
	assert.Equal(t, 0, cs.Map(910000100).PlayerCount(), "the old map dropped the sprite")
	assert.Equal(t, 1, cs.Map(910000101).PlayerCount())

	// CHANGE_MAP_SPECIAL enters a portal by name: the target map's "in" portal
	// points back at 910000100/"sp" (portal id 0).
	allowReads(t, conn)
	sendChangeMapSpecial(t, conn, send, "in")
	warp = readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn = readTransferSpawn(t, warp)
	assert.Equal(t, 910000100, mapID)
	assert.Equal(t, 0, spawn, "tn=sp resolves to portal 0")
	self = readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(self))
	x, y, _ = spawnTailPos(t, self)
	assert.Equal(t, int16(-5), x)
	assert.Equal(t, int16(-10), y)

	// ...and the "out" portal of the start map resolves tn=in to portal id 1.
	allowReads(t, conn)
	sendChangeMap(t, conn, send, -1, "out")
	warp = readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn = readTransferSpawn(t, warp)
	assert.Equal(t, 910000101, mapID)
	assert.Equal(t, 1, spawn, "tn=in resolves to portal id 1")
	assert.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(readTestReply(t, conn, recv)))
}

func TestChangeMapUnmatchedTargetIsSilent(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 0, 50, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, send, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()
	drain(t, conn, recv, 3)

	// A non-GM target id outside the transfer table is dropped without a reply
	// (Java has no else on that branch).
	sendChangeMap(t, conn, send, 910000101, "")
	expectSilence(t, conn)

	// The empty body (channel change) is not ported: ignored, session stays.
	w := protocol.NewWriter(2)
	w.Short(int(protocol.RecvCHANGE_MAP))
	sendTestFrame(t, conn, send, w.Bytes())
	expectSilence(t, conn)

	assert.Equal(t, 1, cs.Map(910000100).PlayerCount())
	require.NotNil(t, cs.Players().GetPlayerByID(31))
}

func TestChangeMapGMUsesPortalZero(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 0, 50, 5)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, send, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()
	drain(t, conn, recv, 3)

	// The GM branch warps to portal 0 of the requested map and sends no
	// enableActions first.
	allowReads(t, conn)
	sendChangeMap(t, conn, send, 910000101, "")
	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readTransferSpawn(t, warp)
	assert.Equal(t, 910000101, mapID)
	assert.Equal(t, 0, spawn)

	self := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(self))
	x, y, _ := spawnTailPos(t, self)
	assert.Equal(t, int16(-93), x)
	assert.Equal(t, int16(450), y)
	waitFor(t, func() bool { return cs.Map(910000101).PlayerCount() == 1 })
}

func TestChangeMapRevivesDeadCharacterAtReturnMap(t *testing.T) {
	// targetid != -1 with hp 0 takes the revive branch: HP is set to 50 and the
	// character is moved to the map's return map (map 910000100 -> 910000101).
	fs := &fakeStore{chars: map[int]*database.Character{31: mapChar(31, 7, 910000100, 0, 0, 0)}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	conn, send, recv := sendLoggedIn(t, cs, 31)
	defer conn.Close()
	drain(t, conn, recv, 3)
	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)
	assert.False(t, p.IsAlive())

	allowReads(t, conn)
	sendChangeMap(t, conn, send, 10000, "")
	warp := readTestReply(t, conn, recv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	mapID, spawn := readTransferSpawn(t, warp)
	assert.Equal(t, 910000101, mapID, "the return map, not the requested target")
	assert.Equal(t, 0, spawn)
	assert.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(readTestReply(t, conn, recv)))
	waitFor(t, func() bool { return p.MapID() == 910000101 })
	assert.True(t, p.IsAlive())
	assert.Equal(t, 50, p.HP())
}

func TestInnerPortalMovesPositionWithoutBroadcast(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: mapChar(31, 7, 910000100, 0, 50, 0),
		32: mapChar(32, 8, 910000100, 0, 50, 0),
	}}
	s := startChannelMaps(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)
	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1)

	pA := cs.Players().GetPlayerByID(31)
	require.NotNil(t, pA)

	sendInnerPortal(t, connA, sendA, "sp", 123, -456)
	expectSilence(t, connA) // Java sends no reply
	expectSilence(t, connB) // and no position broadcast either
	waitFor(t, func() bool {
		x, y := pA.Position()
		return x == 123 && y == -456
	})

	// An unknown portal is ignored (Java returns before reading the target).
	sendInnerPortal(t, connA, sendA, "nope", 1, 2)
	expectSilence(t, connA)
	x, y := pA.Position()
	assert.Equal(t, int16(123), x)
	assert.Equal(t, int16(-456), y)
}
