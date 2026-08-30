package login

// P2.4 e2e over the wire: login -> CHARLIST (empty) -> CHECK_CHAR_NAME ->
// CREATE_CHAR -> CHARLIST (one char, fields asserted) -> DELETE_CHAR ->
// CHARLIST (empty again). Plus the SET_GENDER flow for a gender=10 account
// (CHOOSE_GENDER -> SET_GENDER -> GENDER_SET + license + state reset).

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"GMS/internal/config"
	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
)

// startCharServer boots a login server with a fake account and default
// world set for the char flow.
func startCharServer(t *testing.T, gender int) (*Server, *fakeStore) {
	t.Helper()
	fs := newFakeStore()
	fs.accounts["charuser"] = &database.Account{
		ID: 11, Name: "charuser", Password: gSha1("pw"), Gender: gender,
	}
	fs.states[11] = database.AccountState{LoggedIn: 0}

	lg := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	srv := New(config.Login{Host: "127.0.0.1", Port: 0}, lg)
	srv.SetStore(fs)
	srv.SetWorlds(WorldConfig{
		ServerName: "GMS", UserLimit: 500,
		Worlds: []World{{ID: 0, State: 1}}, ChannelCount: 3,
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	return srv, fs
}

// dialCharLogin logs in as charuser and returns conn + codecs.
func dialCharLogin(t *testing.T, srv *Server) (net.Conn, *crypto.AESOFB, *crypto.AESOFB) {
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
	w.MapleAsciiString("charuser")
	w.MapleAsciiString("pw")
	w.Write([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
	sendFrame(t, conn, cSend, w.Bytes())

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	body := readReply(t, conn, cRecv)
	if len(body) < 3 || body[2] != 0x00 && body[2] != 0x04 {
		// gender=10 accounts take the CHOOSE_GENDER branch instead
		t.Fatalf("login reply = % X", body)
	}
	return conn, cSend, cRecv
}

// sendCharlist sends a CHARLIST_REQUEST for world 0, channel 1.
func sendCharlist(t *testing.T, conn net.Conn, cSend *crypto.AESOFB) {
	w := protocol.NewWriter(16)
	w.Short(int(protocol.RecvCHARLIST_REQUEST))
	w.Byte(0) // server
	w.Byte(0) // channel byte (0 -> channel 1)
	w.Int(0)  // skipped
	sendFrame(t, conn, cSend, w.Bytes())
}

// parseCharlist decodes a CHARLIST body into names and the slot count.
func parseCharlist(t *testing.T, b []byte) (names []string, slots int) {
	t.Helper()
	if op := uint16(b[0]) | uint16(b[1])<<8; op != 0x000A {
		t.Fatalf("opcode = % X, want 0A 00 (CHARLIST)", b[0:2])
	}
	r := protocol.NewReader(b[2:])
	r.Byte() // 0
	r.Int()  // 0
	n := int(r.Byte())
	for i := 0; i < n; i++ {
		id := int(r.Int())
		name := r.AsciiString(13)
		r.Byte() // gender
		r.Byte() // skin
		r.Int()  // face
		r.Int()  // hair
		r.Skip(24)
		r.Byte() // level
		r.Short() // job
		for j := 0; j < 10; j++ { // 8 stat shorts + ap + sp
			r.Short()
		}
		r.Int()  // exp
		r.Short() // fame
		r.Int()  // gachapon
		r.Long() // time
		r.Int()  // map
		r.Byte() // spawn
		// look
		r.Byte()
		r.Byte()
		r.Int() // face
		r.Byte()
		r.Int() // hair
		r.Byte() // 0xFF visible end
		r.Byte() // 0xFF masked end
		r.Int()  // cWeapon
		r.Int()  // pet1
		r.Int()  // pet2
		r.Int()  // pet3
		r.Byte() // addCharEntry trailing 0
		_ = id
		names = append(names, trimName(name))
	}
	r.Short() // 3
	slots = int(r.Int())
	if r.Err != nil {
		t.Fatalf("charlist parse error: %v", r.Err)
	}
	return names, slots
}

func trimName(b string) string {
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			return b[:i]
		}
	}
	return b
}

func TestCharFlow(t *testing.T) {
	srv, fs := startCharServer(t, 1)
	defer srv.Stop()
	conn, cSend, cRecv := dialCharLogin(t, srv)
	defer conn.Close()

	// 1. empty char list
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sendCharlist(t, conn, cSend)
	names, slots := parseCharlist(t, readReply(t, conn, cRecv))
	if len(names) != 0 {
		t.Fatalf("fresh account has chars: %v", names)
	}
	if slots != 6 {
		t.Fatalf("slots = %d, want 6", slots)
	}

	// 2. check name (CJK 2..5 chars ok)
	w := protocol.NewWriter(16)
	w.Short(int(protocol.RecvCHECK_CHAR_NAME))
	w.MapleAsciiString("测试1")
	sendFrame(t, conn, cSend, w.Bytes())
	bName := readReply(t, conn, cRecv)
	if op := uint16(bName[0]) | uint16(bName[1])<<8; op != 0x000C {
		t.Fatalf("check-name opcode = % X", bName[0:2])
	}
	r := protocol.NewReader(bName[2:])
	if got := r.MapleAsciiString(); got != "测试1" {
		t.Fatalf("check-name echo = %q", got)
	}
	if used := r.Byte(); used != 0 {
		t.Fatalf("name used = %d, want 0", used)
	}

	// 3. create char (JobType 1 = adventurer, starter look)
	w = protocol.NewWriter(48)
	w.Short(int(protocol.RecvCREATE_CHAR))
	w.MapleAsciiString("测试1")
	w.Int(1)        // JobType: adventurer -> job 0, map 0
	w.Int(20000)    // face
	w.Int(30000)    // hair
	w.Int(1040002)  // top
	w.Int(1060002)  // bottom
	w.Int(1072001)  // shoes (whitelisted)
	w.Int(1302000)  // weapon (whitelisted)
	sendFrame(t, conn, cSend, w.Bytes())
	bNew := readReply(t, conn, cRecv)
	if op := uint16(bNew[0]) | uint16(bNew[1])<<8; op != 0x0011 {
		t.Fatalf("create opcode = % X", bNew[0:2])
	}
	if bNew[2] != 0 {
		t.Fatalf("create worked flag = %d, want 0", bNew[2])
	}
	if len(fs.characters) != 1 {
		t.Fatalf("fake store chars = %d, want 1", len(fs.characters))
	}
	nc := fs.characters[0]
	if nc.Name != "测试1" || nc.Level != 1 || nc.Job != 0 || nc.Map != 0 {
		t.Fatalf("created char = %+v", nc)
	}
	if nc.Str != 12 || nc.Dex != 5 || nc.Int != 4 || nc.Luk != 4 ||
		nc.HP != 50 || nc.MP != 50 || nc.MaxHP != 50 || nc.MaxMP != 50 {
		t.Fatalf("created stats = %+v", nc)
	}
	if nc.BuddyCapacity != 20 || nc.Spawnpoint != 0 {
		t.Fatalf("created misc = %+v", nc)
	}

	// 4. char list now shows the char (fields walked by the parser)
	sendCharlist(t, conn, cSend)
	names, slots = parseCharlist(t, readReply(t, conn, cRecv))
	if len(names) != 1 || names[0] != "测试1" {
		t.Fatalf("char list = %v", names)
	}
	charID := fs.characters[0].ID

	// 5. duplicate name check -> notice + LOGIN_STATUS failed(1) + used=1
	w = protocol.NewWriter(16)
	w.Short(int(protocol.RecvCHECK_CHAR_NAME))
	w.MapleAsciiString("测试1")
	sendFrame(t, conn, cSend, w.Bytes())
	readReply(t, conn, cRecv) // SERVERMESSAGE dialog
	readReply(t, conn, cRecv) // LOGIN_STATUS failed(1)
	bDup := readReply(t, conn, cRecv)
	if op := uint16(bDup[0]) | uint16(bDup[1])<<8; op != 0x000C {
		t.Fatalf("dup check-name opcode = % X", bDup[0:2])
	}
	if used := bDup[len(bDup)-1]; used != 1 {
		t.Fatalf("dup name used = %d, want 1", used)
	}

	// 6. delete char (account has no 2ndpassword -> state 0)
	w = protocol.NewWriter(24)
	w.Short(int(protocol.RecvDELETE_CHAR))
	w.Byte(0)
	w.MapleAsciiString("")
	w.Int(int32(charID))
	sendFrame(t, conn, cSend, w.Bytes())
	bDel := readReply(t, conn, cRecv)
	if op := uint16(bDel[0]) | uint16(bDel[1])<<8; op != 0x7FFE {
		t.Fatalf("delete opcode = %04X", op)
	}
	rd := protocol.NewReader(bDel[2:])
	if cid := int(rd.Int()); cid != charID {
		t.Fatalf("delete cid = %d, want %d", cid, charID)
	}
	if state := rd.Byte(); state != 0 {
		t.Fatalf("delete state = %d, want 0", state)
	}
	if len(fs.characters) != 0 {
		t.Fatalf("char not deleted: %d left", len(fs.characters))
	}

	// 7. list empty again
	sendCharlist(t, conn, cSend)
	names, _ = parseCharlist(t, readReply(t, conn, cRecv))
	if len(names) != 0 {
		t.Fatalf("post-delete list = %v", names)
	}
}

func TestSetGenderFlow(t *testing.T) {
	srv, fs := startCharServer(t, 10) // gender=10 -> CHOOSE_GENDER
	defer srv.Stop()

	conn, err := net.DialTimeout("tcp", srv.acceptor.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	hello := readN(t, conn, 15)
	recvIV := [4]byte{hello[6], hello[7], hello[8], hello[9]}
	sendIV := [4]byte{hello[10], hello[11], hello[12], hello[13]}
	cSend := crypto.NewAESOFB(recvIV, 79)
	cRecv := crypto.NewAESOFB(sendIV, -80)

	// login -> CHOOSE_GENDER (04 00)
	w := protocol.NewWriter(64)
	w.Short(int(protocol.RecvLOGIN_PASSWORD))
	w.MapleAsciiString("charuser")
	w.MapleAsciiString("pw")
	w.Write([]byte{1, 2, 3, 4, 5, 6})
	sendFrame(t, conn, cSend, w.Bytes())
	bChoose := readReply(t, conn, cRecv)
	if op := uint16(bChoose[0]) | uint16(bChoose[1])<<8; op != 0x0004 {
		t.Fatalf("opcode = % X, want 04 00 (CHOOSE_GENDER)", bChoose[0:2])
	}

	// SET_GENDER: byte 1 + str "charuser"
	w = protocol.NewWriter(32)
	w.Short(int(protocol.RecvSET_GENDER))
	w.Byte(1)
	w.MapleAsciiString("charuser")
	sendFrame(t, conn, cSend, w.Bytes())

	// expect GENDER_SET (05 00) then LOGIN_STATUS 22 (license request)
	bSet := readReply(t, conn, cRecv)
	if op := uint16(bSet[0]) | uint16(bSet[1])<<8; op != 0x0005 {
		t.Fatalf("gender set opcode = % X, want 05 00", bSet[0:2])
	}
	r := protocol.NewReader(bSet[2:])
	r.Byte() // 0
	if name := r.MapleAsciiString(); name != "charuser" {
		t.Fatalf("gender set name = %q", name)
	}
	if idStr := r.MapleAsciiString(); idStr != "11" {
		t.Fatalf("gender set accID str = %q", idStr)
	}
	bLic := readReply(t, conn, cRecv)
	if op := uint16(bLic[0]) | uint16(bLic[1])<<8; op != 0x0000 || bLic[2] != 22 {
		t.Fatalf("license request = % X, want 00 00 16", bLic[0:3])
	}

	// gender + loggedin=0 persisted
	if g, ok := fs.updates["gender:11"]; !ok || g != "1" {
		t.Fatalf("gender update = %v", fs.updates)
	}
	if st, ok := fs.updates["state:11"]; !ok || st == "" || st[0] != '0' {
		t.Fatalf("login state after set gender = %q, want 0|<ip>", st)
	}
}

func TestCreateCharSlotLimit(t *testing.T) {
	srv, fs := startCharServer(t, 1)
	defer srv.Stop()
	fs.slots = 1 // one slot only
	fs.characters = []database.Character{{
		ID: 500, AccountID: 11, World: 0, Name: "已有1", Level: 1,
	}}
	conn, cSend, cRecv := dialCharLogin(t, srv)
	defer conn.Close()

	// read the initial LOGIN_STATUS ok first
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	w := protocol.NewWriter(48)
	w.Short(int(protocol.RecvCREATE_CHAR))
	w.MapleAsciiString("新建1")
	w.Int(1)
	w.Int(20000)
	w.Int(30000)
	w.Int(1040002)
	w.Int(1060002)
	w.Int(1072001)
	w.Int(1302000)
	sendFrame(t, conn, cSend, w.Bytes())

	// serverNotice dialog + LOGIN_STATUS failed(1) - no ADD_NEW_CHAR_ENTRY
	bNotice := readReply(t, conn, cRecv)
	if op := uint16(bNotice[0]) | uint16(bNotice[1])<<8; op != 0x0041 {
		t.Fatalf("notice opcode = % X, want 41 00 (SERVERMESSAGE)", bNotice[0:2])
	}
	if bNotice[2] != 1 {
		t.Fatalf("notice type = %d, want 1 (dialog)", bNotice[2])
	}
	bFail := readReply(t, conn, cRecv)
	if op := uint16(bFail[0]) | uint16(bFail[1])<<8; op != 0x0000 {
		t.Fatalf("login failed opcode = % X", bFail[0:2])
	}
	if reason := bFail[2]; reason != 1 {
		t.Fatalf("login failed reason = %d, want 1", reason)
	}
	if len(fs.characters) != 1 {
		t.Fatalf("slot limit not enforced: %d chars", len(fs.characters))
	}
}
