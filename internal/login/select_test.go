package login

// P4.1 e2e over the wire: login -> CHARLIST -> CHAR_SELECT -> SERVER_IP
// (the login -> channel handoff), plus the Java guard branches: DB state
// mismatch (getLoginFailed 7), unauthorized char (enableActions), stopped
// channel (session close), and the login-ticket registry side effect.

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/config"
	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/protocol"
	"GMS/internal/world"
)

// startSelectServer boots a login server wired like cmd/gms does for P4.1:
// registry + channel-port lookup + external ip. The account is "charuser"
// so dialCharLogin's hardcoded credentials apply.
func startSelectServer(t *testing.T, channelOK bool) (*Server, *fakeStore, *world.LoginRegistry) {
	t.Helper()
	fs := newFakeStore()
	fs.accounts["charuser"] = &database.Account{
		ID: 21, Name: "charuser", Password: gSha1("pw"), Gender: 1,
	}
	fs.states[21] = database.AccountState{LoggedIn: 0}
	fs.characters = append(fs.characters, database.Character{
		ID: 31, AccountID: 21, World: 0, Name: "测试者", Level: 1,
	})

	lg := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	srv := New(config.Login{Host: "127.0.0.1", Port: 0}, lg)
	srv.SetStore(fs)
	srv.SetWorlds(WorldConfig{ServerName: "GMS", UserLimit: 500,
		Worlds: []World{{ID: 0, State: 1}}, ChannelCount: 3})
	registry := world.NewLoginRegistry()
	srv.SetRegistry(registry)
	require.NoError(t, srv.SetExternalIP("127.0.0.1"))
	srv.SetChannelPortLookup(func(ch int) (int, bool) {
		if !channelOK {
			return 0, false
		}
		return 7574 + ch, true // Java port formula; channel 1 -> 7575
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	return srv, fs, registry
}

// dialSelectFlow logs in, loads the charlist (fills allowedChars), and
// returns conn + codecs ready for the CHAR_SELECT packet.
func dialSelectFlow(t *testing.T, srv *Server) (net.Conn, *crypto.AESOFB, *crypto.AESOFB) {
	t.Helper()
	conn, cSend, cRecv := dialCharLogin(t, srv)
	// simulate the post-login DB state (fakeStore does not self-update:
	// the real accounts.loggedin column reads 2 after a successful login)
	srv.store.(*fakeStore).states[21] = database.AccountState{LoggedIn: database.LoginLoggedIn}
	sendCharlist(t, conn, cSend)
	list := readReply(t, conn, cRecv)
	if op := opOf(list); op != 0x000A {
		t.Fatalf("charlist reply opcode = %04X, want 000A", op)
	}
	return conn, cSend, cRecv
}

func TestCharSelectServerIPReply(t *testing.T) {
	srv, fs, registry := startSelectServer(t, true)
	defer srv.Stop()

	conn, cSend, cRecv := dialSelectFlow(t, srv)
	defer conn.Close()

	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvCHAR_SELECT))
	w.Int(31)
	sendFrame(t, conn, cSend, w.Bytes())

	b := readReply(t, conn, cRecv)
	// Java MaplePacketCreator.getServerIP(port, clientId):
	//   short SERVER_IP + short 0 + ip[4] + short port + int charId +
	//   {1, 0, 0, 0, 0}
	want := []byte{
		0x0B, 0x00, // SERVER_IP
		0x00, 0x00, // short 0
		0x7F, 0x00, 0x00, 0x01, // 127.0.0.1
		0x97, 0x1D, // port 7575 (7574 + channel 1) little-endian
		0x1F, 0x00, 0x00, 0x00, // charId 31
		0x01, 0x00, 0x00, 0x00, 0x00,
	}
	require.Equal(t, want, b, "SERVER_IP body mismatch")

	// putLoginAuth side effect: ticket keyed by charId, ip in "ip:port"
	// form, channel 1 (the charlist request selected channel 1).
	auth, ok := registry.TakeLoginAuth(31)
	require.True(t, ok, "login ticket must be recorded")
	assert.Equal(t, "127.0.0.1:", auth.IP[:len("127.0.0.1:")])
	assert.Equal(t, "", auth.TempIP)
	assert.Equal(t, 1, auth.Channel)
	_ = fs
}

func TestCharSelectDBStateGuard(t *testing.T) {
	// Java 取账号在线状态(DB) != 2 -> getLoginFailed(7): the live accounts
	// column governs, not the in-memory login state.
	srv, _, _ := startSelectServer(t, true)
	defer srv.Stop()

	conn, cSend, cRecv := dialSelectFlow(t, srv)
	defer conn.Close()
	// drop the live loggedin state after the charlist loaded: the DB check
	// (not the in-memory one) must reject the select
	srv.store.(*fakeStore).states[21] = database.AccountState{LoggedIn: 0}

	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvCHAR_SELECT))
	w.Int(31)
	sendFrame(t, conn, cSend, w.Bytes())

	if got := failedReason(t, readReply(t, conn, cRecv)); got != 7 {
		t.Fatalf("reason = %d, want 7 (already logged in)", got)
	}
}

func TestCharSelectUnauthorizedChar(t *testing.T) {
	// Java !login_Auth(charId) -> enableActions (UPDATE_STATS skeleton).
	srv, _, _ := startSelectServer(t, true)
	defer srv.Stop()

	conn, cSend, cRecv := dialSelectFlow(t, srv)
	defer conn.Close()

	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvCHAR_SELECT))
	w.Int(999) // not in allowedChars (charlist only authorized 31)
	sendFrame(t, conn, cSend, w.Bytes())

	b := readReply(t, conn, cRecv)
	// enableActions: UPDATE_STATS + byte 1 + int 0 + short 0
	want := []byte{0x22, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	require.Equal(t, want, b)
}

func TestCharSelectStoppedChannelCloses(t *testing.T) {
	// Java ChannelServer.getInstance(channel) == null -> session close.
	srv, _, _ := startSelectServer(t, false)
	defer srv.Stop()

	conn, cSend, _ := dialSelectFlow(t, srv)
	defer conn.Close()

	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvCHAR_SELECT))
	w.Int(31)
	sendFrame(t, conn, cSend, w.Bytes())

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected the connection to be closed, got data")
	}
}

// loginOn performs the handshake + LOGIN_PASSWORD for an arbitrary account
// and returns the connection, codecs, and the first reply.
func loginOn(t *testing.T, srv *Server, user, pwd string) (net.Conn, *crypto.AESOFB, []byte) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", srv.acceptor.Addr().String(), 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	hello := readN(t, conn, 15)
	recvIV := [4]byte{hello[6], hello[7], hello[8], hello[9]}
	sendIV := [4]byte{hello[10], hello[11], hello[12], hello[13]}
	cSend := crypto.NewAESOFB(recvIV, 79)
	cRecv := crypto.NewAESOFB(sendIV, -80)

	w := protocol.NewWriter(64)
	w.Short(int(protocol.RecvLOGIN_PASSWORD))
	w.MapleAsciiString(user)
	w.MapleAsciiString(pwd)
	w.Write([]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66})
	sendFrame(t, conn, cSend, w.Bytes())
	return conn, cRecv, readReply(t, conn, cRecv)
}

func TestDoubleLoginEvictsLiveSession(t *testing.T) {
	// Java unlockAcc first branch: a live session holds the account ->
	// unLockDisconnect (notice + close), the new attempt still answers 7.
	fs := newFakeStore()
	fs.accounts["dupuser"] = &database.Account{
		ID: 41, Name: "dupuser", Password: gSha1("pw"), Gender: 1,
	}
	fs.states[41] = database.AccountState{LoggedIn: 0}

	srv := New(config.Login{Host: "127.0.0.1", Port: 0},
		slog.New(slog.NewTextHandler(&discardWriter{}, nil)))
	srv.SetStore(fs)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	conn1, cRecv1, first := loginOn(t, srv, "dupuser", "pw")
	require.Equal(t, 0, int(first[2]), "first login must succeed")

	// the real store's loggedin column now reads 2 (fakeStore does not
	// self-update on UpdateLoginState)
	fs.states[41] = database.AccountState{LoggedIn: database.LoginLoggedIn}

	_, _, second := loginOn(t, srv, "dupuser", "pw")
	require.Equal(t, 7, failedReason(t, second))

	// the evicted session gets the notice, then the socket closes
	notice := readReply(t, conn1, cRecv1)
	require.Equal(t, uint16(protocol.SendSERVERMESSAGE), opOf(notice))
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 8)
	if _, err := conn1.Read(buf); err == nil {
		t.Fatal("evicted session should be closed")
	}
}

func TestDoubleLoginClearsStaleLoggedIn(t *testing.T) {
	// Java unlockAcc else-branch: no live session (crashed client) -> the
	// stale loggedin is cleared so the retry can proceed; this attempt
	// still answers 7.
	fs := newFakeStore()
	fs.accounts["orphan"] = &database.Account{
		ID: 51, Name: "orphan", Password: gSha1("goldenpassword"), Gender: 1,
	}
	fs.states[51] = database.AccountState{LoggedIn: database.LoginLoggedIn}

	body := loginFlow(t, fs, "orphan", "goldenpassword")
	if got := failedReason(t, body); got != 7 {
		t.Fatalf("reason = %d, want 7 (already logged in)", got)
	}
	if _, ok := fs.updates["reset:51"]; !ok {
		t.Fatal("stale loggedin must be reset so the retry can proceed")
	}
}

func TestSetChannelLoadAndDisplay(t *testing.T) {
	// LoginWorker scaling: factor = 1200 * channels / userLimit,
	// load = min(1200, players * factor); usersOn = true player sum.
	srv := New(config.Login{Host: "127.0.0.1", Port: 0},
		slog.New(slog.NewTextHandler(&discardWriter{}, nil)))
	srv.SetWorlds(WorldConfig{ServerName: "GMS", UserLimit: 600,
		Worlds: []World{{ID: 0, State: 1}}, ChannelCount: 2})

	srv.SetChannelLoad(1, 50)
	srv.SetChannelLoad(2, 10)
	assert.Equal(t, 60, srv.UsersOn())

	// factor = 1200*2/600 = 4: channel 1 -> 200, channel 2 -> 40
	load := srv.displayChannelLoad()
	assert.Equal(t, map[int]int{1: 200, 2: 40}, load)

	// the cap: 400 players on one channel -> 1200+ would read as 1200
	srv.SetChannelLoad(1, 400) // 400*4 = 1600 -> capped
	assert.Equal(t, 1200, srv.displayChannelLoad()[1])
}
