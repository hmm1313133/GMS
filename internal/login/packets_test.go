package login

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestLoginFailedPacket ports Java LoginPacket.getLoginFailed byte layout:
// short LOGIN_STATUS (0x0000) + int reason + short 0.
func TestLoginFailedPacket(t *testing.T) {
	for _, reason := range []int{0, 1, 3, 4, 5, 7} {
		b := LoginFailedPacket(reason)
		if len(b) != 8 {
			t.Fatalf("reason %d: len = %d, want 8", reason, len(b))
		}
		if b[0] != 0x00 || b[1] != 0x00 {
			t.Fatalf("reason %d: opcode = % X, want 00 00 (LOGIN_STATUS)", reason, b[0:2])
		}
		want := []byte{byte(reason), 0, 0, 0, 0, 0}
		if hex.EncodeToString(b[2:]) != hex.EncodeToString(want) {
			t.Fatalf("reason %d: body = % X, want % X", reason, b[2:], want)
		}
	}
}

// TestAuthSuccessPacket ports Java LoginPacket.getAuthSuccessRequest byte
// layout: short LOGIN_STATUS + 0 + int accID + byte gender + short gm +
// str name + fixed 15-byte tail + int 0 + long 0 + str accID + str name + 1.
func TestAuthSuccessPacket(t *testing.T) {
	b := AuthSuccessPacket(232, 1, true, "goldenadmin")
	if b[0] != 0x00 || b[1] != 0x00 {
		t.Fatalf("opcode = % X, want 00 00 (LOGIN_STATUS)", b[0:2])
	}
	if b[2] != 0 {
		t.Fatalf("success flag = %d, want 0", b[2])
	}
	// accID little-endian at offset 3
	if v := int(b[3]) | int(b[4])<<8 | int(b[5])<<16 | int(b[6])<<24; v != 232 {
		t.Fatalf("accID = %d, want 232", v)
	}
	if b[7] != 1 {
		t.Fatalf("gender = %d, want 1", b[7])
	}
	if b[8] != 1 || b[9] != 0 {
		t.Fatalf("gm short = % X, want 01 00", b[8:10])
	}
	// name string at offset 10
	nl := int(b[10]) | int(b[11])<<8
	if nl != len("goldenadmin") || string(b[12:12+nl]) != "goldenadmin" {
		t.Fatalf("name field = %q", b[12:12+nl])
	}
	off := 12 + nl
	// fixed tail
	wantTail := "0000000301000000e2eda37afac901"
	if got := hex.EncodeToString(b[off : off+15]); got != wantTail {
		t.Fatalf("auth tail = %s, want %s", got, wantTail)
	}
	off += 15
	// int 0 + long 0
	if hex.EncodeToString(b[off:off+12]) != strings.Repeat("00", 12) {
		t.Fatalf("zero int+long = % X", b[off:off+12])
	}
	off += 12
	// str(accID)
	kl := int(b[off]) | int(b[off+1])<<8
	if kl != 3 || string(b[off+2:off+2+kl]) != "232" {
		t.Fatalf("accID string field wrong: % X", b[off:off+2+kl])
	}
	off += 2 + kl
	// str(name) again
	nl2 := int(b[off]) | int(b[off+1])<<8
	if nl2 != len("goldenadmin") || string(b[off+2:off+2+nl2]) != "goldenadmin" {
		t.Fatalf("name string field wrong: % X", b[off:off+2+nl2])
	}
	off += 2 + nl2
	if b[off] != 1 {
		t.Fatalf("trailing byte = %d, want 1", b[off])
	}
	if off+1 != len(b) {
		t.Fatalf("packet len = %d, want %d", len(b), off+1)
	}
}

// TestGenderNeededPacket ports Java LoginPacket.getGenderNeeded:
// short CHOOSE_GENDER (0x0004) + str accountName.
func TestGenderNeededPacket(t *testing.T) {
	b := GenderNeededPacket("newbie")
	if b[0] != 0x04 || b[1] != 0x00 {
		t.Fatalf("opcode = % X, want 04 00 (CHOOSE_GENDER)", b[0:2])
	}
	nl := int(b[2]) | int(b[3])<<8
	if nl != len("newbie") || string(b[4:4+nl]) != "newbie" {
		t.Fatalf("name field wrong: % X", b)
	}
	if len(b) != 4+nl {
		t.Fatalf("len = %d, want %d", len(b), 4+nl)
	}
}

// TestServerListPacket ports Java LoginPacket.getServerList byte layout:
// short SERVERLIST + byte id + str name + byte state + str event +
// short 100 + short 100 + byte lastChannel + int 500 + per channel
// (str "name-i" + int load + byte id + short i-1) + short balloon count +
// per balloon (short x + short y + str message).
func TestServerListPacket(t *testing.T) {
	load := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
	b := ServerListPacket(0, "GMS", 1, "", load, []Balloon{{X: 40, Y: 300, Message: "hi"}})

	if b[0] != 0x09 || b[1] != 0x00 {
		t.Fatalf("opcode = % X, want 09 00 (SERVERLIST)", b[0:2])
	}
	if b[2] != 0 {
		t.Fatalf("world id = %d, want 0", b[2])
	}
	off := 3
	if nl := int(b[off]) | int(b[off+1])<<8; nl != 3 || string(b[off+2:off+5]) != "GMS" {
		t.Fatalf("server name field wrong: % X", b[off:off+5])
	}
	off += 5
	if b[off] != 1 {
		t.Fatalf("state = %d, want 1", b[off])
	}
	off++
	if el := int(b[off]) | int(b[off+1])<<8; el != 0 {
		t.Fatalf("event message length = %d, want 0", el)
	}
	off += 2
	if s := int(b[off]) | int(b[off+1])<<8; s != 100 {
		t.Fatalf("first short = %d, want 100", s)
	}
	off += 2
	if s := int(b[off]) | int(b[off+1])<<8; s != 100 {
		t.Fatalf("second short = %d, want 100", s)
	}
	off += 2
	if lc := int(b[off]); lc != 5 {
		t.Fatalf("lastChannel = %d, want 5", lc)
	}
	off++
	if v := int(b[off]) | int(b[off+1])<<8 | int(b[off+2])<<16 | int(b[off+3])<<24; v != 500 {
		t.Fatalf("int 500 = %d", v)
	}
	off += 4
	for i := 1; i <= 5; i++ {
		want := "GMS-" + string(rune('0'+i))
		nl := int(b[off]) | int(b[off+1])<<8
		if nl != 5 || string(b[off+2:off+7]) != want {
			t.Fatalf("channel %d name wrong: %q, want %q", i, b[off+2:off+7], want)
		}
		off += 7
		if v := int(b[off]) | int(b[off+1])<<8 | int(b[off+2])<<16 | int(b[off+3])<<24; v != 0 {
			t.Fatalf("channel %d load = %d, want 0", i, v)
		}
		off += 4
		if b[off] != 0 {
			t.Fatalf("channel %d world byte = %d, want 0", i, b[off])
		}
		off++
		if ch := int(b[off]) | int(b[off+1])<<8; ch != i-1 {
			t.Fatalf("channel %d id = %d, want %d", i, ch, i-1)
		}
		off += 2
	}
	if n := int(b[off]) | int(b[off+1])<<8; n != 1 {
		t.Fatalf("balloon count = %d, want 1", n)
	}
	off += 2
	if x := int(b[off]) | int(b[off+1])<<8; x != 40 {
		t.Fatalf("balloon x = %d, want 40", x)
	}
	off += 2
	if y := int(b[off]) | int(b[off+1])<<8; y != 300 {
		t.Fatalf("balloon y = %d, want 300", y)
	}
	off += 2
	if ml := int(b[off]) | int(b[off+1])<<8; ml != 2 || string(b[off+2:off+4]) != "hi" {
		t.Fatalf("balloon message wrong: % X", b[off:off+4])
	}
	off += 4
	if off != len(b) {
		t.Fatalf("packet len = %d, walked %d", len(b), off)
	}
}

// TestServerListPacketSparse: missing channels report load 1200 and
// lastChannel follows the highest key <= 30 (Java loop quirk).
func TestServerListPacketSparse(t *testing.T) {
	b := ServerListPacket(2, "W", 0, "evt", map[int]int{2: 50}, nil)

	off := 2 + 1 // opcode + id
	off += 2 + 1 // "W" (len 1)
	off++        // state
	off += 2 + 3 // "evt"
	off += 4     // 100,100
	if lc := int(b[off]); lc != 2 {
		t.Fatalf("lastChannel = %d, want 2 (highest key)", lc)
	}
	off++
	off += 4 // int 500
	// channel 1: missing -> 1200
	nl := int(b[off]) | int(b[off+1])<<8
	if string(b[off+2:off+2+nl]) != "W-1" {
		t.Fatalf("channel 1 name = %q", b[off+2:off+2+nl])
	}
	off += 2 + nl
	if v := int(b[off]) | int(b[off+1])<<8 | int(b[off+2])<<16 | int(b[off+3])<<24; v != 1200 {
		t.Fatalf("missing channel load = %d, want 1200", v)
	}
	off += 4 + 1 + 2
	// channel 2: load 50
	nl = int(b[off]) | int(b[off+1])<<8
	if string(b[off+2:off+2+nl]) != "W-2" {
		t.Fatalf("channel 2 name = %q", b[off+2:off+2+nl])
	}
	off += 2 + nl
	if v := int(b[off]) | int(b[off+1])<<8 | int(b[off+2])<<16 | int(b[off+3])<<24; v != 50 {
		t.Fatalf("channel 2 load = %d, want 50", v)
	}
	off += 4
	if b[off] != 2 { // world id byte repeats per channel
		t.Fatalf("channel 2 world byte = %d, want 2", b[off])
	}
	off++
	if ch := int(b[off]) | int(b[off+1])<<8; ch != 1 {
		t.Fatalf("channel 2 id = %d, want 1", ch)
	}
	off += 2
	if n := int(b[off]) | int(b[off+1])<<8; n != 0 {
		t.Fatalf("balloon count = %d, want 0", n)
	}
	off += 2
	if off != len(b) {
		t.Fatalf("packet len = %d, walked %d", len(b), off)
	}
}

// TestEndOfServerListPacket: short SERVERLIST + byte 255 (Java marker).
func TestEndOfServerListPacket(t *testing.T) {
	b := EndOfServerListPacket()
	if len(b) != 3 || b[0] != 0x09 || b[1] != 0x00 || b[2] != 0xFF {
		t.Fatalf("end packet = % X, want 09 00 FF", b)
	}
}

// TestServerStatusPacket: short SERVERSTATUS + short status.
func TestServerStatusPacket(t *testing.T) {
	for _, st := range []int{0, 1, 2} {
		b := ServerStatusPacket(st)
		if len(b) != 4 || b[0] != 0x06 || b[1] != 0x00 {
			t.Fatalf("status %d opcode = % X, want 06 00", st, b[0:2])
		}
		if v := int(b[2]) | int(b[3])<<8; v != st {
			t.Fatalf("status %d body = %d", st, v)
		}
	}
}
