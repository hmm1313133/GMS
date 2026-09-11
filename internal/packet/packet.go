// Package packet is the Go port of Java's MaplePacketCreator / PacketHelper -
// the send-side packet body builders. P4.2 starts it with the pieces the login
// and channel servers share (character stat serialisation, the channel-enter
// WARP_TO_MAP packet); later phases move the per-domain builders here as they
// are needed (PLAN §4.1 packet/, "MaplePacketCreator 的 Go 版").
//
// Java sources (see docs/FILETRACK.md):
//   - tools/packet/PacketHelper.addCharStats        -> AddCharStats
//   - tools/packet/PacketHelper.addCharLook         -> AddCharLook
//   - tools/packet/PacketHelper.addCharacterInfo    -> addCharacterInfo
//   - tools/packet/PacketHelper.getTime             -> PacketTimeNow
//   - tools/MaplePacketCreator.getCharInfo          -> CharInfoPacket
//   - tools/MaplePacketCreator.temporaryStats_Reset -> TemporaryStatsResetPacket
//   - tools/MaplePacketCreator.serverMessage        -> ServerMessagePacket
//   - tools/MaplePacketCreator.spawnPlayerMapobject -> SpawnPlayerPacket
//   - tools/MaplePacketCreator.removePlayerFromMap  -> RemovePlayerFromMapPacket
//   - tools/MaplePacketCreator.movePlayer           -> MovePlayerPacket
//   - client/PlayerRandomStream                     -> RandStream (rand.go)
package packet

import (
	"math/rand/v2"
	"time"

	"GMS/internal/database"
	"GMS/internal/movement"
	"GMS/internal/protocol"
)

// PacketTimeNow ports PacketHelper.getTime(System.currentTimeMillis()):
// seconds-since-epoch * 10^7 + 116444592000000000 (the Windows FILETIME
// offset, 1970-01-01 in 100 ns ticks).
func PacketTimeNow() int64 {
	const ftUTOffset = 116444592000000000
	return time.Now().Unix()*10000000 + ftUTOffset
}

// inventorySlotDefaults are Java MapleCharacter.saveNewCharToDB's inventoryslot
// row (equip/use/setup/etc = 32, cash = 60). The channelserver load path
// (loadCharFromDB) reads the real row from the inventoryslot table; that table
// is not ported yet (PLAN P5.2), so character-info packets use these defaults.
var inventorySlotDefaults = [5]byte{32, 32, 32, 32, 60}

// AddCharStats ports PacketHelper.addCharStats: int id + 13-byte padded name
// + byte gender + byte skin + int face + int hair + 24 zero bytes + byte
// level + short job + 8 shorts (str/dex/int/luk/hp/maxhp/mp/maxmp,
// PlayerStats.connectData) + short ap + short sp + int exp + short fame +
// int 0 (gachapon exp) + long time + int map + byte spawnpoint.
//
// Shared by the login CHARLIST/ADD_NEW_CHAR_ENTRY entries and the channel
// WARP_TO_MAP (getCharInfo) packet.
func AddCharStats(w *protocol.Writer, c *database.Character) {
	w.Int(int32(c.ID))
	w.AsciiStringMax(c.Name, 13)
	w.IntAsByte(c.Gender)
	w.IntAsByte(c.SkinColor)
	w.Int(int32(c.Face))
	w.Int(int32(c.Hair))
	w.Zero(24)
	w.IntAsByte(c.Level)
	w.Short(c.Job)
	w.Short(c.Str)
	w.Short(c.Dex)
	w.Short(c.Int)
	w.Short(c.Luk)
	w.Short(c.HP)
	w.Short(c.MaxHP)
	w.Short(c.MP)
	w.Short(c.MaxMP)
	w.Short(c.AP)
	w.Short(remainingSP(c))
	w.Int(int32(c.Exp))
	w.Short(c.Fame)
	w.Int(0)
	w.Long(PacketTimeNow())
	w.Int(int32(c.Map))
	w.IntAsByte(c.Spawnpoint)
}

// remainingSP ports MapleCharacter.getRemainingSp (remainingSp[skill book]).
// P2.4/P4.2 simplification: the `sp` column ("0,0,0,..." - one slot per skill
// book) is not parsed yet, job 0 maps to book 0 and every created character
// has 0 there, so the value is always 0. The parse arrives with P5.1.
func remainingSP(c *database.Character) int {
	_ = c
	return 0
}

// CharInfoPacket ports MaplePacketCreator.getCharInfo(chr): the WARP_TO_MAP
// packet the channel server sends right after PLAYER_LOGGEDIN (Java
// InterServerHandler.Loggedin2). The client loads the map and spawns the
// player from it.
//
//	short WARP_TO_MAP + int (channel - 1) + byte 0 + byte 1 + byte 1 +
//	short 0 + CRand.connectData (3 ints) + addCharacterInfo +
//	long getTime(now)
//
// channel is 1-based (the wire value is channel-1).
func CharInfoPacket(c *database.Character, channel int, rnd *RandStream) []byte {
	w := protocol.NewWriter(512)
	w.Op(protocol.SendWARP_TO_MAP)
	w.Int(int32(channel - 1))
	w.Byte(0)
	w.Byte(1)
	w.Byte(1)
	w.Short(0)
	rnd.ConnectData(w)
	addCharacterInfo(w, c)
	w.Long(PacketTimeNow())
	return w.Bytes()
}

// addCharacterInfo ports PacketHelper.addCharacterInfo: long -1 + byte 0 +
// addCharStats + byte buddyCapacity + byte 1 (bless) + inventory + skills +
// cooldowns + quests + rings + rocks + monster book + questinfo + int 0 (PQ
// rank) + short 0.
//
// P4.2 simplification: inventoryitems / skills / cooldowns / queststatus /
// rings / monsterbook / questinfo tables are not ported yet, so every list is
// empty (the Java shape is preserved byte-for-byte).
func addCharacterInfo(w *protocol.Writer, c *database.Character) {
	w.Long(-1)
	w.Byte(0)
	AddCharStats(w, c)
	w.IntAsByte(c.BuddyCapacity) // Java getBuddylist().getCapacity()
	w.Byte(1)                    // bless-of-fairy marker
	addInventoryInfo(w, c)
	w.Short(0) // addSkillInfo: no skills
	w.Short(0) // addCoolDownInfo: no cooldowns
	w.Short(0) // addQuestInfo: started
	w.Short(0) // addQuestInfo: completed
	w.Short(0) // addRingInfo: equip/cash ring counts + "111" marker
	w.Short(0)
	w.Short(0)
	w.Short(0)
	w.Zero(5 * 4)  // addRocksInfo: 5 VIP teleport maps
	w.Zero(10 * 4) // addRocksInfo: 10 teleport rock maps
	w.Int(int32(c.BookCover))
	w.Byte(0)  // addMonsterBookInfo: marker before the card list
	w.Short(0) // addMonsterBookInfo: empty card list
	w.Short(0) // QuestInfoPacket: empty questinfo map
	w.Int(0)   // PQ rank
	w.Short(0)
}

// addInventoryInfo ports PacketHelper.addInventoryInfo with every inventory
// empty: name + meso + id + beans + int 0 + the five slot limits + time +
// the seven "end of section" markers Java writes between the item groups.
func addInventoryInfo(w *protocol.Writer, c *database.Character) {
	w.MapleAsciiString(c.Name)
	w.Int(int32(c.Meso))
	w.Int(int32(c.ID))
	w.Int(0) // beans
	w.Int(0)
	for _, limit := range inventorySlotDefaults {
		w.IntAsByte(int(limit)) // equip/use/setup/etc/cash slot limits
	}
	w.Long(PacketTimeNow())
	// equipped (pos < 0), equipped cash (pos <= -100), equip, use, setup,
	// etc, cash - each an empty group closed by its 0 marker.
	w.Zero(7)
}

// AddCharLook ports PacketHelper.addCharLook(mplew, chr, mega, channelserver):
// byte gender + byte skin + int face + byte (mega ? 0 : 1) + int hair + the
// visible-equip entries closed by 0xFF + the masked entries closed by 0xFF +
// int cWeapon + 3x pet item ids.
//
// The two callers differ only in mega (LoginPacket.addCharEntry passes true,
// spawnPlayerMapobject passes false) and in the pet block (login's
// channelserver=false always writes zeros, the channel path writes the real
// pet ids). P2.4 simplification kept here: the inventory is not ported yet, so
// the equip maps are empty and all pets are 0 - only the markers are written.
func AddCharLook(w *protocol.Writer, c *database.Character, mega bool) {
	w.IntAsByte(c.Gender)
	w.IntAsByte(c.SkinColor)
	w.Int(int32(c.Face))
	if mega {
		w.Byte(0)
	} else {
		w.Byte(1)
	}
	w.Int(int32(c.Hair))
	w.Byte(0xFF) // end of visible items (empty)
	w.Byte(0xFF) // end of masked items (empty)
	w.Int(0)     // cWeapon (slot -111), none equipped
	w.Int(0)     // pet 1
	w.Int(0)     // pet 2
	w.Int(0)     // pet 3
}

// SpawnPlayerPacket ports MaplePacketCreator.spawnPlayerMapobject(chr): the
// SPAWN_PLAYER packet that puts another player on screen. MapleMap pumps it to
// the players already on the map when a character enters, and to the newcomer
// for each character that is already there.
//
// Layout (v079, default new-character state - empty inventory, no guild, no
// buffs, no mount, no pets, no shop, no rings):
//
//	short SPAWN_PLAYER + int cid + byte level + str name
//	str guildName("") + byte[6] guild look
//	int morph(0) + 0x00 0xE0 0x1F 0x00 + int buffmaskHi(0) + int buffmaskLo(0)
//	byte[6] + (int CHAR_MAGIC_SPAWN + long 0 + short 0 + byte 0) x2
//	int CHAR_MAGIC_SPAWN + short 0 + byte 0
//	int CHAR_MAGIC_SPAWN + long 0 + byte 0          (no monster riding)
//	long 0 + int CHAR_MAGIC_SPAWN + 0x00 0x01 0x41 0x9A 0x70 0x07
//	long 0 + int CHAR_MAGIC_SPAWN + long 0 + int 0 + byte 0
//	int CHAR_MAGIC_SPAWN + long 0 + short 0 + byte 0
//	int CHAR_MAGIC_SPAWN + byte 0 + short job
//	addCharLook(mega=false) + int 0 (5110000 count) + int itemEffect(0)
//	int 0 + int -1 + int chair(0) + pos + byte stance(0) + short 0 (foothold)
//	byte 0 (pets) + int mountLevel(1) + int mountExp(0) + int mountFatigue(0)
//	byte 0 (announce box) + byte 0 (chalkboard)
//	addRingInfo x2 (byte 0 + int 0) + addMarriageRingLook (byte 0) + short 0
//
// CHAR_MAGIC_SPAWN is Randomizer.nextInt() - a per-packet tick count, drawn
// once and repeated in every slot (Java comment: it explains the seven dummy
// buffstats). The mount fields come from MapleMount built with level 1 / exp 0
// / fatigue 0 (MapleCharacter.loadCharFromDB:894).
//
// P4.4: the position/stance are the player's live map state (Java
// chr.getPosition() / chr.getStance()); the foothold stays hardcoded 0 as in
// the Java source. Everything else matches the Java byte stream.
func SpawnPlayerPacket(c *database.Character, x, y int16, stance int) []byte {
	w := protocol.NewWriter(256)
	w.Op(protocol.SendSPAWN_PLAYER)
	w.Int(int32(c.ID))
	w.IntAsByte(c.Level)
	w.MapleAsciiString(c.Name)
	// Guild: getGuildId() <= 0 -> empty name + zeroed look (the AriantPQ and
	// guild branches collapse to the same bytes when there is no guild).
	w.MapleAsciiString("")
	w.Zero(6)
	// A fixed 4-byte tail, the morph marker (writeInt(2) only with the MORPH
	// buff) and the two buffmask halves.
	w.Int(0)
	w.Byte(0x00)
	w.Byte(0xE0)
	w.Byte(0x1F)
	w.Byte(0x00)
	w.Int(0) // morph (no MORPH buff)
	w.Int(0) // buffmask >> 32
	// buffvalue: only written when a buff is active (none here)
	w.Int(0) // buffmask & 0xFFFFFFFF
	w.Zero(6)

	magic := int32(rand.Int32()) // CHAR_MAGIC_SPAWN (Randomizer.nextInt)
	// The seven irregular buffstats, each: int tick + long 0 [+ short 0 + byte 0].
	w.Int(magic)
	w.Long(0)
	w.Short(0)
	w.Byte(0)
	w.Int(magic)
	w.Long(0)
	w.Short(0)
	w.Byte(0)
	// Monster riding: short 0 + byte 0 then the "not riding" branch.
	w.Int(magic)
	w.Short(0)
	w.Byte(0)
	w.Int(magic)
	w.Long(0)
	w.Byte(0)
	w.Long(0)
	w.Int(magic)
	w.Byte(0)
	w.Byte(0x01)
	w.Byte(0x41)
	w.Byte(0x9A)
	w.Byte(0x70)
	w.Byte(0x07)
	w.Long(0)
	w.Short(0)
	w.Int(magic)
	w.Long(0)
	w.Int(0)
	w.Byte(0)
	w.Int(magic)
	w.Long(0)
	w.Short(0)
	w.Byte(0)
	w.Int(magic)
	w.Byte(0)
	w.Short(c.Job)

	AddCharLook(w, c, false)
	w.Int(0)  // min(250, cash count of 5110000)
	w.Int(0)  // item effect
	w.Int(0)  // TW/CN only
	w.Int(-1) // TW/CN only
	w.Int(0)  // chair (none)
	w.Pos(x, y)
	w.Byte(byte(stance))
	w.Short(0) // foothold (Java writes a hardcoded 0)
	w.Byte(0)  // pets (Java writes a single zero)
	w.Int(1)   // mount level (MapleCharacter.loadCharFromDB: level 1)
	w.Int(0)   // mount exp
	w.Int(0)   // mount fatigue
	w.Byte(0)  // addAnnounceBox (no player shop)
	w.Byte(0)  // chalkboard (none)
	// addRingInfo(allrings) is called twice, then addMarriageRingLook.
	w.Byte(0)
	w.Int(0)
	w.Byte(0)
	w.Int(0)
	w.Byte(0)  // marriage ring look (none)
	w.Short(0) // carnival/coconut tail
	return w.Bytes()
}

// RemovePlayerFromMapPacket ports MaplePacketCreator.removePlayerFromMap(cid):
// short REMOVE_PLAYER_FROM_MAP + int cid. MapleMap.removePlayer broadcasts it
// to everyone left on the map.
func RemovePlayerFromMapPacket(cid int) []byte {
	w := protocol.NewWriter(8)
	w.Op(protocol.SendREMOVE_PLAYER_FROM_MAP)
	w.Int(int32(cid))
	return w.Bytes()
}

// MovePlayerPacket ports MaplePacketCreator.movePlayer(cid, moves, startPos):
// short MOVE_PLAYER + int cid + int 0 + the movement list. The Java source
// comments the writePos(startPos) out, so the packet carries a literal int 0
// where the start position used to be - the client only reads the moved-to
// positions out of the fragment list.
//
// MapleMap.broadcastMessage(player, movePlayer(...), false) sends it to every
// other player on the map.
func MovePlayerPacket(cid int, moves []movement.Fragment) []byte {
	w := protocol.NewWriter(64)
	w.Op(protocol.SendMOVE_PLAYER)
	w.Int(int32(cid))
	w.Int(0) // Java: mplew.writeInt(0); (writePos(startPos) commented out)
	movement.SerializeMovementList(w, moves)
	return w.Bytes()
}

// TemporaryStatsResetPacket ports MaplePacketCreator.temporaryStats_Reset:
// short TEMP_STATS_RESET, no body. Sent right after getCharInfo.
func TemporaryStatsResetPacket() []byte {
	w := protocol.NewWriter(2)
	w.Op(protocol.SendTEMP_STATS_RESET)
	return w.Bytes()
}

// ServerMessagePacket ports MaplePacketCreator.serverMessage(message) ->
// serverMessage(4, 0, message, true): short SERVERMESSAGE + byte 4 (rolling
// banner) + byte 1 + MapleAsciiString(message). Java ChannelServer.addPlayer
// pushes the configured banner to every player that enters.
func ServerMessagePacket(message string) []byte {
	w := protocol.NewWriter(32 + len(message))
	w.Op(protocol.SendSERVERMESSAGE)
	w.Byte(4) // 4 = scrolling top banner
	w.Byte(1)
	w.MapleAsciiString(message)
	return w.Bytes()
}
