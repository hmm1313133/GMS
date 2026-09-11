package packet

// P4.2 tests: the WARP_TO_MAP (getCharInfo) layout - decoded field by field so
// the whole body is consumed - plus the CRand32 connectData quirk (three
// identical ints) and the small reset/banner packets.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/database"
	"GMS/internal/movement"
	"GMS/internal/protocol"
)

func testCharacter() *database.Character {
	return &database.Character{
		ID: 31, AccountID: 7, World: 0, Name: "测试者",
		Level: 1, Exp: 0, Str: 12, Dex: 5, Luk: 4, Int: 4,
		HP: 50, MP: 50, MaxHP: 50, MaxMP: 50, Meso: 1234,
		Job: 0, SkinColor: 0, Gender: 1, Fame: 0,
		Hair: 30000, Face: 20000, AP: 0,
		Map: 100000000, Spawnpoint: 0,
		BuddyCapacity: 20, BookCover: 0,
	}
}

// trimName strips the zero padding of a fixed 13-byte name field.
func trimName(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return s[:i]
		}
	}
	return s
}

func TestCharInfoPacketLayout(t *testing.T) {
	c := testCharacter()
	rnd := &RandStream{}
	rnd.seed(0x1234, 0x5678, 0x9ABC)
	b := CharInfoPacket(c, 3, rnd)

	r := protocol.NewReader(b)
	require.Equal(t, uint16(protocol.SendWARP_TO_MAP), uint16(r.Short()))
	assert.Equal(t, 2, int(r.Int()), "channel is 1-based, the wire carries channel-1")
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, 1, int(r.Byte()))
	assert.Equal(t, 1, int(r.Byte()))
	assert.Equal(t, 0, int(r.Short()))

	// CRand.connectData: Java draws three values from the same state, so all
	// three ints are identical (see rand.go).
	r1, r2, r3 := r.Int(), r.Int(), r.Int()
	assert.Equal(t, r1, r2)
	assert.Equal(t, r2, r3)

	// addCharacterInfo header.
	assert.Equal(t, int64(-1), r.Long())
	assert.Equal(t, 0, int(r.Byte()))

	// addCharStats.
	assert.Equal(t, int32(31), r.Int())
	assert.Equal(t, "测试者", trimName(r.AsciiString(13)))
	assert.Equal(t, 1, int(r.Byte())) // gender
	assert.Equal(t, 0, int(r.Byte())) // skin
	assert.Equal(t, int32(20000), r.Int())
	assert.Equal(t, int32(30000), r.Int())
	assert.Equal(t, make([]byte, 24), r.Read(24))
	assert.Equal(t, 1, int(r.Byte()))  // level
	assert.Equal(t, 0, int(r.Short())) // job
	for _, want := range []int{12, 5, 4, 4, 50, 50, 50, 50, 0, 0} {
		assert.Equal(t, want, int(r.Short()), "stats block (str..sp)")
	}
	assert.Equal(t, int32(0), r.Int()) // exp
	assert.Equal(t, 0, int(r.Short())) // fame
	assert.Equal(t, int32(0), r.Int()) // gachapon exp
	assert.NotZero(t, r.Long())        // FT timestamp
	assert.Equal(t, int32(100000000), r.Int())
	assert.Equal(t, 0, int(r.Byte())) // spawnpoint

	// addCharacterInfo tail.
	assert.Equal(t, 20, int(r.Byte()), "buddy capacity")
	assert.Equal(t, 1, int(r.Byte()), "bless marker")

	// addInventoryInfo.
	assert.Equal(t, "测试者", r.MapleAsciiString())
	assert.Equal(t, int32(1234), r.Int()) // meso
	assert.Equal(t, int32(31), r.Int())   // character id again
	assert.Equal(t, int32(0), r.Int())    // beans
	assert.Equal(t, int32(0), r.Int())
	for _, slot := range []int{32, 32, 32, 32, 60} {
		assert.Equal(t, slot, int(r.Byte()), "inventory slot limit")
	}
	assert.NotZero(t, r.Long())                 // time
	assert.Equal(t, make([]byte, 7), r.Read(7)) // seven empty-group markers

	// addSkillInfo / addCoolDownInfo / addQuestInfo / addRingInfo.
	for i := 0; i < 8; i++ {
		assert.Equal(t, 0, int(r.Short()), "empty list count %d", i)
	}
	// addRocksInfo: 5 VIP maps + 10 teleport rocks.
	assert.Equal(t, make([]byte, 60), r.Read(60))
	// addMonsterBookInfo + QuestInfoPacket.
	assert.Equal(t, int32(0), r.Int()) // monster book cover
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, 0, int(r.Short())) // cards
	assert.Equal(t, 0, int(r.Short())) // questinfo
	assert.Equal(t, int32(0), r.Int()) // PQ rank
	assert.Equal(t, 0, int(r.Short()))
	assert.NotZero(t, r.Long()) // trailing getTime(now)

	assert.NoError(t, r.Err)
	assert.Zero(t, r.Len(), "the packet must be consumed exactly")
}

func TestSpawnPlayerPacketLayout(t *testing.T) {
	c := testCharacter()
	// P4.4: the spawn carries the live map position/stance.
	b := SpawnPlayerPacket(c, 123, -456, 5)

	r := protocol.NewReader(b)
	require.Equal(t, uint16(protocol.SendSPAWN_PLAYER), uint16(r.Short()))
	assert.Equal(t, int32(31), r.Int())
	assert.Equal(t, 1, int(r.Byte()), "level")
	assert.Equal(t, "测试者", r.MapleAsciiString())
	assert.Equal(t, "", r.MapleAsciiString(), "guild name (none)")
	assert.Equal(t, make([]byte, 6), r.Read(6), "guild look")

	// Fixed marker tail + morph + the two buffmask halves.
	assert.Equal(t, int32(0), r.Int())
	assert.Equal(t, []byte{0x00, 0xE0, 0x1F, 0x00}, r.Read(4))
	assert.Equal(t, int32(0), r.Int(), "morph")
	assert.Equal(t, int32(0), r.Int(), "buffmask >> 32")
	assert.Equal(t, int32(0), r.Int(), "buffmask & 0xFFFFFFFF")
	assert.Equal(t, make([]byte, 6), r.Read(6))

	// CHAR_MAGIC_SPAWN is drawn once and repeated in all eight slots.
	magic := r.Int()
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, 0, int(r.Short()))
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, 0, int(r.Short()))
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, 0, int(r.Short()))
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, magic, r.Int(), "no-monster-riding branch")
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, []byte{0x01, 0x41, 0x9A, 0x70, 0x07}, r.Read(5))
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, 0, int(r.Short()))
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, int32(0), r.Int())
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, int64(0), r.Long())
	assert.Equal(t, 0, int(r.Short()))
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, magic, r.Int())
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, 0, int(r.Short()), "job")

	// addCharLook(mega=false).
	assert.Equal(t, 1, int(r.Byte()), "gender")
	assert.Equal(t, 0, int(r.Byte()), "skin")
	assert.Equal(t, int32(20000), r.Int())
	assert.Equal(t, 1, int(r.Byte()), "mega ? 0 : 1 = 1 for the channel spawn")
	assert.Equal(t, int32(30000), r.Int())
	assert.Equal(t, 0xFF, int(r.Byte()))
	assert.Equal(t, 0xFF, int(r.Byte()))
	for i := 0; i < 4; i++ {
		assert.Equal(t, int32(0), r.Int(), "cWeapon + 3 pet ids")
	}

	assert.Equal(t, int32(0), r.Int(), "cash count")
	assert.Equal(t, int32(0), r.Int(), "item effect")
	assert.Equal(t, int32(0), r.Int())
	assert.Equal(t, int32(-1), r.Int())
	assert.Equal(t, int32(0), r.Int(), "chair")
	x, y := r.Pos()
	assert.Equal(t, int16(123), x, "spawn position x")
	assert.Equal(t, int16(-456), y, "spawn position y")
	assert.Equal(t, 5, int(r.Byte()), "stance")
	assert.Equal(t, 0, int(r.Short()), "foothold is hardcoded 0 in Java")
	assert.Equal(t, 0, int(r.Byte()), "pets marker")
	assert.Equal(t, int32(1), r.Int(), "mount level")
	assert.Equal(t, int32(0), r.Int(), "mount exp")
	assert.Equal(t, int32(0), r.Int(), "mount fatigue")
	assert.Equal(t, 0, int(r.Byte()), "announce box")
	assert.Equal(t, 0, int(r.Byte()), "chalkboard")
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, int32(0), r.Int())
	assert.Equal(t, 0, int(r.Byte()))
	assert.Equal(t, int32(0), r.Int())
	assert.Equal(t, 0, int(r.Byte()), "marriage ring look")
	assert.Equal(t, 0, int(r.Short()))

	assert.NoError(t, r.Err)
	assert.Zero(t, r.Len(), "the packet must be consumed exactly")
}

func TestRemovePlayerFromMapPacket(t *testing.T) {
	b := RemovePlayerFromMapPacket(31)
	assert.Equal(t, []byte{0xA3, 0x00, 0x1F, 0x00, 0x00, 0x00}, b)
	assert.Equal(t, uint16(protocol.SendREMOVE_PLAYER_FROM_MAP), uint16(b[0])|uint16(b[1])<<8)
}

func TestMovePlayerPacketLayout(t *testing.T) {
	moves := []movement.Fragment{{
		Type: 0, Pos: &movement.Point{X: 10, Y: 20}, Wobble: &movement.Point{X: 1, Y: 2},
		Unk: 3, NewState: 4, Duration: 100,
	}}
	b := MovePlayerPacket(31, moves)

	r := protocol.NewReader(b)
	require.Equal(t, uint16(protocol.SendMOVE_PLAYER), uint16(r.Short()))
	assert.Equal(t, int32(31), r.Int())
	assert.Equal(t, int32(0), r.Int(), "Java writes a literal 0 where startPos used to be")
	assert.Equal(t, 1, int(r.Byte()), "movement count")
	assert.Equal(t, 0, int(r.Byte()), "command type")
	x, y := r.Pos()
	assert.Equal(t, int16(10), x)
	assert.Equal(t, int16(20), y)
	xw, yw := r.Pos()
	assert.Equal(t, int16(1), xw)
	assert.Equal(t, int16(2), yw)
	assert.Equal(t, 3, int(r.Short()), "unk")
	assert.Equal(t, 4, int(r.Byte()), "newstate")
	assert.Equal(t, 100, int(r.Short()), "duration")
	assert.NoError(t, r.Err)
	assert.Zero(t, r.Len(), "the packet must be consumed exactly")
}

func TestRandStreamConnectDataIsDeterministic(t *testing.T) {
	a, b := &RandStream{}, &RandStream{}
	a.seed(1, 2, 3)
	b.seed(1, 2, 3)

	wa, wb := protocol.NewWriter(16), protocol.NewWriter(16)
	a.ConnectData(wa)
	b.ConnectData(wb)
	assert.Equal(t, wa.Bytes(), wb.Bytes(), "same seeds -> same connectData bytes")

	// Three identical ints, and the re-seed makes the stream reusable.
	r := protocol.NewReader(wa.Bytes())
	v1, v2, v3 := r.Int(), r.Int(), r.Int()
	assert.Equal(t, v1, v2)
	assert.Equal(t, v2, v3)

	// Calling connectData again produces a different group (the stream moved).
	wc := protocol.NewWriter(16)
	a.ConnectData(wc)
	assert.NotEqual(t, wa.Bytes(), wc.Bytes())
}

func TestTemporaryStatsResetAndServerMessage(t *testing.T) {
	reset := TemporaryStatsResetPacket()
	assert.Equal(t, []byte{0x26, 0x00}, reset, "TEMPORARY_STAT_RESET carries no body")

	banner := ServerMessagePacket("欢迎")
	assert.Equal(t, uint16(protocol.SendSERVERMESSAGE), uint16(banner[0])|uint16(banner[1])<<8)
	assert.Equal(t, byte(4), banner[2], "type 4 = scrolling top banner")
	assert.Equal(t, byte(1), banner[3])
	assert.Equal(t, "欢迎", protocol.NewReader(banner[4:]).MapleAsciiString())
}

// TestServerNoticeAndDropMessagePackets pins the P4.5b notice layout against
// literal bytes: short SERVERMESSAGE (0x41 LE) + byte type + short length +
// GB18030 message, built here without the production helper.
func TestServerNoticeAndDropMessagePackets(t *testing.T) {
	// serverNotice(1, ...) - ChatHandler.GeneralChat:44.
	msg := "管理员从后台关闭了聊天功能"
	enc := protocol.EncodeGB18030(msg)
	want := append([]byte{0x41, 0x00, 0x01, byte(len(enc)), byte(len(enc) >> 8)}, enc...)
	assert.Equal(t, want, ServerNoticePacket(msg))

	// dropMessage(5, ...) -> serverNotice(5, ...) - Whisper_Find:265.
	msg5 := "找人功能被关闭"
	enc5 := protocol.EncodeGB18030(msg5)
	want5 := append([]byte{0x41, 0x00, 0x05, byte(len(enc5)), byte(len(enc5) >> 8)}, enc5...)
	assert.Equal(t, want5, DropMessagePacket(5, msg5))

	// Type 1 is not the type-4 banner: no extra byte before the string.
	assert.Equal(t, byte(1), ServerNoticePacket("x")[2])
	assert.NotEqual(t, ServerMessagePacket("x"), ServerNoticePacket("x"))
}

func TestChatTextAndFacialExpressionPackets(t *testing.T) {
	// P4.5: getChatText(cid, text, whiteBG, show).
	chat := ChatTextPacket(31, "你好", true, 7)
	r := protocol.NewReader(chat)
	assert.Equal(t, int16(protocol.SendCHATTEXT), r.Short())
	assert.Equal(t, int32(31), r.Int())
	assert.Equal(t, byte(1), r.Byte(), "whiteBG = isGM()")
	assert.Equal(t, "你好", r.MapleAsciiString())
	assert.Equal(t, byte(7), r.Byte(), "the client's trailing byte")
	assert.Zero(t, r.Len())

	plain := ChatTextPacket(31, "hi", false, 0)
	assert.Equal(t, byte(0), plain[6], "whiteBG=false writes 0")

	// facialExpression(from, expression): cid + int expression.
	expr := FacialExpressionPacket(31, 5)
	er := protocol.NewReader(expr)
	assert.Equal(t, int16(protocol.SendFACIAL_EXPRESSION), er.Short())
	assert.Equal(t, int32(31), er.Int())
	assert.Equal(t, int32(5), er.Int())
	assert.Zero(t, er.Len())
}

func TestWhisperPackets(t *testing.T) {
	// getWhisper(sender, channel, text) - the channel is written as channel-1.
	msg := WhisperPacket("甲", 2, "在吗")
	mr := protocol.NewReader(msg)
	assert.Equal(t, int16(protocol.SendWHISPER), mr.Short())
	assert.Equal(t, byte(0x12), mr.Byte())
	assert.Equal(t, "甲", mr.MapleAsciiString())
	assert.Equal(t, int16(1), mr.Short())
	assert.Equal(t, "在吗", mr.MapleAsciiString())
	assert.Zero(t, mr.Len())

	// getWhisperReply(target, reply).
	rep := WhisperReplyPacket("乙", 1)
	rr := protocol.NewReader(rep)
	assert.Equal(t, int16(protocol.SendWHISPER), rr.Short())
	assert.Equal(t, byte(0x0A), rr.Byte())
	assert.Equal(t, "乙", rr.MapleAsciiString())
	assert.Equal(t, byte(1), rr.Byte())
	assert.Zero(t, rr.Len())

	// getFindReply(target, channel, buddy): buddy flips the marker 9 -> 72.
	find := FindReplyPacket("乙", 3, true)
	fr := protocol.NewReader(find)
	assert.Equal(t, int16(protocol.SendWHISPER), fr.Short())
	assert.Equal(t, byte(72), fr.Byte())
	assert.Equal(t, "乙", fr.MapleAsciiString())
	assert.Equal(t, byte(3), fr.Byte())
	assert.Equal(t, int32(2), fr.Int(), "channel-1")
	assert.Zero(t, fr.Len())
	assert.Equal(t, byte(9), FindReplyPacket("乙", 3, false)[2])

	// getFindReplyWithMap(target, mapid, buddy): marker 1 + map id + 8 zeros.
	withMap := FindReplyWithMapPacket("丙", 100000000, false)
	wmr := protocol.NewReader(withMap)
	assert.Equal(t, int16(protocol.SendWHISPER), wmr.Short())
	assert.Equal(t, byte(9), wmr.Byte())
	assert.Equal(t, "丙", wmr.MapleAsciiString())
	assert.Equal(t, byte(1), wmr.Byte())
	assert.Equal(t, int32(100000000), wmr.Int())
	assert.Equal(t, 8, wmr.Len())
	assert.Equal(t, make([]byte, 8), wmr.Read(8))
	assert.Zero(t, wmr.Len())
}
