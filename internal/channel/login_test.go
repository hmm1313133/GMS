package channel

// P4.2 e2e over the wire: PLAYER_LOGGEDIN -> character load -> PlayerStorage
// registration -> WARP_TO_MAP + TEMP_STATS_RESET (the client enters the map),
// plus the guard branches (unknown character, no store), the CharacterTransfer
// pending path and the duplicate-account eviction.

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
)

// fakeStore implements characterStore for the channel tests.
type fakeStore struct {
	chars map[int]*database.Character
	err   error
}

func (f *fakeStore) GetCharacterByID(_ context.Context, id int) (*database.Character, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.chars[id], nil
}

func testChar(id, accID, mapID int) *database.Character {
	return &database.Character{
		ID: id, AccountID: accID, World: 0, Name: "测试者",
		Level: 1, Str: 12, Dex: 5, Luk: 4, Int: 4,
		HP: 50, MP: 50, MaxHP: 50, MaxMP: 50,
		Job: 0, SkinColor: 0, Gender: 1, Fame: 0,
		Hair: 30000, Face: 20000, Map: mapID,
		BuddyCapacity: 20,
	}
}

func startChannelWith(t *testing.T, cfg Config, store characterStore) *Server {
	t.Helper()
	cfg.Host = "127.0.0.1"
	cfg.BasePort = 0 // :0 - OS-assigned ports
	s := New(cfg, slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	if store != nil {
		s.SetStore(store)
	}
	require.NoError(t, s.Start())
	t.Cleanup(s.Stop)
	return s
}

// opOf reads the send opcode out of a decrypted body.
func opOf(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }

// waitFor polls cond for up to a second (the handler runs on the session
// goroutine; the frames it wrote may already be in hand when we assert state).
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within a second")
}

// sendLoggedIn performs the handshake and sends PLAYER_LOGGEDIN.
func sendLoggedIn(t *testing.T, cs *ChannelServer, charID int) (net.Conn, *crypto.AESOFB, *crypto.AESOFB) {
	t.Helper()
	conn, cSend, cRecv := dialChannel(t, cs.acceptor.Addr().String())
	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvPLAYER_LOGGEDIN))
	w.Int(int32(charID))
	sendTestFrame(t, conn, cSend, w.Bytes())
	return conn, cSend, cRecv
}

func TestPlayerLoggedInEntersMap(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{31: testChar(31, 7, 100000000)}}
	s := startChannelWith(t, Config{Count: 1, ServerMessage: "欢迎"}, fs)
	cs := s.Channel(1)

	conn, _, cRecv := sendLoggedIn(t, cs, 31)
	defer conn.Close()

	// Java ChannelServer.addPlayer pushes the scrolling banner first.
	banner := readTestReply(t, conn, cRecv)
	require.Equal(t, uint16(protocol.SendSERVERMESSAGE), opOf(banner))
	assert.Equal(t, byte(4), banner[2])
	assert.Equal(t, "欢迎", protocol.NewReader(banner[4:]).MapleAsciiString())

	// getCharInfo -> WARP_TO_MAP.
	warp := readTestReply(t, conn, cRecv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	r := protocol.NewReader(warp)
	r.Short()                              // WARP_TO_MAP
	assert.Equal(t, 0, int(r.Int()), "channel 1 is written as channel-1 = 0")
	r.Byte()                               // 0
	r.Byte()                               // 1
	r.Byte()                               // 1
	r.Short()                              // 0
	r.Int()                                // CRand x3
	r.Int()
	r.Int()
	r.Long()                               // addCharacterInfo: long -1
	r.Byte()                               // addCharacterInfo: byte 0
	assert.Equal(t, int32(31), r.Int(), "addCharStats id")
	require.NoError(t, r.Err)

	// temporaryStats_Reset.
	reset := readTestReply(t, conn, cRecv)
	require.Equal(t, uint16(protocol.SendTEMP_STATS_RESET), opOf(reset))
	assert.Len(t, reset, 2)

	// channel registration + World.Find side effect + map membership.
	p := cs.Players().GetPlayerByID(31)
	require.NotNil(t, p)
	assert.Equal(t, "测试者", p.Name)
	assert.Equal(t, 7, p.AccountID())
	assert.NotNil(t, p.Rand)
	assert.Same(t, p, cs.Players().GetCharacterByName("测试者"))
	e, ok := s.Finder().Find(31)
	require.True(t, ok)
	assert.Equal(t, 1, e.Channel)
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })
	assert.Same(t, p, cs.Map(100000000).Players()[0])

	// disconnecting deregisters the player and empties the map.
	conn.Close()
	waitFor(t, func() bool { return cs.Players().GetPlayerByID(31) == nil })
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 0 })
	_, ok = s.Finder().Find(31)
	assert.False(t, ok)
}

func TestMapSpawnAndDespawnBroadcast(t *testing.T) {
	// P4.3: two clients on the same map see each other (SPAWN_PLAYER) and the
	// remaining player is told to drop the sprite on disconnect
	// (REMOVE_PLAYER_FROM_MAP).
	fs := &fakeStore{chars: map[int]*database.Character{
		31: testChar(31, 7, 100000000),
		32: testChar(32, 8, 100000000),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	// First player: WARP_TO_MAP, TEMP_STATS_RESET, then its own spawn (nobody
	// else is on the map yet).
	connA, _, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(readTestReply(t, connA, recvA)))
	require.Equal(t, uint16(protocol.SendTEMP_STATS_RESET), opOf(readTestReply(t, connA, recvA)))
	selfA := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(selfA))
	assert.Equal(t, int32(31), protocol.NewReader(selfA[2:]).Int())
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })

	// Second player joins the same map.
	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(readTestReply(t, connB, recvB)))
	require.Equal(t, uint16(protocol.SendTEMP_STATS_RESET), opOf(readTestReply(t, connB, recvB)))

	// The newcomer learns about the player already there, then spawns itself.
	spawnA := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(spawnA))
	assert.Equal(t, int32(31), protocol.NewReader(spawnA[2:]).Int(), "newcomer sees player 31")
	spawnB := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(spawnB))
	assert.Equal(t, int32(32), protocol.NewReader(spawnB[2:]).Int(), "newcomer gets its own spawn")

	// The player already on the map learns about the newcomer.
	bcast := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), opOf(bcast))
	assert.Equal(t, int32(32), protocol.NewReader(bcast[2:]).Int(), "existing player sees player 32")

	// B disconnects -> A is told to remove the sprite.
	connB.Close()
	despawn := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendREMOVE_PLAYER_FROM_MAP), opOf(despawn))
	assert.Equal(t, int32(32), protocol.NewReader(despawn[2:]).Int())
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })
}

func TestPlayerLoggedInUnknownCharacterCloses(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{}}
	s := startChannelWith(t, Config{Count: 1}, fs)

	conn, _, _ := sendLoggedIn(t, s.Channel(1), 99)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("an unknown character must close the session")
	}
	assert.Equal(t, 0, s.Channel(1).Players().ConnectedPlayers())
}

func TestPlayerLoggedInWithoutStoreCloses(t *testing.T) {
	// cmd/gms DB-degraded startup: hello/PING still work, PLAYER_LOGGEDIN
	// cannot load a character and closes.
	s := startChannelWith(t, Config{Count: 1}, nil)

	conn, _, _ := sendLoggedIn(t, s.Channel(1), 31)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("no store -> the session must close")
	}
}

func TestPlayerLoggedInStoreErrorCloses(t *testing.T) {
	fs := &fakeStore{err: context.DeadlineExceeded}
	s := startChannelWith(t, Config{Count: 1}, fs)

	conn, _, _ := sendLoggedIn(t, s.Channel(1), 31)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("a failed character load must close the session")
	}
}

func TestPlayerLoggedInUsesPendingTransfer(t *testing.T) {
	// A pending CharacterTransfer wins over the DB load (Java
	// getPendingCharacter before loadCharFromDB); the store knows nothing
	// about the id, so a DB path would close the session instead.
	fs := &fakeStore{chars: map[int]*database.Character{}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	pending := newPlayer(testChar(41, 7, 100000000), nil)
	cs.Players().RegisterPendingPlayer(pending)

	conn, _, cRecv := sendLoggedIn(t, cs, 41)
	defer conn.Close()

	warp := readTestReply(t, conn, cRecv)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), opOf(warp))
	_ = readTestReply(t, conn, cRecv) // TEMP_STATS_RESET

	assert.Same(t, pending, cs.Players().GetPlayerByID(41))
	require.NotNil(t, pending.Sess, "the picked-up transfer must adopt the new session")
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })
	assert.Same(t, pending, cs.Map(100000000).Players()[0])
}

func TestPendingCharacterIsOneShotAndExpires(t *testing.T) {
	ps := newPlayerStorage(1, nil, nil)
	p := &Player{ID: 5, Name: "x"}
	ps.RegisterPendingPlayer(p)
	assert.Same(t, p, ps.GetPendingCharacter(5))
	assert.Nil(t, ps.GetPendingCharacter(5), "getPendingCharacter removes the entry")

	ps.RegisterPendingPlayer(p)
	ps.mu.Lock()
	ps.pending[5].TransferTime = time.Now().Add(-pendingTransferTTL - time.Second)
	ps.mu.Unlock()
	assert.Nil(t, ps.GetPendingCharacter(5), "transfers older than 40s are dropped")

	ps.RegisterPendingPlayer(p)
	ps.DeregisterPendingPlayer(5)
	assert.Nil(t, ps.GetPendingCharacter(5))
}

func TestDuplicateAccountPlayerEvicted(t *testing.T) {
	// Java ChannelServer.forceRemovePlayerByAccId: logging the same account in
	// on a second channel disconnects the character already online.
	fs := &fakeStore{chars: map[int]*database.Character{
		11: testChar(11, 7, 100000000),
		21: testChar(21, 7, 100000000),
	}}
	s := startChannelWith(t, Config{Count: 2}, fs)

	old := newPlayer(fs.chars[11], nil)
	s.Channel(1).Players().RegisterPlayer(old)

	conn, _, cRecv := sendLoggedIn(t, s.Channel(2), 21)
	defer conn.Close()
	_ = readTestReply(t, conn, cRecv) // WARP_TO_MAP
	_ = readTestReply(t, conn, cRecv) // TEMP_STATS_RESET

	waitFor(t, func() bool { return s.Channel(1).Players().GetPlayerByID(11) == nil })
	newP := s.Channel(2).Players().GetPlayerByID(21)
	require.NotNil(t, newP)
	assert.Same(t, fs.chars[21], newP.Chr)
	_, ok := s.Finder().Find(11)
	assert.False(t, ok, "the evicted character must leave World.Find")
}
