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
//	go run ./tools/protocoltest -addr 127.0.0.1:7575 -loggedinas 3 -chat hello -hold -wait 10s
//	go run ./tools/protocoltest -addr 127.0.0.1:7575 -loggedinas 3 -whisper "name:hi" -hold -wait 10s
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
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
	selectChar := flag.Int("selectchar", 0, "send CHAR_SELECT for this char id and decode SERVER_IP (requires -login; pair with -charlist to authorize)")
	deleteChar := flag.Int("deletechar", 0, "send DELETE_CHAR for this char id (requires -login -charlist to authorize)")
	loggedInAs := flag.Int("loggedinas", 0, "send PLAYER_LOGGEDIN for this char id (channel-server probe: -addr 127.0.0.1:7575)")
	moveTo := flag.String("move", "", "send MOVE_PLAYER (0x0024) to this 'x,y' position after PLAYER_LOGGEDIN (P4.4; pair with -loggedinas)")
	chatText := flag.String("chat", "", "send GENERAL_CHAT (0x002D) with this text after PLAYER_LOGGEDIN (P4.5; use ASCII - the Windows shell mangles CJK args)")
	emote := flag.Int("emote", 0, "send FACE_EXPRESSION (0x002F) with this emote id (P4.5)")
	whisper := flag.String("whisper", "", "send WHISPER mode 6 to 'name:text' (P4.5)")
	findWho := flag.String("find", "", "send WHISPER mode 5 'find character' for this name (P4.5)")
	hold := flag.Bool("hold", false, "keep listening until -wait elapses even after the scripted replies arrived (P4.3 spawn/despawn observation; required to see chat replies, which arrive after WARP_TO_MAP)")
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
	if *deleteChar > 0 {
		// Java DeleteChar: byte skip + str secondpw (empty = none) + int cid
		w := protocol.NewWriter(16)
		w.Short(int(protocol.RecvDELETE_CHAR))
		w.Byte(0)
		w.MapleAsciiString("")
		w.Int(int32(*deleteChar))
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent DELETE_CHAR (0x0012) charID=%d\n", *deleteChar)
	}
	if *selectChar > 0 {
		w := protocol.NewWriter(8)
		w.Short(int(protocol.RecvCHAR_SELECT))
		w.Int(int32(*selectChar))
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent CHAR_SELECT (0x000A) charID=%d\n", *selectChar)
	}
	if *loggedInAs > 0 {
		w := protocol.NewWriter(8)
		w.Short(int(protocol.RecvPLAYER_LOGGEDIN))
		w.Int(int32(*loggedInAs))
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent PLAYER_LOGGEDIN (0x000B) playerID=%d\n", *loggedInAs)
	}
	if *moveTo != "" {
		x, y, err := parsePoint(*moveTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "-move: %v\n", err)
			os.Exit(1)
		}
		w := protocol.NewWriter(64)
		w.Short(int(protocol.RecvMOVE_PLAYER))
		w.Zero(33) // v079 client prefix (PlayerHandler.MovePlayer -> slea.skip(33))
		w.Byte(1)  // one movement command
		w.Byte(0)  // 0 = normal move
		w.Pos(x, y)
		w.Pos(0, 0) // wobble (pixelsPerSecond)
		w.Short(0)  // unk
		w.Byte(0)   // newstate
		w.Short(0)  // duration
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent MOVE_PLAYER (0x0024) to (%d,%d)\n", x, y)
	}
	if *chatText != "" {
		w := protocol.NewWriter(16 + len(*chatText))
		w.Short(int(protocol.RecvGENERAL_CHAT))
		w.MapleAsciiString(*chatText)
		w.Byte(0) // Java reads the trailing byte as the getChatText "show" value
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent GENERAL_CHAT (0x002D) %q\n", *chatText)
	}
	if *emote != 0 {
		w := protocol.NewWriter(6)
		w.Short(int(protocol.RecvFACE_EXPRESSION))
		w.Int(int32(*emote))
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent FACE_EXPRESSION (0x002F) emote=%d\n", *emote)
	}
	if *whisper != "" {
		i := strings.Index(*whisper, ":")
		if i < 0 {
			fmt.Fprintf(os.Stderr, "-whisper: expect name:text\n")
			os.Exit(1)
		}
		w := protocol.NewWriter(32 + len(*whisper))
		w.Short(int(protocol.RecvWHISPER))
		w.Byte(6)                        // mode 6 = deliver a whisper
		w.MapleAsciiString((*whisper)[:i]) // recipient
		w.MapleAsciiString((*whisper)[i+1:])
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent WHISPER (0x0075) mode=6 to %q\n", (*whisper)[:i])
	}
	if *findWho != "" {
		w := protocol.NewWriter(16 + len(*findWho))
		w.Short(int(protocol.RecvWHISPER))
		w.Byte(5) // mode 5 = find a character
		w.MapleAsciiString(*findWho)
		sendPacket(conn, cSend, w.Bytes())
		fmt.Printf("sent WHISPER (0x0075) mode=5 find %q\n", *findWho)
	}

	// ---- 3. listen for replies ----
	// Scripted modes exit once their expected replies arrived; bare mode
	// (-wait only) listens until the deadline.
	deadline := time.Now().Add(*wait)
	sawLoginReply, sawEndOfList, sawStatus, sawCharList, sawScripted, sawServerIP, sawDeleteResp, sawWarpToMap := false, false, false, false, false, false, false, false
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
		case 0x000B:
			sawServerIP = true
		case 0x0081: // WARP_TO_MAP - the channel server let the player enter (P4.2)
			sawWarpToMap = true
		case 0x00A2: // SPAWN_PLAYER - another player entered the map (P4.3)
		case 0x00A3: // REMOVE_PLAYER_FROM_MAP - another player left the map (P4.3)
		case 0x00BB: // MOVE_PLAYER - another player moved (P4.4)
		case 0x00A4: // CHATTEXT - someone talked on the map (P4.5)
		case 0x00C3: // FACIAL_EXPRESSION - someone emoted (P4.5)
		case 0x008B: // WHISPER - whisper / find reply (P4.5)
		case 0x7FFE:
			if *deleteChar > 0 {
				sawDeleteResp = true
			}
			sawScripted = true
		case 0x0011, 0x000C: // scripted -hex replies
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
		if *selectChar > 0 && !sawServerIP {
			done = false
		}
		if *deleteChar > 0 && !sawDeleteResp {
			done = false
		}
		if *loggedInAs > 0 && !sawWarpToMap {
			done = false
		}
		if *hexSend != "" && !sawScripted {
			done = false
		}
		if *loginCred == "" && !*serverList && !*statusReq && !*charList && *selectChar == 0 && *deleteChar == 0 && *loggedInAs == 0 && *hexSend == "" {
			done = false // bare listen mode: run out the clock
		}
		if *hold {
			done = false // keep watching (spawn/despawn observation)
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
	case 0x000B: // SERVER_IP: the CHAR_SELECT reply (P4.1 channel handoff)
		decodeServerIP(b)
	case 0x0081: // WARP_TO_MAP: the channel server's getCharInfo (P4.2)
		decodeWarpToMap(b)
	case 0x00BB: // MOVE_PLAYER: another player moved (P4.4)
		decodeMovePlayer(b)
	case 0x00A2: // SPAWN_PLAYER: another player on the map (P4.3)
		decodeSpawnPlayer(b)
	case 0x00A3: // REMOVE_PLAYER_FROM_MAP (P4.3)
		if len(b) >= 6 {
			cid := int(b[2]) | int(b[3])<<8 | int(b[4])<<16 | int(b[5])<<24
			fmt.Printf("       [REMOVE_PLAYER_FROM_MAP: charID=%d]\n", cid)
		}
	case 0x0026: // TEMP_STATS_RESET
		fmt.Println("       [TEMP_STATS_RESET: empty body]")
	case 0x00A4: // CHATTEXT (P4.5)
		decodeChatText(b)
	case 0x00C3: // FACIAL_EXPRESSION (P4.5)
		decodeFaceExpression(b)
	case 0x008B: // WHISPER (P4.5)
		decodeWhisper(b)
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

// decodeServerMessage parses MaplePacketCreator.serverNotice(type, msg) /
// serverMessage(msg) (opcode 0x0041): byte type + [byte 1 when type == 4] +
// MapleAsciiString message.
func decodeServerMessage(b []byte) {
	if len(b) < 3 {
		fmt.Println("       [SERVERMESSAGE: truncated]")
		return
	}
	typ := int(b[2])
	off := 3
	if typ == 4 { // serverMessage(type 4): Java writes an extra byte 1
		off = 4
	}
	if off > len(b) {
		fmt.Println("       [SERVERMESSAGE: truncated]")
		return
	}
	msg := protocol.NewReader(b[off:]).MapleAsciiString()
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

// decodeServerIP parses the SERVER_IP reply (0x000B, Java
// MaplePacketCreator.getServerIP): short 0 + 4-byte ip + short port +
// int charId + fixed {1,0,0,0,0} tail. The client then reconnects there and
// opens with PLAYER_LOGGEDIN.
func decodeServerIP(b []byte) {
	// short op + short 0 + ip[4] + short port + int charId + tail[5]
	if len(b) < 19 {
		fmt.Println("       [SERVER_IP: truncated]")
		return
	}
	ip := net.IP(b[4:8]).String()
	port := int(b[8]) | int(b[9])<<8
	charID := int(b[10]) | int(b[11])<<8 | int(b[12])<<16 | int(b[13])<<24
	fmt.Printf("       [SERVER_IP: ip=%s port=%d charID=%d]\n", ip, port, charID)
}

// decodeWarpToMap parses the channel-enter packet (Java
// MaplePacketCreator.getCharInfo): short WARP_TO_MAP + int (channel-1) +
// byte 0 + byte 1 + byte 1 + short 0 + 3 rand ints + long -1 + byte 0 +
// addCharStats (int id + 13-byte name + gender/skin/face/hair + 24 zero +
// level/job/stat block/exp/fame + time + map + spawnpoint) + buddy capacity +
// byte 1 + inventory info ... Only the head fields are printed.
func decodeWarpToMap(b []byte) {
	if len(b) < 8 {
		fmt.Println("       [WARP_TO_MAP: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	ch := int(r.Int()) + 1
	r.Byte()
	r.Byte()
	r.Byte()
	r.Short()
	rnd1, rnd2, rnd3 := r.Int(), r.Int(), r.Int()
	r.Long()
	r.Byte()
	id := int(r.Int())
	name := trimNameStr(r.AsciiString(13))
	gender := int(r.Byte())
	r.Byte() // skin
	r.Int()  // face
	r.Int()  // hair
	r.Skip(24)
	level := int(r.Byte())
	job := int(r.Short())
	for i := 0; i < 10; i++ { // str..maxmp + ap + sp
		r.Short()
	}
	r.Int()   // exp
	r.Short() // fame
	r.Int()   // gachapon exp
	r.Long()  // time
	mapID := int(r.Int())
	spawn := int(r.Byte())
	capacity := int(r.Byte())
	fmt.Printf("       [WARP_TO_MAP: channel=%d charID=%d name=%q gender=%d level=%d job=%d map=%d spawn=%d buddy=%d rand=%d/%d/%d]\n",
		ch, id, name, gender, level, job, mapID, spawn, capacity, rnd1, rnd2, rnd3)
	if r.Err != nil {
		fmt.Printf("       [WARP_TO_MAP: parse error: %v]\n", r.Err)
	}
}

// decodeSpawnPlayer prints the head of MaplePacketCreator.spawnPlayerMapobject
// (0x00A2): int cid + byte level + str name + str guild. Everything past that
// is the buff/mount/look block (see packet.SpawnPlayerPacket).
func decodeSpawnPlayer(b []byte) {
	if len(b) < 8 {
		fmt.Println("       [SPAWN_PLAYER: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	id := int(r.Int())
	level := int(r.Byte())
	name := r.MapleAsciiString()
	guild := r.MapleAsciiString()
	fmt.Printf("       [SPAWN_PLAYER: charID=%d level=%d name=%q guild=%q]\n", id, level, name, guild)
}

// decodeMovePlayer prints the head of MaplePacketCreator.movePlayer (0x00BB):
// int cid + int 0 + movement list (count byte + fragments; see the movement
// package for the per-command layout).
func decodeMovePlayer(b []byte) {
	if len(b) < 11 {
		fmt.Println("       [MOVE_PLAYER: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	cid := int(r.Int())
	r.Int() // Java writes a literal 0 where startPos used to be
	n := int(r.Byte())
	fmt.Printf("       [MOVE_PLAYER: charID=%d commands=%d]\n", cid, n)
}

// decodeChatText parses MaplePacketCreator.getChatText (0x00A4): int cid +
// byte whiteBG + str text + byte show.
func decodeChatText(b []byte) {
	if len(b) < 7 {
		fmt.Println("       [CHATTEXT: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	cid := int(r.Int())
	white := int(r.Byte())
	text := r.MapleAsciiString()
	show := int(r.Byte())
	fmt.Printf("       [CHATTEXT: charID=%d gmBubble=%v text=%q show=%d]\n", cid, white == 1, text, show)
}

// decodeFaceExpression parses MaplePacketCreator.facialExpression (0x00C3):
// int cid + int expression.
func decodeFaceExpression(b []byte) {
	if len(b) < 10 {
		fmt.Println("       [FACIAL_EXPRESSION: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	cid := int(r.Int())
	emote := int(r.Int())
	fmt.Printf("       [FACIAL_EXPRESSION: charID=%d emote=%d]\n", cid, emote)
}

// decodeWhisper parses the MaplePacketCreator family behind 0x008B:
// byte 0x12 (getWhisper: sender + short channel-1 + text), 0x0A
// (getWhisperReply: target + byte reply) and the find replies
// (9/72: target + byte kind + int mapid/channel-1 [+ 8 zero bytes]).
func decodeWhisper(b []byte) {
	if len(b) < 3 {
		fmt.Println("       [WHISPER: truncated]")
		return
	}
	r := protocol.NewReader(b[2:])
	sub := r.Byte()
	switch sub {
	case 0x12:
		sender := r.MapleAsciiString()
		channel := int(r.Short()) + 1
		text := r.MapleAsciiString()
		fmt.Printf("       [WHISPER: from=%q channel=%d text=%q]\n", sender, channel, text)
	case 0x0A:
		target := r.MapleAsciiString()
		reply := int(r.Byte())
		fmt.Printf("       [WHISPER_REPLY: target=%q delivered=%v]\n", target, reply == 1)
	case 9, 72:
		target := r.MapleAsciiString()
		kind := int(r.Byte())
		switch kind {
		case 1:
			fmt.Printf("       [FIND_REPLY: target=%q on this channel, map=%d]\n", target, int(r.Int()))
		case 3:
			fmt.Printf("       [FIND_REPLY: target=%q on channel=%d]\n", target, int(r.Int())+1)
		case 0, 2:
			fmt.Printf("       [FIND_REPLY: target=%q kind=%d (cash shop / MTS - not ported)]\n", target, kind)
		default:
			fmt.Printf("       [FIND_REPLY: target=%q kind=%d]\n", target, kind)
		}
	default:
		fmt.Printf("       [WHISPER: subtype=0x%02X]\n", sub)
	}
}

// parsePoint parses an "x,y" flag value into a map position.
func parsePoint(s string) (int16, int16, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expect x,y")
	}
	x, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return int16(x), int16(y), nil
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
