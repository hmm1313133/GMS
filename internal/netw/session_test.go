package netw

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"GMS/internal/crypto"
)

// pipeConn is a synchronous in-memory net.Conn pair for session tests.
type pipeConn struct {
	c1, c2 net.Conn
}

func newPipe(t *testing.T) (client, server net.Conn) {
	t.Helper()
	c1, c2 := net.Pipe()
	return c1, c2
}

type collectHandler struct {
	opened   int
	closed   int
	packets  [][]byte
	ready    chan struct{}
	gotPkt   chan struct{}
}

func newCollect() *collectHandler {
	return &collectHandler{ready: make(chan struct{}), gotPkt: make(chan struct{}, 8)}
}

func (h *collectHandler) OnOpen(s *Session)        { h.opened++; close(h.ready) }
func (h *collectHandler) OnClose(s *Session)       { h.closed++ }
func (h *collectHandler) OnPacket(s *Session, b []byte) {
	h.packets = append(h.packets, append([]byte(nil), b...))
	select {
	case h.gotPkt <- struct{}{}:
	default:
	}
}

func waitFor(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

// TestSessionEchoRoundTrip drives a full session with real crypto:
// server writes a packet, client-side manually decrypts; client sends one
// back, handler must receive the plaintext.
func TestSessionEchoRoundTrip(t *testing.T) {
	clientConn, serverConn := newPipe(t)

	h := newCollect()
	ivSend := [4]byte{82, 48, 120, 55}
	ivRecv := [4]byte{70, 114, 12, 199}

	sendH, recvH := NewCryptoPair(ivSend, ivRecv)
	s := NewSession(serverConn, h, sendH, recvH)
	go s.readLoop(nil) // standalone session (no acceptor)

	// server greeting packet
	hello := buildHello(79, ivSend, ivRecv)
	s.Write(hello)

	// ---- client side ----
	cSend := crypto.NewAESOFB(ivRecv, 79)   // client send uses recv IV
	cRecv := crypto.NewAESOFB(ivSend, -80)  // client recv uses send IV

	// read hello frame from pipe (server wrote 18-byte hello)
	frame := readExact(t, clientConn, 4+len(hello))
	if !bytes.Equal(frame[4:], hello) {
		// decrypt check: header + body
		body := append([]byte(nil), frame[4:]...)
		cRecv.Crypt(body)
		crypto.ShandaDecrypt(body)
		if !bytes.Equal(body, hello) {
			t.Fatalf("hello body mismatch: % X vs % X", body, hello)
		}
	}

	// client -> server PONG packet: [opcode 0x13][payload]
	pong := []byte{0x13, 0x00, 'P', 'O', 'N', 'G'}
	cbody := append([]byte(nil), pong...)
	hdr := cSend.PacketHeader(len(cbody))
	crypto.ShandaEncrypt(cbody)
	cSend.Crypt(cbody)
	frame2 := append(hdr[:], cbody...)
	if _, err := clientConn.Write(frame2); err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.gotPkt, "packet delivery")

	if len(h.packets) != 1 {
		t.Fatalf("packets received: %d", len(h.packets))
	}
	if !bytes.Equal(h.packets[0], pong) {
		t.Fatalf("pong mismatch: % X vs % X", h.packets[0], pong)
	}

	// server -> client another packet, verify client can decode
	msg2 := []byte{0x14, 0x00, 1, 2, 3}
	s.Write(msg2)
	frame3 := readExact(t, clientConn, 4+len(msg2))
	body3 := append([]byte(nil), frame3[4:]...)
	cRecv.Crypt(body3)
	crypto.ShandaDecrypt(body3)
	if !bytes.Equal(body3, msg2) {
		t.Fatalf("msg2 mismatch: % X", body3)
	}

	s.Close("done")
	clientConn.Close()
}

// buildHello mirrors Java LoginPacket.getHello(79, ivSend, ivRecv):
// short 13 (len?), short version, 2 zeros, RECV iv, SEND iv, byte 4.
func buildHello(version int, sendIV, recvIV [4]byte) []byte {
	b := make([]byte, 0, 15)
	b = append(b, 0x0D, 0x00)
	b = binary.LittleEndian.AppendUint16(b, uint16(version))
	b = append(b, 0, 0)
	b = append(b, recvIV[:]...)
	b = append(b, sendIV[:]...)
	b = append(b, 4)
	return b
}

func readExact(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	buf := make([]byte, 0, n)
	tmp := make([]byte, n)
	deadline := time.Now().Add(3 * time.Second)
	c.SetReadDeadline(deadline)
	for len(buf) < n {
		k, err := c.Read(tmp[:n-len(buf)])
		if err != nil {
			t.Fatalf("read: %v (have %d/%d)", err, len(buf), n)
		}
		buf = append(buf, tmp[:k]...)
	}
	return buf
}

func TestPacketLengthHeader(t *testing.T) {
	// build header for len 100 and decode back
	c := crypto.NewAESOFB([4]byte{82, 48, 120, 55}, -80)
	h := c.PacketHeader(100)
	got := packetLength(h[:])
	if got != 100 {
		t.Fatalf("packetLength = %d, want 100", got)
	}
}

func TestAcceptorLifecycle(t *testing.T) {
	h := newCollect()
	a := &Acceptor{Handler: h}
	a.NewSession = func(conn net.Conn, hh Handler) *Session {
		sendH, recvH := NewCryptoPair([4]byte{82, 48, 120, 55}, [4]byte{70, 114, 12, 199})
		return NewSession(conn, hh, sendH, recvH)
	}
	if err := a.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	c, err := net.DialTimeout("tcp", a.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, h.ready, "OnOpen")
	if a.SessionCount() != 1 {
		t.Fatalf("session count %d", a.SessionCount())
	}
	c.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && a.SessionCount() != 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if a.SessionCount() != 0 {
		t.Fatalf("session not removed after close")
	}
}
