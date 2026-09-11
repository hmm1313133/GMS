package movement

// P4.4 tests: MovementParse.parseMovement / StaticLifeMovement.serialize round
// trip, plus the quirks the Java source preserves (discarded duration for the
// teleport family, NewFh = the constructor's fifth argument).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/protocol"
)

// encode builds a MOVE_PLAYER movement list body (count byte + fragments).
func encode(t *testing.T, moves ...Fragment) []byte {
	t.Helper()
	w := protocol.NewWriter(64)
	SerializeMovementList(w, moves)
	return w.Bytes()
}

func TestParseAndSerializeRoundTrip(t *testing.T) {
	// One fragment of every command family the parser understands.
	moves := []Fragment{
		{Type: 0, Pos: &Point{10, 20}, Wobble: &Point{1, 2}, Unk: 7, NewState: 4, Duration: 100, NewFh: 7},
		{Type: 5, Pos: &Point{11, 21}, Wobble: &Point{3, 4}, Unk: 8, NewState: 5, Duration: 101, NewFh: 8},
		{Type: 15, Pos: &Point{12, 22}, Wobble: &Point{5, 6}, Unk: 9, Fh: 33, NewState: 6, Duration: 102, NewFh: 9},
		{Type: 17, Pos: &Point{13, 23}, Wobble: &Point{7, 8}, Unk: 10, NewState: 7, Duration: 103, NewFh: 10},
		{Type: 1, Wobble: &Point{-1, -2}, NewState: 1, Duration: 50},
		{Type: 19, Wobble: &Point{2, 3}, NewState: 2, Duration: 51},
		{Type: 22, Wobble: &Point{4, 5}, NewState: 3, Duration: 52},
		{Type: 3, Pos: &Point{100, 200}, Unk: 11, NewState: 8},
		{Type: 11, Pos: &Point{101, 201}, Unk: 12, NewState: 9},
		{Type: 10, Wui: 42},
		{Type: 14, Wobble: &Point{6, 7}, Fh: 44, NewState: 10, Duration: 104},
		{Type: 99, NewState: 11, Duration: 105}, // default family
	}

	b := encode(t, moves...)
	got, ok := Parse(protocol.NewReader(b), 1)
	require.True(t, ok)
	require.Len(t, got, len(moves))
	assert.Equal(t, moves, got, "a well-formed list must round-trip field for field")

	// And re-serialising the parsed list yields the same bytes.
	out := protocol.NewWriter(64)
	SerializeMovementList(out, got)
	assert.Equal(t, b, out.Bytes())
}

func TestParseDiscardsDurationForTeleportFamily(t *testing.T) {
	// Java reads the duration for 3/4/7/8/9/11 and then builds the fragment
	// with duration 0 - the re-broadcast therefore writes 0. Assert both the
	// parse and the re-serialised bytes.
	b := encode(t, Fragment{Type: 3, Pos: &Point{1, 2}, Unk: 3, NewState: 4, Duration: 999})
	moves, ok := Parse(protocol.NewReader(b), 1)
	require.True(t, ok)
	require.Len(t, moves, 1)
	assert.Equal(t, 0, moves[0].Duration, "duration must be discarded")

	w := protocol.NewWriter(16)
	SerializeMovementList(w, moves)
	// count(1) + type(3) + pos(1,2) + unk(3) + newstate(4) + duration(discarded -> 0)
	assert.Equal(t, []byte{1, 3, 1, 0, 2, 0, 3, 0, 4, 0, 0}, w.Bytes())
}

func TestParseNewFhQuirk(t *testing.T) {
	// For 0/5/15/17 the fifth constructor argument is `unk`, so NewFh carries
	// the unknown short rather than the foothold.
	moves, ok := Parse(protocol.NewReader(encode(t,
		Fragment{Type: 0, Pos: &Point{1, 2}, Wobble: &Point{3, 4}, Unk: 0x1234, NewState: 5, Duration: 6},
	)), 1)
	require.True(t, ok)
	assert.Equal(t, 0x1234, moves[0].NewFh)

	// Every other family passes 0.
	moves, ok = Parse(protocol.NewReader(encode(t,
		Fragment{Type: 1, Wobble: &Point{3, 4}, NewState: 5, Duration: 6},
	)), 1)
	require.True(t, ok)
	assert.Zero(t, moves[0].NewFh)
}

func TestParseRejectsMalformedLists(t *testing.T) {
	// Declared count larger than the data: Java returns null.
	_, ok := Parse(protocol.NewReader([]byte{2, 0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6}), 1)
	assert.False(t, ok, "a truncated list must fail")

	// A count >= 128 is a negative byte in Java, so the loop never runs and the
	// size check fails.
	_, ok = Parse(protocol.NewReader([]byte{0x80}), 1)
	assert.False(t, ok)

	// Zero commands is a valid (empty) list in Java.
	moves, ok := Parse(protocol.NewReader([]byte{0}), 1)
	assert.True(t, ok)
	assert.Empty(t, moves)
}

// recordingTarget captures the Target callbacks.
type recordingTarget struct {
	pos    []Point
	fh     []int
	stance []int
}

func (t *recordingTarget) SetPosition(x, y int16) { t.pos = append(t.pos, Point{x, y}) }
func (t *recordingTarget) SetFh(fh int)           { t.fh = append(t.fh, fh) }
func (t *recordingTarget) SetStance(s int)        { t.stance = append(t.stance, s) }

func TestUpdatePositionAppliesLastFragment(t *testing.T) {
	// Java loops the whole list: foothold/stance come from the last fragment,
	// the position from the last fragment that carries one.
	moves := []Fragment{
		{Type: 1, Wobble: &Point{1, 1}, NewState: 2, NewFh: 0},
		{Type: 0, Pos: &Point{100, 200}, Wobble: &Point{0, 0}, Unk: 3, NewState: 4, NewFh: 3},
		{Type: 1, Wobble: &Point{2, 2}, NewState: 9, NewFh: 0},
	}
	tgt := &recordingTarget{}
	UpdatePosition(moves, tgt, 0)

	require.Equal(t, []Point{{100, 200}}, tgt.pos)
	assert.Equal(t, []int{0, 3, 0}, tgt.fh)
	assert.Equal(t, []int{2, 4, 9}, tgt.stance)
}
