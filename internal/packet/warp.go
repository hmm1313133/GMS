// Package packet - P4.7 warp packets: the in-world map transfer
// (MaplePacketCreator.getWarpToMap) and the empty-mask enableActions reply
// every guard path of the transfer handlers uses.
//
// Java sources (see docs/FILETRACK.md):
//   - tools/MaplePacketCreator.getWarpToMap -> WarpToMapPacket
//   - tools/MaplePacketCreator.enableActions -> EnableActionsPacket
package packet

import "GMS/internal/protocol"

// WarpToMapPacket ports MaplePacketCreator.getWarpToMap(to, spawnPoint, chr):
//
//	short WARP_TO_MAP + int (channel - 1) + byte 0 + byte 3 + short 0 +
//	byte 0 + int to.getId() + byte spawnPoint + short chr.getStat().getHp() +
//	long getTime(now)
//
// Unlike getCharInfo (CharInfoPacket) this packet carries the map id directly
// instead of the whole addCharacterInfo block - it is the transfer within the
// channel server (MapleCharacter.changeMapInternal).
//
// The middle marker is a literal 3 here, where getCharInfo writes 1/1: the v079
// client uses it as the "portable/transfer" flag. spawnPoint is the portal id
// and goes out as a SINGLE byte (Java `mplew.write(spawnPoint)`), so portal ids
// must stay below 256 - the door portals start at 128 for exactly that reason.
func WarpToMapPacket(mapID, spawnPoint, channel, hp int) []byte {
	w := protocol.NewWriter(32)
	w.Op(protocol.SendWARP_TO_MAP)
	w.Int(int32(channel - 1))
	w.Byte(0)
	w.Byte(3)
	w.Short(0)
	w.Byte(0)
	w.Int(int32(mapID))
	w.Byte(byte(spawnPoint))
	w.Short(hp)
	w.Long(PacketTimeNow())
	return w.Bytes()
}

// EnableActionsPacket ports MaplePacketCreator.enableActions():
// updatePlayerStats(EMPTY_STATUPDATE, itemReaction=true, 0) degenerates to
// short UPDATE_STATS + byte 1 + int 0 (empty mask) + short 0.
//
// Every guard path of the warp handlers answers with it (a portal that does not
// transfer, a missing target map, an unmatched transfer request), and the login
// server replies with it when a CHAR_SELECT is not authorized - hence the
// shared home in the packet layer (the login package delegates here).
func EnableActionsPacket() []byte {
	w := protocol.NewWriter(8)
	w.Op(protocol.SendUPDATE_STATS)
	w.Byte(1)
	w.Int(0)
	w.Short(0)
	return w.Bytes()
}
