package login

// P2.3 e2e: login -> SERVERLIST_REQUEST (world list + end marker) ->
// SERVERSTATUS_REQUEST, plus the NeedsChecking gate (requests from
// not-logged-in sessions are silently dropped, Java MapleClient.isLoggedIn).

import (
	"encoding/binary"
	"log/slog"
	"net"
	"testing"
	"time"

	"GMS/internal/config"
	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
)

// readReply reads one encrypted frame and returns the decrypted body.
func readReply(t *testing.T, c net.Conn, ofb *crypto.AESOFB) []byte {
	t.Helper()
	hdr := readN(t, c, 4)
	v := binary.BigEndian.Uint32(hdr)
	l := v>>16 ^ v&0xFFFF
	n := int(l<<8&0xFF00 | l>>8)
	body := readN(t, c, n)
	ofb.Crypt(body)
	crypto.ShandaDecrypt(body)
	return body
}

// startListServer boots a login server with a fake account and a 2-world
// set (worlds deliberately unsorted: SetWorlds must order them by ID).
func startListServer(t *testing.T) *Server {
	t.Helper()
	fs := newFakeStore()
	fs.accounts["listuser"] = &database.Account{
		ID: 9, Name: "listuser", Password: gSha1("pw"), Gender: 1,
	}
	fs.states[9] = database.AccountState{LoggedIn: 0}

	lg := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	srv := New(config.Login{Host: "127.0.0.1", Port: 0}, lg)
	srv.SetStore(fs)
	srv.SetWorlds(WorldConfig{
		ServerName:   "GMS",
		EventMessage: "",
		UserLimit:    500,
		Worlds:       []World{{ID: 1, State: 0}, {ID: 0, State: 1}},
		ChannelCount: 3,
		Balloons:     []Balloon{{X: 40, Y: 300, Message: "hi"}},
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	return srv
}

// dialLogin performs the raw hello handshake, logs in as listuser, and
// returns the conn plus the client-side send/recv codecs (both stateful).
func dialLogin(t *testing.T, srv *Server) (net.Conn, *crypto.AESOFB, *crypto.AESOFB) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", srv.acceptor.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	hello := readN(t, conn, 15)
	recvIV := [4]byte{hello[6], hello[7], hello[8], hello[9]}
	sendIV := [4]byte{hello[10], hello[11], hello[12], hello[13]}
	cSend := crypto.NewAESOFB(recvIV, 79)
	cRecv := crypto.NewAESOFB(sendIV, -80)

	w := protocol.NewWriter(64)
	w.Short(int(protocol.RecvLOGIN_PASSWORD))
	w.MapleAsciiString("listuser")
	w.MapleAsciiString("pw")
	w.Write([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
	sendFrame(t, conn, cSend, w.Bytes())

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	body := readReply(t, conn, cRecv)
	if len(body) < 3 || body[0] != 0x00 || body[1] != 0x00 || body[2] != 0 {
		t.Fatalf("login reply = % X, want LOGIN_STATUS ok", body)
	}
	return conn, cSend, cRecv
}

// sendOp sends a bare 2-byte opcode packet (SERVERLIST/SERVERSTATUS style).
func sendOp(t *testing.T, c net.Conn, cSend *crypto.AESOFB, op protocol.RecvOp) {
	t.Helper()
	w := protocol.NewWriter(4)
	w.Short(int(op))
	sendFrame(t, c, cSend, w.Bytes())
}

func TestServerListFlow(t *testing.T) {
	srv := startListServer(t)
	defer srv.Stop()
	conn, cSend, cRecv := dialLogin(t, srv)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sendOp(t, conn, cSend, protocol.RecvSERVERLIST_REQUEST)

	// world 0 packet (sorted first despite unsorted input)
	b := readReply(t, conn, cRecv)
	r := protocol.NewReader(b[2:])
	if op := uint16(b[0]) | uint16(b[1])<<8; op != 0x0009 {
		t.Fatalf("opcode = % X, want 09 00 (SERVERLIST)", b[0:2])
	}
	if id := int(r.Byte()); id != 0 {
		t.Fatalf("world id = %d, want 0 (sorted by ID)", id)
	}
	if name := r.MapleAsciiString(); name != "GMS" {
		t.Fatalf("server name = %q", name)
	}
	if state := int(r.Byte()); state != 1 {
		t.Fatalf("world state = %d, want 1", state)
	}
	if msg := r.MapleAsciiString(); msg != "" {
		t.Fatalf("event message = %q, want empty", msg)
	}
	if v1, v2 := int(r.Short()), int(r.Short()); v1 != 100 || v2 != 100 {
		t.Fatalf("short pair = %d,%d, want 100,100", v1, v2)
	}
	lastCh := int(r.Byte())
	if lastCh != 3 {
		t.Fatalf("lastChannel = %d, want 3", lastCh)
	}
	if v := r.Int(); v != 500 {
		t.Fatalf("channel-500 int = %d, want 500", v)
	}
	for i := 1; i <= lastCh; i++ {
		cn := r.MapleAsciiString()
		if want := "GMS-" + itoa(i); cn != want {
			t.Fatalf("channel %d name = %q, want %q", i, cn, want)
		}
		if load := r.Int(); load != 0 {
			t.Fatalf("channel %d load = %d, want 0", i, load)
		}
		if wid := int(r.Byte()); wid != 0 {
			t.Fatalf("channel %d world byte = %d, want 0", i, wid)
		}
		if ch := int(r.Short()); ch != i-1 {
			t.Fatalf("channel %d id = %d, want %d", i, ch, i-1)
		}
	}
	if n := int(r.Short()); n != 1 {
		t.Fatalf("balloon count = %d, want 1", n)
	}
	if x, y := int(r.Short()), int(r.Short()); x != 40 || y != 300 {
		t.Fatalf("balloon pos = %d,%d, want 40,300", x, y)
	}
	if msg := r.MapleAsciiString(); msg != "hi" {
		t.Fatalf("balloon message = %q", msg)
	}
	if r.Err != nil || r.Len() != 0 {
		t.Fatalf("trailing bytes in world packet: len=%d err=%v", r.Len(), r.Err)
	}

	// world 1 packet
	b2 := readReply(t, conn, cRecv)
	r2 := protocol.NewReader(b2[2:])
	if id := int(r2.Byte()); id != 1 {
		t.Fatalf("second world id = %d, want 1", id)
	}
	r2.MapleAsciiString() // server name (shared)
	if state := int(r2.Byte()); state != 0 {
		t.Fatalf("world 1 state = %d, want 0", state)
	}

	// end of list: opcode 09 + byte 255
	bEnd := readReply(t, conn, cRecv)
	if len(bEnd) != 3 || bEnd[0] != 0x09 || bEnd[1] != 0x00 || bEnd[2] != 0xFF {
		t.Fatalf("end-of-list = % X, want 09 00 FF", bEnd)
	}

	// SERVERSTATUS_REQUEST -> 0 users vs limit 500 -> status 0
	sendOp(t, conn, cSend, protocol.RecvSERVERSTATUS_REQUEST)
	bStat := readReply(t, conn, cRecv)
	if len(bStat) != 4 || bStat[0] != 0x06 || bStat[1] != 0x00 {
		t.Fatalf("status = % X, want 06 00 ..", bStat)
	}
	if st := int(bStat[2]) | int(bStat[3])<<8; st != 0 {
		t.Fatalf("status level = %d, want 0 (normal)", st)
	}
}

func TestServerListRequiresLogin(t *testing.T) {
	srv := startListServer(t)
	defer srv.Stop()

	conn, err := net.DialTimeout("tcp", srv.acceptor.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	hello := readN(t, conn, 15)
	recvIV := [4]byte{hello[6], hello[7], hello[8], hello[9]}
	cSend := crypto.NewAESOFB(recvIV, 79)

	// pre-login SERVERLIST_REQUEST: Java NeedsChecking gate -> no reply.
	sendOp(t, conn, cSend, protocol.RecvSERVERLIST_REQUEST)
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	one := make([]byte, 1)
	if n, err := conn.Read(one); err == nil {
		t.Fatalf("server replied %d byte(s) to unauthenticated SERVERLIST (want silent drop)", n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
