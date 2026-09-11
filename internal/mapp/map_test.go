package mapp

// P4.3 tests: the spawn/despawn broadcasts of MapleMap.addPlayer/removePlayer
// and the source-excluding broadcastMessage overload.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSink records every packet it is handed.
type fakeSink struct{ got [][]byte }

func (f *fakeSink) SendPacket(p []byte) {
	f.got = append(f.got, append([]byte(nil), p...))
}

// fakePlayer is a minimal Player: spawn data is {'S', id}, despawn {'D', id}.
type fakePlayer struct {
	id   int
	x, y int16
	sink *fakeSink
}

func (p *fakePlayer) ObjectID() int              { return p.id }
func (p *fakePlayer) SendPacket(b []byte)        { p.sink.SendPacket(b) }
func (p *fakePlayer) SendSpawnData(s PacketSink) { s.SendPacket([]byte{'S', byte(p.id)}) }
func (p *fakePlayer) DespawnData() []byte        { return []byte{'D', byte(p.id)} }
func (p *fakePlayer) Position() (int16, int16)   { return p.x, p.y }

func newFakePlayer(id int) *fakePlayer {
	return &fakePlayer{id: id, sink: &fakeSink{}}
}

func TestAddPlayerSpawnBroadcast(t *testing.T) {
	m := New(100000000)
	a := newFakePlayer(1)

	// First player: nobody else to notify, but the client still gets its own
	// spawn (Java MapleMap.addPlayer -> sendPacket(spawnPlayerMapobject)).
	m.AddPlayer(a)
	require.Equal(t, [][]byte{{'S', 1}}, a.sink.got)

	// Second player: a learns about b; b learns about a and gets its own spawn.
	b := newFakePlayer(2)
	m.AddPlayer(b)
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}}, a.sink.got, "existing player sees the newcomer")
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}}, b.sink.got, "newcomer sees the others then itself")

	assert.Equal(t, 2, m.PlayerCount())
}

func TestRemovePlayerDespawnBroadcast(t *testing.T) {
	m := New(100000000)
	a, b := newFakePlayer(1), newFakePlayer(2)
	m.AddPlayer(a)
	m.AddPlayer(b)

	assert.True(t, m.RemovePlayer(b))
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}, {'D', 2}}, a.sink.got, "remaining player drops the sprite")
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}}, b.sink.got, "the leaving player is not told about its own despawn")

	// Removing a player that already left must not rebroadcast.
	assert.False(t, m.RemovePlayer(b))
	assert.Equal(t, 1, m.PlayerCount())
}

func TestBroadcastExcludesSource(t *testing.T) {
	m := New(100000000)
	a, b := newFakePlayer(1), newFakePlayer(2)
	m.AddPlayer(a)
	m.AddPlayer(b)

	m.Broadcast([]byte("X"), a)
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}}, a.sink.got, "the source is skipped")
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}, {'X'}}, b.sink.got)

	m.Broadcast([]byte("Y"), nil)
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}, {'Y'}}, a.sink.got, "nil source reaches everyone")
	assert.Equal(t, [][]byte{{'S', 1}, {'S', 2}, {'X'}, {'Y'}}, b.sink.got)
}

func TestBroadcastRangedViewRange(t *testing.T) {
	// P4.5: MapleMap.broadcastMessage(packet, rangedFrom) filters on
	// GameConstants.maxViewRangeSq (10000^2) and reaches the source too.
	m := New(100000000)
	a, b, c := newFakePlayer(1), newFakePlayer(2), newFakePlayer(3)
	b.x, b.y = 9999, 0  // inside the 10000 radius
	c.x, c.y = 10001, 0 // just outside
	m.AddPlayer(a)
	m.AddPlayer(b)
	m.AddPlayer(c)

	// Drop the spawn traffic of the setup, so only the ranged packet shows up.
	for _, p := range []*fakePlayer{a, b, c} {
		p.sink.got = nil
	}
	m.BroadcastRanged([]byte("C"), 0, 0)
	assert.Equal(t, [][]byte{{'C'}}, a.sink.got, "the source is included (Java source=null)")
	assert.Equal(t, [][]byte{{'C'}}, b.sink.got, "9999 units away is in range")
	assert.Empty(t, c.sink.got, "10001 units away is out of range")

	// The comparison is on the *squared* distance: 8000/6000 is 1e8 exactly,
	// i.e. on the limit and therefore still in range.
	d := newFakePlayer(4)
	d.x, d.y = 8000, 6000
	m.AddPlayer(d)
	d.sink.got = nil
	m.BroadcastRanged([]byte("D"), 0, 0)
	assert.Equal(t, [][]byte{{'D'}}, d.sink.got, "8000,6000 is exactly maxViewRangeSq")
}
