// Package packet - P4.5 chat packets: the general-chat bubble, the face
// expression and the whisper / find-reply family.
//
// Java sources (see docs/FILETRACK.md):
//   - tools/MaplePacketCreator.getChatText         -> ChatTextPacket
//   - tools/MaplePacketCreator.facialExpression    -> FacialExpressionPacket
//   - tools/MaplePacketCreator.getWhisper          -> WhisperPacket
//   - tools/MaplePacketCreator.getWhisperReply     -> WhisperReplyPacket
//   - tools/MaplePacketCreator.getFindReply        -> FindReplyPacket
//   - tools/MaplePacketCreator.getFindReplyWithMap -> FindReplyWithMapPacket
//   - tools/MaplePacketCreator.serverNotice        -> ServerNoticePacket /
//     DropMessagePacket (P4.5b, the configvalues gate notices)
package packet

import "GMS/internal/protocol"

// ChatTextPacket ports MaplePacketCreator.getChatText(cidfrom, text, whiteBG,
// show): short CHATTEXT + int cid + byte (whiteBG ? 1 : 0) + str text +
// byte show.
//
// Java ChatHandler.GeneralChat passes whiteBG = isGM() (GMs get the white
// bubble) and show = the trailing byte the client sent with the message.
func ChatTextPacket(cid int, text string, whiteBG bool, show int) []byte {
	w := protocol.NewWriter(16 + len(text))
	w.Op(protocol.SendCHATTEXT)
	w.Int(int32(cid))
	w.Bool(whiteBG)
	w.MapleAsciiString(text)
	w.Byte(byte(show))
	return w.Bytes()
}

// FacialExpressionPacket ports MaplePacketCreator.facialExpression(from,
// expression): short FACIAL_EXPRESSION + int cid + int expression. The Java
// source keeps a commented-out writeInt(-1); the packet stays 10 bytes.
func FacialExpressionPacket(cid, expression int) []byte {
	w := protocol.NewWriter(10)
	w.Op(protocol.SendFACIAL_EXPRESSION)
	w.Int(int32(cid))
	w.Int(int32(expression))
	return w.Bytes()
}

// WhisperPacket ports MaplePacketCreator.getWhisper(sender, channel, text):
// short WHISPER + byte 0x12 + str sender + short (channel - 1) + str text.
// It is written to the recipient (ChatHandler.Whisper_Find mode 6).
func WhisperPacket(sender string, channel int, text string) []byte {
	w := protocol.NewWriter(32 + len(text))
	w.Op(protocol.SendWHISPER)
	w.Byte(0x12)
	w.MapleAsciiString(sender)
	w.Short(channel - 1)
	w.MapleAsciiString(text)
	return w.Bytes()
}

// WhisperReplyPacket ports MaplePacketCreator.getWhisperReply(target, reply):
// short WHISPER + byte 0x0A + str target + byte reply (0 = not found,
// 1 = delivered). It goes back to the sender.
func WhisperReplyPacket(target string, reply byte) []byte {
	w := protocol.NewWriter(16 + len(target))
	w.Op(protocol.SendWHISPER)
	w.Byte(0x0A)
	w.MapleAsciiString(target)
	w.Byte(reply)
	return w.Bytes()
}

// FindReplyPacket ports MaplePacketCreator.getFindReply(target, channel,
// buddy): short WHISPER + byte (buddy ? 72 : 9) + str target + byte 3 +
// int (channel - 1) - "the character is online on another channel".
func FindReplyPacket(target string, channel int, buddy bool) []byte {
	w := protocol.NewWriter(24 + len(target))
	w.Op(protocol.SendWHISPER)
	w.Byte(findReplyMode(buddy))
	w.MapleAsciiString(target)
	w.Byte(3)
	w.Int(int32(channel - 1))
	return w.Bytes()
}

// FindReplyWithMapPacket ports MaplePacketCreator.getFindReplyWithMap(target,
// mapid, buddy): short WHISPER + byte (buddy ? 72 : 9) + str target + byte 1 +
// int mapid + 8 zero bytes - "the character is online on this channel, on
// this map".
func FindReplyWithMapPacket(target string, mapID int, buddy bool) []byte {
	w := protocol.NewWriter(32 + len(target))
	w.Op(protocol.SendWHISPER)
	w.Byte(findReplyMode(buddy))
	w.MapleAsciiString(target)
	w.Byte(1)
	w.Int(int32(mapID))
	w.Zero(8)
	return w.Bytes()
}

// findReplyMode is the Java `write(buddy ? 72 : 9)` marker of the find-reply
// family (72 = buddy-list variant).
func findReplyMode(buddy bool) byte {
	if buddy {
		return 72
	}
	return 9
}

// serverNoticeChat is the `type` byte of MaplePacketCreator.serverNotice(1,
// message) - the plain popup ChatHandler.GeneralChat sends when the admin
// console has switched public chat off. It is NOT the type-4 rolling banner of
// ServerMessagePacket.
const serverNoticeChat = 1

// ServerNoticePacket ports MaplePacketCreator.serverNotice(1, message) ->
// serverMessage(1, 0, message, false). Layout:
//
//	short SERVERMESSAGE (0x41) | byte 1 | short len | GB18030 message
//
// Java: chat gate ChatHandler.GeneralChat:44
// (`c.sendPacket(MaplePacketCreator.serverNotice(1, "管理员从后台关闭了聊天功能"))`).
// Type 1 is not one of the smega types, so nothing follows the string.
func ServerNoticePacket(message string) []byte {
	return serverNoticePacket(serverNoticeChat, message)
}

// DropMessagePacket ports client/MapleCharacter.dropMessage(type, message):
//
//	if (type == -2) { client.sendPacket(PlayerShopPacket.shopChat(message, 0)); }
//	else            { client.sendPacket(MaplePacketCreator.serverNotice(type, message)); }
//
// Only the serverNotice branch is ported - the player-shop bubble (type -2)
// has no Go counterpart yet. ChatHandler.Whisper_Find calls it with type 5
// (the pink full-width notice) for the 找人功能被关闭 gate, which produces the
// same layout as ServerNoticePacket with a 5 in the type byte.
func DropMessagePacket(kind int, message string) []byte {
	return serverNoticePacket(kind, message)
}

// serverNoticePacket ports the private MaplePacketCreator.serverMessage(type,
// channel, message, megaEar), which every serverNotice overload funnels into.
// The body is:
//
//	short SERVERMESSAGE | byte type | [byte 1 when type == 4] |
//	short len | GB18030 message | type-specific trailer
//
// Deferred: the trailer of the smega family - byte (channel-1) + byte megaEar
// for types 3/9/10/11/12, and int (1000000 <= channel < 6000000 ? channel : 0)
// for types 6/18. The Go port has no smega broadcast yet (P7) and the two
// call sites above use types 1 and 5, which Java leaves trailer-less; channel
// and megaEar are therefore not parameters here.
func serverNoticePacket(kind int, message string) []byte {
	w := protocol.NewWriter(32 + len(message))
	w.Op(protocol.SendSERVERMESSAGE)
	w.IntAsByte(kind)
	if kind == 4 { // Java: if (type == 4) mplew.write(1);
		w.Byte(1)
	}
	w.MapleAsciiString(message)
	return w.Bytes()
}
