package channel

// P4.4 e2e over the wire: MOVE_PLAYER (0x24) is parsed, broadcast to the other
// players on the map (never back to the mover) and applied to the mover's
// position/stance.

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/movement"
	"GMS/internal/protocol"
)

// moveBody builds a MOVE_PLAYER body: the v079 client prefix (skipped by the
// handler) followed by the movement list.
func moveBody(moves ...movement.Fragment) []byte {
	body := make([]byte, moveHeaderSkip)
	w := protocol.NewWriter(64)
	movement.SerializeMovementList(w, moves)
	return append(body, w.Bytes()...)
}

func sendMove(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, body []byte) {
	t.Helper()
	w := protocol.NewWriter(len(body) + 2)
	w.Short(int(protocol.RecvMOVE_PLAYER))
	w.Write(body)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

// drain reads and discards n replies.
func drain(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		readTestReply(t, conn, ofb)
	}
}

func TestMovePlayerBroadcastAndPosition(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: testChar(31, 7, 100000000),
		32: testChar(32, 8, 100000000),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	// A enters: WARP_TO_MAP, TEMP_STATS_RESET, own spawn.
	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 1 })

	// B enters the same map: WARP_TO_MAP, TEMP_STATS_RESET, spawn(A), spawn(B).
	connB, sendB, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1) // A is told about B
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 2 })

	// A moves to (300, -150) with stance 5.
	sendMove(t, connA, sendA, moveBody(movement.Fragment{
		Type: 0, Pos: &movement.Point{X: 300, Y: -150},
		Wobble: &movement.Point{X: 0, Y: 0}, Unk: 0, NewState: 5, Duration: 60,
	}))

	// B sees the move; A (the source) does not.
	move := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendMOVE_PLAYER), opOf(move))
	mr := protocol.NewReader(move[2:])
	assert.Equal(t, int32(31), mr.Int(), "mover cid")
	assert.Equal(t, int32(0), mr.Int(), "Java writes a literal 0 where startPos was")
	assert.Equal(t, 1, int(mr.Byte()), "movement count")
	assert.Equal(t, 0, int(mr.Byte()), "command type")
	x, y := mr.Pos()
	assert.Equal(t, int16(300), x)
	assert.Equal(t, int16(-150), y)
	wx, wy := mr.Pos()
	assert.Equal(t, int16(0), wx)
	assert.Equal(t, int16(0), wy)
	assert.Equal(t, 0, int(mr.Short()), "unk")
	assert.Equal(t, 5, int(mr.Byte()), "newstate")
	assert.Equal(t, 60, int(mr.Short()), "duration")
	assert.NoError(t, mr.Err)
	assert.Zero(t, mr.Len(), "the movement list is relayed verbatim")

	// The mover's position/stance were updated (Java updatePosition).
	pA := cs.Players().GetPlayerByID(31)
	require.NotNil(t, pA)
	waitFor(t, func() bool { return pA.Pos == movement.Point{X: 300, Y: -150} })
	assert.Equal(t, 5, pA.Stance)
	assert.Equal(t, movement.Point{X: 300, Y: -150}, pA.OldPos)

	// B moves too - the same broadcast path covers it.
	sendMove(t, connB, sendB, moveBody(movement.Fragment{
		Type: 1, Wobble: &movement.Point{X: -8, Y: 0}, NewState: 2, Duration: 30,
	}))
	moveB := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendMOVE_PLAYER), opOf(moveB))
	br := protocol.NewReader(moveB[2:])
	assert.Equal(t, int32(32), br.Int())

	// A malformed movement list is dropped without closing the session.
	bad := make([]byte, moveHeaderSkip+1)
	bad[moveHeaderSkip] = 5 // declares 5 commands but carries none
	sendMove(t, connA, sendA, bad)
	connA.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := connA.Read(make([]byte, 16)); err == nil {
		t.Fatal("a malformed movement must not produce a reply")
	}
	assert.Equal(t, 2, cs.Map(100000000).PlayerCount(), "the session stays on the map")
}
