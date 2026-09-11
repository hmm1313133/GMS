// Package login - packet builders for the v079 login handshake (subset for
// P1 smoke; P2 expands to the full LoginPacket.java port).
package login

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"GMS/internal/database"
	"GMS/internal/packet"
	"GMS/internal/protocol"
)

// HelloPacket ports Java LoginPacket.getHello(mapleVersion, ivSend, ivRecv):
//
//	writeShort(13); writeShort(version); write(0,0);
//	write(recvIv); write(sendIv); write(4)
//
// (The 13 is a legacy length field; body is 15 bytes.)
func HelloPacket(version int, sendIV, recvIV [4]byte) []byte {
	w := protocol.NewWriter(16)
	w.Short(13)
	w.Short(version)
	w.Byte(0)
	w.Byte(0)
	w.Write(recvIV[:])
	w.Write(sendIV[:])
	w.Byte(4)
	return w.Bytes()
}

// PingPacket ports Java LoginPacket.getPing: short PING opcode, no body.
func PingPacket() []byte {
	w := protocol.NewWriter(2)
	w.Op(protocol.SendPING)
	return w.Bytes()
}

// LoginFailedPacket ports Java LoginPacket.getLoginFailed(reason):
//
//	writeShort(LOGIN_STATUS); writeInt(reason); writeShort(0)
//
// Reason codes (Java MapleClient.login): 3=banned, 4=wrong password,
// 5=no such account, 7=already logged in (double login).
func LoginFailedPacket(reason int) []byte {
	w := protocol.NewWriter(8)
	w.Op(protocol.SendLOGIN_STATUS)
	w.Int(int32(reason))
	w.Short(0)
	return w.Bytes()
}

// authTail is the fixed 15-byte blob Java writes after the account name in
// getAuthSuccessRequest ("00 00 00 03 01 00 00 00 E2 ED A3 7A FA C9 01" -
// v079 client protocol constant, meaning unknown/fixed).
var authTail = []byte{
	0x00, 0x00, 0x00, 0x03, 0x01, 0x00, 0x00, 0x00,
	0xE2, 0xED, 0xA3, 0x7A, 0xFA, 0xC9, 0x01,
}

// AuthSuccessPacket ports Java LoginPacket.getAuthSuccessRequest(client):
//
//	writeShort(LOGIN_STATUS); write(0); writeInt(accID); write(gender);
//	writeShort(gm ? 1 : 0); writeMapleAsciiString(name);
//	write(authTail); writeInt(0); writeLong(0);
//	writeMapleAsciiString(accID); writeMapleAsciiString(name); write(1)
func AuthSuccessPacket(accID int, gender byte, gm bool, accountName string) []byte {
	w := protocol.NewWriter(64)
	w.Op(protocol.SendLOGIN_STATUS)
	w.Byte(0) // 0 = success (failures carry an int reason instead)
	w.Int(int32(accID))
	w.Byte(gender)
	if gm {
		w.Short(1)
	} else {
		w.Short(0)
	}
	w.MapleAsciiString(accountName)
	w.Write(authTail)
	w.Int(0)
	w.Long(0)
	w.MapleAsciiString(strconv.Itoa(accID))
	w.MapleAsciiString(accountName)
	w.Byte(1)
	return w.Bytes()
}

// GenderNeededPacket ports Java LoginPacket.getGenderNeeded(client):
//
//	writeShort(CHOOSE_GENDER); writeMapleAsciiString(accountName)
//
// Sent instead of auth success when the account has gender 10 (unset) - the
// accounts table DEFAULT gender=10, so fresh accounts take this path
// (Java LoginWorker.registerClient).
func GenderNeededPacket(accountName string) []byte {
	w := protocol.NewWriter(32)
	w.Op(protocol.SendCHOOSE_GENDER)
	w.MapleAsciiString(accountName)
	return w.Bytes()
}

var _ = binary.LittleEndian

// ServerListPacket ports Java LoginPacket.getServerList (LoginPacket.java:158):
//
//	writeShort(SERVERLIST); write(serverId); writeMapleAsciiString(serverName);
//	write(state); writeMapleAsciiString(eventMessage); writeShort(100); writeShort(100);
//	write(lastChannel); writeInt(500);
//	for i := 1; i <= lastChannel; i++ {
//	    writeMapleAsciiString(serverName + "-" + i); writeInt(load or 1200);
//	    write(serverId); writeShort(i - 1)
//	}
//	writeShort(balloonCount); per balloon: writeShort(x); writeShort(y);
//	    writeMapleAsciiString(message)
//
// lastChannel is the highest channel id <= 30 present in channelLoad
// (default 1); channels missing from the map report load 1200 (full).
func ServerListPacket(serverID int, serverName string, state int, eventMessage string, channelLoad map[int]int, balloons []Balloon) []byte {
	w := protocol.NewWriter(96)
	w.Op(protocol.SendSERVERLIST)
	w.IntAsByte(serverID)
	w.MapleAsciiString(serverName)
	w.IntAsByte(state)
	w.MapleAsciiString(eventMessage)
	w.Short(100)
	w.Short(100)
	lastChannel := 1
	for i := 30; i > 0; i-- {
		if _, ok := channelLoad[i]; ok {
			lastChannel = i
			break
		}
	}
	w.IntAsByte(lastChannel)
	w.Int(500)
	for i := 1; i <= lastChannel; i++ {
		load := 1200
		if v, ok := channelLoad[i]; ok {
			load = v
		}
		w.MapleAsciiString(fmt.Sprintf("%s-%d", serverName, i))
		w.Int(int32(load))
		w.IntAsByte(serverID)
		w.Short(i - 1)
	}
	w.Short(len(balloons))
	for _, b := range balloons {
		w.Short(b.X)
		w.Short(b.Y)
		w.MapleAsciiString(b.Message)
	}
	return w.Bytes()
}

// EndOfServerListPacket ports Java LoginPacket.getEndOfServerList:
// short SERVERLIST + byte 255.
func EndOfServerListPacket() []byte {
	w := protocol.NewWriter(4)
	w.Op(protocol.SendSERVERLIST)
	w.IntAsByte(0xFF)
	return w.Bytes()
}

// ServerStatusPacket ports Java LoginPacket.getServerStatus:
// short SERVERSTATUS + short status (0 normal, 1 busy, 2 full).
func ServerStatusPacket(status int) []byte {
	w := protocol.NewWriter(4)
	w.Op(protocol.SendSERVERSTATUS)
	w.Short(status)
	return w.Bytes()
}

// ---- P2.4: character list / create / delete / gender packets ----
// Java sources: LoginPacket.getCharList / addCharEntry / addNewCharEntry /
// charNameResponse / deleteCharResponse / getGenderChanged +
// PacketHelper.addCharStats (now shared, see internal/packet) / addCharLook,
// and MaplePacketCreator.serverNotice (the login "notice dialog" replies).

// addCharLook ports PacketHelper.addCharLook(mplew, chr, mega=true,
// channelserver=false) as used by LoginPacket.addCharEntry. The layout now
// lives in packet.AddCharLook (shared with the channel spawn packet); the
// P2.4 simplification stands - the equip maps are empty, so only the two 0xFF
// markers, cWeapon 0 and the three zero pet ids are written.
func addCharLook(w *protocol.Writer, c *database.Character) {
	packet.AddCharLook(w, c, true)
}

// addCharEntry ports LoginPacket.addCharEntry(ranking, viewAll): stats +
// look + trailing byte 0; job 900 (GM) gets an extra byte 2. (The ranking
// parameter is unused in the ZEV decompile - kept faithful.)
func addCharEntry(w *protocol.Writer, c *database.Character) {
	packet.AddCharStats(w, c)
	addCharLook(w, c)
	w.Byte(0)
	if c.Job == 900 {
		w.Byte(2)
	}
}

// CharListPacket ports LoginPacket.getCharList: short CHARLIST + byte 0 +
// int 0 + byte count + per-char addCharEntry + short 3 + int charslots.
func CharListPacket(chars []database.Character, charslots int) []byte {
	w := protocol.NewWriter(32 + 128*len(chars))
	w.Op(protocol.SendCHARLIST)
	w.Byte(0)
	w.Int(0)
	w.IntAsByte(len(chars))
	for i := range chars {
		addCharEntry(w, &chars[i])
	}
	w.Short(3)
	w.Int(int32(charslots))
	return w.Bytes()
}

// CharNameResponsePacket ports LoginPacket.charNameResponse:
// short CHAR_NAME_RESPONSE + str name + byte (nameUsed ? 1 : 0).
func CharNameResponsePacket(name string, used bool) []byte {
	w := protocol.NewWriter(32)
	w.Op(protocol.SendCHAR_NAME_RESPONSE)
	w.MapleAsciiString(name)
	w.Bool(used)
	return w.Bytes()
}

// AddNewCharEntryPacket ports LoginPacket.addNewCharEntry:
// short ADD_NEW_CHAR_ENTRY + byte (worked ? 0 : 1) + addCharEntry.
func AddNewCharEntryPacket(c *database.Character, worked bool) []byte {
	w := protocol.NewWriter(160)
	w.Op(protocol.SendADD_NEW_CHAR_ENTRY)
	if worked {
		w.Byte(0)
	} else {
		w.Byte(1)
	}
	addCharEntry(w, c)
	return w.Bytes()
}

// DeleteCharResponsePacket ports LoginPacket.deleteCharResponse:
// short DELETE_CHAR_RESPONSE + int cid + byte state (0 ok, 1 not found
// / guild leader, 16 second-password mismatch).
func DeleteCharResponsePacket(cid, state int) []byte {
	w := protocol.NewWriter(16)
	w.Op(protocol.SendDELETE_CHAR_RESPONSE)
	w.Int(int32(cid))
	w.IntAsByte(state)
	return w.Bytes()
}

// GenderChangedPacket ports LoginPacket.getGenderChanged:
// short GENDER_SET + byte 0 + str accountName + str accID.
func GenderChangedPacket(accountName string, accID int) []byte {
	w := protocol.NewWriter(32)
	w.Op(protocol.SendGENDER_SET)
	w.Byte(0)
	w.MapleAsciiString(accountName)
	w.MapleAsciiString(strconv.Itoa(accID))
	return w.Bytes()
}

// LicenseRequestPacket ports MaplePacketCreator.licenseRequest (ZEV quirk:
// reuses LOGIN_STATUS with byte 22, sent after SET_GENDER).
func LicenseRequestPacket() []byte {
	w := protocol.NewWriter(8)
	w.Op(protocol.SendLOGIN_STATUS)
	w.Byte(22)
	return w.Bytes()
}

// ServerNoticeDialogPacket ports MaplePacketCreator.serverNotice(1, msg)
// (serverMessage type 1 = popup dialog): short SERVERMESSAGE + byte 1 +
// str message. Used by CreateChar failure notices.
func ServerNoticeDialogPacket(message string) []byte {
	w := protocol.NewWriter(32)
	w.Op(protocol.SendSERVERMESSAGE)
	w.Byte(1)
	w.MapleAsciiString(message)
	return w.Bytes()
}

// ServerIPPacket ports Java MaplePacketCreator.getServerIP(port, clientId)
// (P4.1, the CHAR_SELECT reply): short SERVER_IP + short 0 + 4-byte IP +
// short port + int charId + {1, 0, 0, 0, 0}. The client disconnects from the
// login server and reconnects to ip:port (the channel server), where it
// starts with PLAYER_LOGGEDIN.
func ServerIPPacket(ip [4]byte, port, charID int) []byte {
	w := protocol.NewWriter(16)
	w.Op(protocol.SendSERVER_IP)
	w.Short(0)
	w.Write(ip[:])
	w.Short(port)
	w.Int(int32(charID))
	w.Write([]byte{1, 0, 0, 0, 0})
	return w.Bytes()
}

// EnableActionsPacket ports Java MaplePacketCreator.enableActions():
// updatePlayerStats(EMPTY_STATUPDATE, itemReaction=true, 0) degenerates to
// short UPDATE_STATS + byte 1 + int 0 (empty mask) + short 0. The CHAR_SELECT
// guard failure path replies with it.
func EnableActionsPacket() []byte {
	w := protocol.NewWriter(8)
	w.Op(protocol.SendUPDATE_STATS)
	w.Byte(1)
	w.Int(0)
	w.Short(0)
	return w.Bytes()
}
