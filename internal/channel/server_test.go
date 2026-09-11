package channel

// P4.1 channel-server skeleton tests: port formula (Java 7574 + channel),
// wire handshake (hello version 79), PONG -> PING, PLAYER_LOGGEDIN skeleton
// logging, PlayerStorage registration + load reporting.

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/crypto"
	"GMS/internal/protocol"
)

func testConfig(count int) Config {
	return Config{
		Host:     "127.0.0.1",
		BasePort: 0, // :0 - OS-assigned ports per channel
		Count:    count,
	}
}

func TestChannelPortFormula(t *testing.T) {
	// Java ChannelServer: port = 7574 + channel, channel ids start at 1.
	cfg := Config{Host: "127.0.0.1", BasePort: 7575, Count: 3}
	s := New(cfg, slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	require.NoError(t, s.Start())
	defer s.Stop()

	assert.Equal(t, 7575, s.Channel(1).Port())
	assert.Equal(t, 7576, s.Channel(2).Port())
	assert.Equal(t, 7577, s.Channel(3).Port())
	assert.Nil(t, s.Channel(0))  // ids are 1-based
	assert.Nil(t, s.Channel(4)) // beyond count
}

func TestChannelCountCappedAtTen(t *testing.T) {
	// Java startChannel_Main: if (ch > 10) ch = 10.
	cfg := Config{Host: "127.0.0.1", BasePort: 0, Count: 15}
	s := New(cfg, slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	require.NoError(t, s.Start())
	defer s.Stop()
	assert.NotNil(t, s.Channel(10))
	assert.Nil(t, s.Channel(11))
}

func TestChannelRatesClampedAt100(t *testing.T) {
	// Java run_startup_configurations: >100 -> 100.
	s := New(Config{Host: "127.0.0.1", BasePort: 0, Count: 1, ExpRate: 250, MesoRate: 20, DropRate: 999},
		slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	assert.Equal(t, 100, s.cfg.ExpRate)
	assert.Equal(t, 20, s.cfg.MesoRate)
	assert.Equal(t, 100, s.cfg.DropRate)
}

// dialChannel connects to a channel port and performs the handshake,
// returning conn + client codecs (mirrors the login test helpers).
func dialChannel(t *testing.T, addr string) (net.Conn, *crypto.AESOFB, *crypto.AESOFB) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	hello := readBytes(t, conn, 15)
	version := int16(uint16(hello[2]) | uint16(hello[3])<<8)
	require.Equal(t, int16(79), version, "channel hello must carry version 79 like the login server")
	recvIV := [4]byte{hello[6], hello[7], hello[8], hello[9]}
	sendIV := [4]byte{hello[10], hello[11], hello[12], hello[13]}
	return conn, crypto.NewAESOFB(recvIV, 79), crypto.NewAESOFB(sendIV, -80)
}

func TestChannelHelloAndPing(t *testing.T) {
	s := New(testConfig(2), slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	require.NoError(t, s.Start())
	defer s.Stop()

	conn, cSend, cRecv := dialChannel(t, s.Channel(2).acceptor.Addr().String())

	w := protocol.NewWriter(4)
	w.Short(int(protocol.RecvPONG))
	sendTestFrame(t, conn, cSend, w.Bytes())

	reply := readTestReply(t, conn, cRecv)
	require.GreaterOrEqual(t, len(reply), 2, "PONG must be answered by PING")
	op := uint16(reply[0]) | uint16(reply[1])<<8
	assert.Equal(t, uint16(protocol.SendPING), op)
}

func TestChannelPlayerLoggedInWithoutStore(t *testing.T) {
	// P4.2: PLAYER_LOGGEDIN now loads the character and sends WARP_TO_MAP. A
	// channel server without a character store cannot serve it and closes the
	// session (the wired path is covered by login_test.go).
	s := New(testConfig(1), slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	require.NoError(t, s.Start())
	defer s.Stop()

	conn, cSend, _ := dialChannel(t, s.Channel(1).acceptor.Addr().String())

	w := protocol.NewWriter(8)
	w.Short(int(protocol.RecvPLAYER_LOGGEDIN))
	w.Int(123)
	sendTestFrame(t, conn, cSend, w.Bytes())

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("PLAYER_LOGGEDIN without a store must close the session")
	}
}

func TestPlayerStorageRegisterAndLoad(t *testing.T) {
	var reported []int
	s := New(testConfig(1), slog.New(slog.NewTextHandler(&discardLog{}, nil)))
	s.SetLoadReporter(func(ch, players int) { reported = append(reported, players) })
	require.NoError(t, s.Start())
	defer s.Stop()

	ps := s.Channel(1).Players()
	p := &Player{ID: 5, Name: "Tester"}
	ps.RegisterPlayer(p)

	assert.Equal(t, 1, ps.ConnectedPlayers())
	assert.Same(t, p, ps.GetPlayerByID(5))
	assert.Same(t, p, ps.GetCharacterByName("tester")) // lowercase key
	assert.Same(t, p, ps.GetCharacterByName("TESTER"))

	// World.Find side effect
	e, ok := s.Finder().Find(5)
	require.True(t, ok)
	assert.Equal(t, 5, e.ID)
	assert.Equal(t, 1, e.Channel)

	ps.DeregisterPlayer(p)
	assert.Equal(t, 0, ps.ConnectedPlayers())
	assert.Nil(t, ps.GetPlayerByID(5))
	_, ok = s.Finder().Find(5)
	assert.False(t, ok)

	// channel 0 (startup report) + 1 (register) + 0 (deregister)
	assert.Equal(t, []int{0, 1, 0}, reported, "load reporter must fire on startup, register and deregister")
}

// ---- tiny wire helpers (the login package has identical unexported ones) ----

type discardLog struct{}

func (discardLog) Write(p []byte) (int, error) { return len(p), nil }

func readBytes(t *testing.T, c net.Conn, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	require.NoError(t, readFull(c, buf))
	return buf
}

func readFull(c net.Conn, buf []byte) error {
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}

func sendTestFrame(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, body []byte) {
	t.Helper()
	b := append([]byte(nil), body...)
	hdr := ofb.PacketHeader(len(b))
	crypto.ShandaEncrypt(b)
	ofb.Crypt(b)
	frame := append(hdr[:], b...)
	_, err := conn.Write(frame)
	require.NoError(t, err)
}

func readTestReply(t *testing.T, conn net.Conn, ofb *crypto.AESOFB) []byte {
	t.Helper()
	hdr := readBytes(t, conn, 4)
	v := uint32(hdr[0])<<24 | uint32(hdr[1])<<16 | uint32(hdr[2])<<8 | uint32(hdr[3])
	l := v>>16 ^ v&0xFFFF
	n := int(l<<8&0xFF00 | l>>8)
	body := readBytes(t, conn, n)
	ofb.Crypt(body)
	crypto.ShandaDecrypt(body)
	return body
}
