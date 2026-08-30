// Session-level smoke test for the login server (P1 acceptance):
// start Server on an ephemeral port, connect, verify hello bytes exactly
// match the Java getHello layout, send PONG, expect PING back.
package login

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"GMS/internal/config"
	"GMS/internal/crypto"
)

func TestLoginServerHandshakeAndPing(t *testing.T) {
	lg := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	srv := New(config.Login{Host: "127.0.0.1", Port: 0, AutoRegister: true}, lg)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// connect to the real listening port
	addr := srv.acceptor.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// 1. hello packet - Java sends the very first getHello raw (no frame
	// header, no Shanda/AES) because the client cipher is not known yet.
	body := readN(t, conn, 15)
	if len(body) != 15 {
		t.Fatalf("hello body len %d", len(body))
	}
	if ls := binary.LittleEndian.Uint16(body[0:2]); ls != 13 {
		t.Fatalf("hello[0:2] = %d, want 13", ls)
	}
	if v := binary.LittleEndian.Uint16(body[2:4]); v != 79 {
		t.Fatalf("version = %d, want 79", v)
	}
	recvIV := [4]byte{body[6], body[7], body[8], body[9]}
	sendIV := [4]byte{body[10], body[11], body[12], body[13]}
	if recvIV[0] != 70 || recvIV[1] != 114 || recvIV[2] != 12 {
		t.Fatalf("recvIV prefix wrong: % X", recvIV)
	}
	if sendIV[0] != 82 || sendIV[1] != 48 || sendIV[2] != 120 {
		t.Fatalf("sendIV prefix wrong: % X", sendIV)
	}
	if body[14] != 4 {
		t.Fatalf("hello tail = %d, want 4", body[14])
	}

	// 2. send PONG (opcode 0x13) - client-side codecs
	cSend := crypto.NewAESOFB(recvIV, 79)
	cRecv := crypto.NewAESOFB(sendIV, -80)
	pong := []byte{0x13, 0x00}
	sendFrame(t, conn, cSend, pong)

	// 3. expect PING (opcode 0x14) reply, decrypt it
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	rh := readN(t, conn, 4)
	v := binary.BigEndian.Uint32(rh)
	l := v>>16 ^ v&0xFFFF
	n := int(l<<8&0xFF00 | l>>8)
	if n != 2 {
		t.Fatalf("ping body len %d, want 2", n)
	}
	pb := readN(t, conn, n)
	cRecv.Crypt(pb)
	crypto.ShandaDecrypt(pb)
	if pb[0] != 0x14 || pb[1] != 0x00 {
		t.Fatalf("ping = % X, want 14 00", pb)
	}
}

func readN(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read %d/%d: %v", 0, n, err)
	}
	return buf
}

func sendFrame(t *testing.T, c net.Conn, ofb *crypto.AESOFB, body []byte) {
	t.Helper()
	b := append([]byte(nil), body...)
	h := ofb.PacketHeader(len(b))
	crypto.ShandaEncrypt(b)
	ofb.Crypt(b)
	f := append(h[:], b...)
	if _, err := c.Write(f); err != nil {
		t.Fatal(err)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
