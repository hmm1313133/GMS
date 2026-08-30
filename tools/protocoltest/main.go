// protocoltest is the P1 self-test client: dials a GMS login port, performs
// the v079 handshake, then runs a scripted send/receive exchange and dumps
// hex. It doubles as a live-server probe tool (Java ServerProperties parity).
//
// Usage:
//
//	go run ./tools/protocoltest -addr 127.0.0.1:8484
//	go run ./tools/protocoltest -addr 127.0.0.1:8484 -hex "1300504F4E47"
//	go run ./tools/protocoltest -addr 127.0.0.1:8484 -login testgo:test123
//	go run ./tools/protocoltest -addr 127.0.0.1:8484 -login testgo:test123 -serverlist -status
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"GMS/internal/crypto"
	"GMS/internal/protocol"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8484", "server address")
	hexSend := flag.String("hex", "", "plaintext packet body to send (hex), after handshake")
	loginCred := flag.String("login", "", "user:pass - send a LOGIN_PASSWORD exchange and decode LOGIN_STATUS")
	serverList := flag.Bool("serverlist", false, "send SERVERLIST_REQUEST and decode world entries (requires -login)")
	statusReq := flag.Bool("status", false, "send SERVERSTATUS_REQUEST and decode the level (requires -login)")
	charList := flag.Bool("charlist", false, "send CHARLIST_REQUEST and decode the character entries (requires -login)")
	wait := flag.Duration("wait", 5*time.Second, "how long to listen for replies")
	flag.Parse()

	conn, err := net.DialTimeout("tcp", *addr, 3*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("connected to %s\n", *addr)

	// ---- 1. read handshake (raw 15-byte hello, no frame/encryption) ----
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	body := readN(conn, 15)
	if body == nil {
		fmt.Fprintln(os.Stderr, "no handshake received")
		os.Exit(1)
	}
	version := int16(uint16(body[2]) | uint16(body[3])<<8)
	// Java LoginPacket.getHello(79, sendIv, recvIv) order: [len2][ver2][0 0][recvIv4][sendIv4][4]
	recvIV := [4]byte{body[6], body[7], body[8], body[9]}
	sendIV := [4]byte{body[10], body[11], body[12], body[13]}
	fmt.Printf("hello: version=%d sendIV=% X recvIV=% X\n", version, sendIV, recvIV)

	// client-side codecs: client send uses recvIV+79, client recv uses sendIV+-80
	cSend := crypto.NewAESOFB(recvIV, 79)
	cRecv := crypto.NewAESOFB(sendIV, -80)

	// ---- 2. optional scripted packet ----
	if *loginCred != "" {
		i := strings.Index(*loginCred, ":")
		if i < 0 {
			fmt.Fprintf(os.Stderr, "-login: expect user:pass\n")
			os.Exit(1)
		}
		user, pass := (*loginCred)[:i], (*loginCred)[i+1:]
		w := protocol.NewWriter(64)
		w.Short(int(protocol.RecvLOGIN_PASSWORD))
		w.MapleAsciiString(user)
		w.MapleAsciiString(pass)
		w.Write([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
		b := w.Bytes()
		sendPacket(conn, cSend, b)
		fmt.Printf("sent LOGIN_PASSWORD (%s:%s): % X\n", user, pass, b)
	}
	if *hexSend != "" {
		raw, err := hex.DecodeString(*hexSend)
		if err != nil {
			fmt.Fprintf(os.Stderr, "-hex: %v\n", err)
			os.Exit(1)
		}
		sendPacket(conn, cSend, raw)
		fmt.Printf("sent: % X\n", raw)
	}
	if *serverList {
		sendOpPacket(conn, cSend, protocol.RecvSERVERLIST_REQUEST)
		fmt.Println("sent SERVERLIST_REQUEST (0x0002)")
	}
	if *statusReq {
		sendOpPacket(conn, cSend, protocol.RecvSERVERSTATUS_REQUEST)
		fmt.Println("sent SERVERSTATUS_REQUEST (0x0005)")
	}
	if *charList {
		w := protocol.NewWriter(16)
		w.Short(int(protocol.RecvCHARLIST_REQUEST))
		w.Byte(0) // server/world
		w.Byte(0) // channel byte (0 -> channel 1)
		w.Int(0)
		sendPacket(conn, cSend, w.Bytes())
		fmt.Println("sent CHARLIST_REQUEST (0x0009)")
	}

	// ---- 3. listen for replies ----
	// Scripted modes exit once their expected replies arrived; bare mode
	// (-wait only) listens until the deadline.
	deadline := time.Now().Add(*wait)
	sawLoginReply, sawEndOfList, sawStatus, sawCharList, sawScripted := false, false, false, false, false
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		f := readFrameRaw(conn)
		if f == nil {
			break
		}
		header, bodyBytes := f[:4], f[4:]
		b := append([]byte(nil), bodyBytes...)
		cRecv.Crypt(b)
		crypto.ShandaDecrypt(b)
		fmt.Printf("recv: header=% X len=%d\n       plain=% X\n", header, len(b), b)
		if len(b) >= 2 {
			fmt.Printf("       opcode=0x%04X\n", uint16(b[0])|uint16(b[1])<<8)
		}
		switch uint16(b[0]) | uint16(b[1])<<8 {
		case 0x0000, 0x0004:
			sawLoginReply = true
		case 0x0009:
			if len(b) >= 3 && b[2] == 0xFF {
				sawEndOfList = true
			}
		case 0x0006:
			sawStatus = true
		case 0x000A:
			sawCharList = true
		case 0x0011, 0x7FFE, 0x000C: // scripted -hex replies
			sawScripted = true
		}
		decodeReply(b)
		done := true
		if *loginCred != "" && !sawLoginReply {
			done = false
		}
		if *serverList && !sawEndOfList {
			done = false
		}
		if *statusReq && !sawStatus {
			done = false
		}
		if *charList && !sawCharList {
			done = false
		}
		if *hexSend != "" && !sawScripted {
			done = false
		}
		if *loginCred == "" && !*serverList && !*statusReq && !*charList && *hexSend == "" {
			done = false // bare listen mode: run out the clock
		}
		if done {
			break
		}
	}
	fmt.Println("done")
}

// sendOpPacket sends a bare 2-byte opcode request.
func sendOpPacket(conn net.Conn, c *crypto.AESOFB, op protocol.RecvOp) {
	w := protocol.NewWriter(4)
	w.Short(int(op))
	sendPacket(conn, c, w.Bytes())
}

// decodeReply prints a human-readable view of the login-phase replies:
// LOGIN_STATUS (0x0000), CHOOSE_GENDER (0x0004), SERVERLIST (0x0009) and
// SERVERSTATUS (0x0006); other opcodes are ignored.
func decodeReply(b []byte) {
	if len(b) < 2 {
		return
	}
	op := uint16(b[0]) | uint16(b[1])<<8
	switch op {
	case 0x0000:
		if len(b) < 7 {
			fmt.Println("       [LOGIN_STATUS: truncated]")
			return
		}
		reason := b[2]
		accID := int(b[3]) | int(b[4])<<8 | int(b[5])<<16 | int(b[6])<<24
		switch reason {
		case 0:
			fmt.Printf("       [LOGIN_STATUS: OK, accID=%d]\n", accID)
		case 4:
			fmt.Println("       [LOGIN_STATUS: WRONG_PASSWORD (4)]")
		case 5:
			fmt.Println("       [LOGIN_STATUS: NOT_FOUND / degraded DB (5)]")
		case 7:
			fmt.Println("       [LOGIN_STATUS: ALREADY_LOGGED_IN (7)]")
		default:
			fmt.Printf("       [LOGIN_STATUS: reason=%d, accID=%d]\n", reason, accID)
		}
	case 0x0004:
		fmt.Println("       [CHOOSE_GENDER: account needs gender selection (gender=10)]")
	case 0x0009:
		decodeServerList(b)
	case 0x000A:
		decodeCharList(b)
	case 0x0041: // SERVERMESSAGE: P2.5 auto-register notices (serverNotice type 1)
		decodeServerMessage(b)
	case 0x0006:
		if len(b) < 4 {
			fmt.Println("       [SERVERSTATUS: truncated]")
			return
		}
		st := int(b[2]) | int(b[3])<<8
		name := map[int]string{0: "NORMAL", 1: "BUSY", 2: "FULL"}[st]
		fmt.Printf("       [SERVERSTATUS: %d (%s)]\n", st, name)
	}
}

// decodeServerMessage parses MaplePacketCreator.serverNotice(type, msg)
// (opcode 0x0041): byte type + MapleAsciiString message.
func decodeServerMessage(b []byte) {
	if len(b) < 3 {
		fmt.Println("       [SERVERMESSAGE: truncated]")
		return
	}
	typ := b[2]
	msg := protocol.NewReader(b[3:]).MapleAsciiString()
	fmt.Printf("       [SERVERMESSAGE: type=%d %q]\n", typ, msg)
}

// decodeServerList parses one SERVERLIST entry (or the 0xFF end marker):
// byte worldId + str name + byte state + str event + short 100 + short 100 +
// byte channelCount + int 500 + per channel (str name + int load + byte
// world + short channelId) + short balloons + per balloon (short x, short
// y, str msg) - Java LoginPacket.getServerList.
func decodeServerList(b []byte) {
	if len(b) < 3 {
		return
	}
	if b[2] == 0xFF {
		fmt.Println("       [SERVERLIST: END_OF_LIST (255)]")
		return
	}
	r := protocol.NewReader(b[2:])
	id := int(r.Byte())
	name := r.MapleAsciiString()
	state := int(r.Byte())
	event := r.MapleAsciiString()
	r.Short()
	r.Short()
	nCh := int(r.Byte())
	r.Int()
	fmt.Printf("       [SERVERLIST: world=%d name=%q state=%d event=%q channels=%d]\n", id, name, state, event, nCh)
	for i := 0; i < nCh; i++ {
		cn := r.MapleAsciiString()
		load := int(r.Int())
		r.Byte()
		ch := int(r.Short())
		fmt.Printf("       [SERVERLIST:   channel %q load=%d id=%d]\n", cn, load, ch)
	}
	for i, n := 0, int(r.Short()); i < n; i++ {
		x, y := int(r.Short()), int(r.Short())
		msg := r.MapleAsciiString()
		fmt.Printf("       [SERVERLIST:   balloon (%d,%d) %q]\n", x, y, msg)
	}
	if r.Err != nil {
		fmt.Printf("       [SERVERLIST: parse error: %v]\n", r.Err)
	}
}

// decodeCharList parses a CHARLIST reply (0x000A): byte 0 + int 0 + count +
// per char (stats entry + look entry + trailing byte [+2 if job 900]) +
// short 3 + int slots - Java LoginPacket.getCharList / PacketHelper.
func decodeCharList(b []byte) {
	if len(b) < 8 {
		fmt.Println("       [CHARLIST: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	r.Byte()
	r.Int()
	n := int(r.Byte())
	fmt.Printf("       [CHARLIST: %d char(s)]\n", n)
	for i := 0; i < n; i++ {
		id := int(r.Int())
		name := trimNameStr(r.AsciiString(13))
		gender := r.Byte()
		r.Byte() // skin
		r.Int()  // face
		r.Int()  // hair
		r.Skip(24)
		level := int(r.Byte())
		job := int(r.Short())
		for j := 0; j < 10; j++ { // stats + ap + sp
			r.Short()
		}
		r.Int()   // exp
		r.Short() // fame
		r.Int()   // gachapon
		r.Long()  // time
		mapID := int(r.Int())
		r.Byte() // spawn
		// look: gender, skin, face, mega byte, hair, 0xFF, 0xFF, cWeapon, 3 pets
		r.Byte()
		r.Byte()
		r.Int()
		r.Byte()
		r.Int()
		r.Byte()
		r.Byte()
		r.Int()
		r.Int()
		r.Int()
		r.Int()
		r.Byte() // addCharEntry trailing 0
		fmt.Printf("       [CHARLIST:   id=%d name=%q level=%d job=%d gender=%d map=%d]\n",
			id, name, level, job, gender, mapID)
	}
	r.Short()
	slots := int(r.Int())
	fmt.Printf("       [CHARLIST: slots=%d]\n", slots)
	if r.Err != nil {
		fmt.Printf("       [CHARLIST: parse error: %v]\n", r.Err)
	}
}

func trimNameStr(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return s[:i]
		}
	}
	return s
}

func readFrameRaw(conn net.Conn) []byte {
	hdr := readN(conn, 4)
	if hdr == nil {
		return nil
	}
	// header is not checkPacket-validatable pre-decrypt on client side here;
	// length decode is the same arithmetic
	v := uint32(hdr[0])<<24 | uint32(hdr[1])<<16 | uint32(hdr[2])<<8 | uint32(hdr[3])
	l := v>>16 ^ v&0xFFFF
	n := int(l<<8&0xFF00 | l>>8)
	body := readN(conn, n)
	if body == nil {
		return nil
	}
	return append(hdr, body...)
}

func readN(conn net.Conn, n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil
	}
	return buf
}

func sendPacket(conn net.Conn, c *crypto.AESOFB, body []byte) {
	b := append([]byte(nil), body...)
	hdr := c.PacketHeader(len(b))
	crypto.ShandaEncrypt(b)
	c.Crypt(b)
	frame := append(hdr[:], b...)
	conn.Write(frame)
}
