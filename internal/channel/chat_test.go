package channel

// P4.5 e2e over the wire: GENERAL_CHAT (0x2D) -> CHATTEXT with view-range
// filtering, FACE_EXPRESSION (0x2F) -> FACIAL_EXPRESSION, WHISPER (0x75) ->
// whisper delivery plus the find-reply family.
//
// P4.5b adds the configvalues gates on top of the same flow: 玩家聊天开关
// closes GENERAL_CHAT (serverNotice type 1) and 游戏找人开关 closes the
// whisper find modes 5/68 (dropMessage(5, ...) -> serverNotice type 5).

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/crypto"
	"GMS/internal/database"
	"GMS/internal/movement"
	"GMS/internal/protocol"
)

// namedChar is testChar with a distinct name (PlayerStorage, World.Find and the
// whisper lookup are all name-keyed) and an optional GM level.
func namedChar(id, accID, mapID int, name string, gm int) *database.Character {
	c := testChar(id, accID, mapID)
	c.Name = name
	c.GM = gm
	return c
}

// allowReads gives the next reads a fresh 3s window.
func allowReads(t *testing.T, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
}

// expectSilence asserts nothing arrives within 250ms.
func expectSilence(t *testing.T, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(250*time.Millisecond)))
	if _, err := conn.Read(make([]byte, 16)); err == nil {
		t.Fatal("expected no reply")
	}
}

func sendGeneralChat(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, text string, unk byte) {
	t.Helper()
	w := protocol.NewWriter(16 + len(text))
	w.Short(int(protocol.RecvGENERAL_CHAT))
	w.MapleAsciiString(text)
	w.Byte(unk)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

func sendWhisperFind(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, mode byte, name string) {
	t.Helper()
	w := protocol.NewWriter(16 + len(name))
	w.Short(int(protocol.RecvWHISPER))
	w.Byte(mode)
	w.MapleAsciiString(name)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

func sendWhisperSend(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, name, text string) {
	t.Helper()
	w := protocol.NewWriter(32 + len(name) + len(text))
	w.Short(int(protocol.RecvWHISPER))
	w.Byte(6)
	w.MapleAsciiString(name)
	w.MapleAsciiString(text)
	sendTestFrame(t, conn, ofb, w.Bytes())
}

func sendEmote(t *testing.T, conn net.Conn, ofb *crypto.AESOFB, emote int) {
	t.Helper()
	w := protocol.NewWriter(6)
	w.Short(int(protocol.RecvFACE_EXPRESSION))
	w.Int(int32(emote))
	sendTestFrame(t, conn, ofb, w.Bytes())
}

func TestGeneralChatBroadcastAndViewRange(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	// A enters (WARP_TO_MAP, TEMP_STATS_RESET, own spawn).
	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	// B joins the same map (WARP_TO_MAP, TEMP_STATS_RESET, spawn A, spawn B).
	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1) // A is told about B
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 2 })

	// A chats: everyone on the map gets it - including the chatter itself
	// (Java broadcastMessage(packet, rangedFrom) passes source=null).
	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, "大家好", 3)

	self := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(self))
	sr := protocol.NewReader(self[2:])
	assert.Equal(t, int32(31), sr.Int(), "chatting charID")
	assert.Equal(t, byte(0), sr.Byte(), "whiteBG = isGM() = false")
	assert.Equal(t, "大家好", sr.MapleAsciiString())
	assert.Equal(t, byte(3), sr.Byte(), "the client's trailing byte is echoed")
	assert.NoError(t, sr.Err)
	assert.Zero(t, sr.Len())

	allowReads(t, connB)
	seen := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(seen))
	assert.Equal(t, int32(31), protocol.NewReader(seen[2:]).Int())

	// A walks out of view range (20000 units away), then chats again: A still
	// sees its own line, B must not receive it (maxViewRangeSq filter).
	sendMove(t, connA, sendA, moveBody(movement.Fragment{
		Type: 0, Pos: &movement.Point{X: 20000, Y: 0},
		Wobble: &movement.Point{X: 0, Y: 0}, NewState: 5, Duration: 60,
	}))
	allowReads(t, connB)
	require.Equal(t, uint16(protocol.SendMOVE_PLAYER), opOf(readTestReply(t, connB, recvB)))
	pA := cs.Players().GetPlayerByID(31)
	require.NotNil(t, pA)
	waitFor(t, func() bool { x, _ := pA.Position(); return x == 20000 })

	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, "走远了", 0)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(readTestReply(t, connA, recvA)))
	expectSilence(t, connB)
}

func TestGeneralChatLengthCapIgnoresGM(t *testing.T) {
	long := make([]byte, chatMaxLen)
	for i := range long {
		long[i] = 'x'
	}
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "普通", 0),
		32: namedChar(32, 8, 100000000, "管理", 5),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	// Non-GM: 80 UTF-16 units hit the Java `text.length() >= 80` cap.
	sendGeneralChat(t, connA, sendA, string(long), 0)
	expectSilence(t, connA)

	// 79 units pass.
	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, string(long[:chatMaxLen-1]), 0)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(readTestReply(t, connA, recvA)))

	// A GM is exempt from the cap.
	connB, sendB, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4) // own WARP/RESET/spawn(A)/spawn(B)
	allowReads(t, connB)
	sendGeneralChat(t, connB, sendB, string(long), 1)
	msg := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(msg))
	assert.Equal(t, byte(1), msg[6], "whiteBG = isGM() = true")
}

func TestFaceExpressionBroadcast(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs)
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1)

	// ChangeEmotion broadcasts to everyone but the source (boolean overload).
	allowReads(t, connB)
	sendEmote(t, connA, sendA, 5)
	expr := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendFACIAL_EXPRESSION), opOf(expr))
	er := protocol.NewReader(expr[2:])
	assert.Equal(t, int32(31), er.Int())
	assert.Equal(t, int32(5), er.Int())
	assert.Zero(t, er.Len())
	expectSilence(t, connA)

	// emote > 7 needs the cash expression item (P5.2); emote <= 0 is ignored.
	sendEmote(t, connA, sendA, 8)
	expectSilence(t, connB)
	sendEmote(t, connA, sendA, 0)
	expectSilence(t, connB)
}

func TestWhisperDeliveryAndFind(t *testing.T) {
	// A on channel 1, B on channel 2 and C back on channel 1 (different map so
	// the spawn broadcasts stay out of the way).
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
		33: namedChar(33, 9, 100000001, "丙", 0),
	}}
	s := startChannelWith(t, Config{Count: 2}, fs)

	connA, sendA, recvA := sendLoggedIn(t, s.Channel(1), 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	connB, _, recvB := sendLoggedIn(t, s.Channel(2), 32)
	defer connB.Close()
	drain(t, connB, recvB, 3)

	connC, _, recvC := sendLoggedIn(t, s.Channel(1), 33)
	defer connC.Close()
	drain(t, connC, recvC, 3)

	// mode 5 (find): same channel -> getFindReplyWithMap; other channel ->
	// getFindReply with channel-1.
	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 5, "丙")
	withMap := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(withMap))
	fr := protocol.NewReader(withMap[2:])
	assert.Equal(t, byte(9), fr.Byte(), "buddy=false -> 9")
	assert.Equal(t, "丙", fr.MapleAsciiString())
	assert.Equal(t, byte(1), fr.Byte(), "found on this channel")
	assert.Equal(t, int32(100000001), fr.Int(), "map id")
	assert.Equal(t, make([]byte, 8), fr.Read(8), "8 zero bytes")
	assert.Zero(t, fr.Len())

	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 68, "乙")
	chReply := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(chReply))
	cr := protocol.NewReader(chReply[2:])
	assert.Equal(t, byte(72), cr.Byte(), "buddy=true -> 72")
	assert.Equal(t, "乙", cr.MapleAsciiString())
	assert.Equal(t, byte(3), cr.Byte(), "online on another channel")
	assert.Equal(t, int32(1), cr.Int(), "channel 2 -> 1")
	assert.Zero(t, cr.Len())

	// mode 6: B receives the whisper, A the "delivered" reply.
	allowReads(t, connB)
	sendWhisperSend(t, connA, sendA, "乙", "你好")
	msg := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(msg))
	mr := protocol.NewReader(msg[2:])
	assert.Equal(t, byte(0x12), mr.Byte())
	assert.Equal(t, "甲", mr.MapleAsciiString(), "sender name")
	assert.Equal(t, int16(0), mr.Short(), "sender channel 1 -> 0")
	assert.Equal(t, "你好", mr.MapleAsciiString())
	assert.Zero(t, mr.Len())

	allowReads(t, connA)
	reply := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(reply))
	rr := protocol.NewReader(reply[2:])
	assert.Equal(t, byte(0x0A), rr.Byte())
	assert.Equal(t, "乙", rr.MapleAsciiString())
	assert.Equal(t, byte(1), rr.Byte(), "delivered")
	assert.Zero(t, rr.Len())

	// An unknown recipient answers "not found" both for find and for send.
	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 5, "查无此人")
	nf := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(nf))
	nr := protocol.NewReader(nf[2:])
	assert.Equal(t, byte(0x0A), nr.Byte())
	assert.Equal(t, "查无此人", nr.MapleAsciiString())
	assert.Equal(t, byte(0), nr.Byte())

	allowReads(t, connA)
	sendWhisperSend(t, connA, sendA, "查无此人", "在吗")
	nf = readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(nf))
	nr = protocol.NewReader(nf[2:])
	assert.Equal(t, byte(0x0A), nr.Byte())
	assert.Equal(t, "查无此人", nr.MapleAsciiString())
	assert.Equal(t, byte(0), nr.Byte())
}

// ---- P4.5b: the configvalues gates ----

// serverNoticeBytes builds the expected wire shape of the P4.5b notices,
// independently of packet.ServerNoticePacket: short SERVERMESSAGE (0x41 LE) +
// byte type + short length + GB18030 message. Neither type used here appends
// a trailer in Java (the smega types 3/9/10/11/12 and 6/18 do).
func serverNoticeBytes(kind int, message string) []byte {
	enc := protocol.EncodeGB18030(message)
	out := []byte{
		byte(protocol.SendSERVERMESSAGE), byte(protocol.SendSERVERMESSAGE >> 8),
		byte(kind), byte(len(enc)), byte(len(enc) >> 8),
	}
	return append(out, enc...)
}

// TestGeneralChatSwitchClosed: 玩家聊天开关 > 0 must answer GENERAL_CHAT with
// the serverNotice(1, ...) popup and return - no CHATTEXT to anybody, not even
// the chatter (Java ChatHandler.GeneralChat:44, before the GM length cap).
func TestGeneralChatSwitchClosed(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
	}}
	s := startChannelWithSwitches(t, Config{Count: 1}, fs, map[string]int{"玩家聊天开关": 1})
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1)
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 2 })

	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, "大家好", 3)

	notice := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSERVERMESSAGE), opOf(notice))
	assert.Equal(t, serverNoticeBytes(1, "管理员从后台关闭了聊天功能"), notice)
	expectSilence(t, connA) // the chatter gets no CHATTEXT of its own
	expectSilence(t, connB) // and the map hears nothing either
}

// TestGeneralChatSwitchOpen pins the regression case: with 玩家聊天开关 = 0
// (the value the real 079-max2 database carries) chat behaves exactly as
// before, both on the wire and in the cached map.
func TestGeneralChatSwitchOpen(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
	}}
	s := startChannelWithSwitches(t, Config{Count: 1}, fs, map[string]int{"玩家聊天开关": 0})
	cs := s.Channel(1)
	assert.Equal(t, map[string]int{"玩家聊天开关": 0}, s.ConfigValues())

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1)
	waitFor(t, func() bool { return cs.Map(100000000).PlayerCount() == 2 })

	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, "大家好", 3)
	self := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(self))
	sr := protocol.NewReader(self[2:])
	assert.Equal(t, int32(31), sr.Int())
	assert.Equal(t, byte(0), sr.Byte())
	assert.Equal(t, "大家好", sr.MapleAsciiString())
	assert.Equal(t, byte(3), sr.Byte())

	allowReads(t, connB)
	seen := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(seen))
	assert.Equal(t, int32(31), protocol.NewReader(seen[2:]).Int())
}

// TestWhisperFindSwitchClosed: 游戏找人开关 > 0 answers whisper modes 5 and 68
// with dropMessage(5, "找人功能被关闭") -> serverNotice type 5 and no find
// reply (Java ChatHandler.Whisper_Find:264, before the name is read). Mode 6
// (send a whisper) is NOT gated by it.
func TestWhisperFindSwitchClosed(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000001, "乙", 0),
	}}
	s := startChannelWithSwitches(t, Config{Count: 1}, fs, map[string]int{"游戏找人开关": 1})
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	// 乙 enters another map: it sees no spawn of 甲 and 甲 sees none of it.
	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 3)

	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 5, "乙")
	notice := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSERVERMESSAGE), opOf(notice))
	assert.Equal(t, serverNoticeBytes(5, "找人功能被关闭"), notice)
	expectSilence(t, connA) // no getFindReplyWithMap

	// Mode 68 is the same Java branch, so it is gated too.
	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 68, "乙")
	notice = readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendSERVERMESSAGE), opOf(notice))
	assert.Equal(t, serverNoticeBytes(5, "找人功能被关闭"), notice)
	expectSilence(t, connA)

	// Mode 6 delivers normally: Java gates only the find branch.
	allowReads(t, connB)
	sendWhisperSend(t, connA, sendA, "乙", "你好")
	msg := readTestReply(t, connB, recvB)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(msg))
	mr := protocol.NewReader(msg[2:])
	assert.Equal(t, byte(0x12), mr.Byte())
	assert.Equal(t, "甲", mr.MapleAsciiString(), "sender name")
	assert.Equal(t, int16(0), mr.Short(), "sender channel 1 -> 0")
	assert.Equal(t, "你好", mr.MapleAsciiString())
	assert.Zero(t, mr.Len())

	allowReads(t, connA)
	reply := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(reply))
	rr := protocol.NewReader(reply[2:])
	assert.Equal(t, byte(0x0A), rr.Byte())
	assert.Equal(t, "乙", rr.MapleAsciiString())
	assert.Equal(t, byte(1), rr.Byte(), "delivered")
	assert.Zero(t, rr.Len())
}

// TestConfigValuesUnwired: a channel server with no configvalues store at all
// (cmd/gms DB-degraded startup, and every pre-P4.5b test) keeps the old
// behaviour - the map is empty and every switch reads 0 = feature on.
func TestConfigValuesUnwired(t *testing.T) {
	fs := &fakeStore{chars: map[int]*database.Character{
		31: namedChar(31, 7, 100000000, "甲", 0),
		32: namedChar(32, 8, 100000000, "乙", 0),
	}}
	s := startChannelWith(t, Config{Count: 1}, fs) // no SetConfigValues
	assert.Empty(t, s.ConfigValues(), "no store wired -> no switches loaded")
	assert.False(t, s.switchOn("玩家聊天开关"))
	assert.False(t, s.switchOn("游戏找人开关"))
	cs := s.Channel(1)

	connA, sendA, recvA := sendLoggedIn(t, cs, 31)
	defer connA.Close()
	drain(t, connA, recvA, 3)

	connB, _, recvB := sendLoggedIn(t, cs, 32)
	defer connB.Close()
	drain(t, connB, recvB, 4)
	drain(t, connA, recvA, 1)

	allowReads(t, connA)
	sendGeneralChat(t, connA, sendA, "大家好", 3)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(readTestReply(t, connA, recvA)))
	allowReads(t, connB)
	require.Equal(t, uint16(protocol.SendCHATTEXT), opOf(readTestReply(t, connB, recvB)))

	allowReads(t, connA)
	sendWhisperFind(t, connA, sendA, 5, "乙")
	found := readTestReply(t, connA, recvA)
	require.Equal(t, uint16(protocol.SendWHISPER), opOf(found))
	fr := protocol.NewReader(found[2:])
	assert.Equal(t, byte(9), fr.Byte(), "buddy=false -> 9")
	assert.Equal(t, "乙", fr.MapleAsciiString())
	assert.Equal(t, byte(1), fr.Byte(), "found on this channel")
	assert.Equal(t, int32(100000000), fr.Int())
}

// TestConfigValuesLoadError: a failing (or absent) store must not panic and
// must not switch anything off - a DB hiccup has to leave every gate open,
// which is the Java "row absent reads 0" rule.
func TestConfigValuesLoadError(t *testing.T) {
	fs := &fakeStore{cfgErr: errors.New("db down")}
	s := startChannelWith(t, Config{Count: 1}, fs)
	s.SetConfigValues(fs)

	n, err := s.ReloadConfigValues(context.Background())
	require.Error(t, err)
	assert.Zero(t, n)
	assert.Empty(t, s.ConfigValues(), "a failed load leaves all switches at 0 = on")
	assert.False(t, s.switchOn("玩家聊天开关"))

	s.SetConfigValues(nil)
	n, err = s.ReloadConfigValues(context.Background())
	require.NoError(t, err, "no store wired is a silent no-op")
	assert.Zero(t, n)
	assert.Empty(t, s.ConfigValues())
}
