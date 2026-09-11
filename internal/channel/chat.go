// Package channel - P4.5 chat: GENERAL_CHAT (0x2D, the public chat bubble),
// FACE_EXPRESSION (0x2F) and WHISPER (0x75, the whisper / find-character
// family).
//
// Java sources (see docs/FILETRACK.md):
//   - handling/MapleServerHandler case GENERAL_CHAT / FACE_EXPRESSION / WHISPER
//   - handling/channel/handler/ChatHandler.GeneralChat / Whisper_Find
//   - handling/channel/handler/PlayerHandler.ChangeEmotion
package channel

import (
	"unicode/utf16"

	"GMS/internal/netw"
	"GMS/internal/packet"
	"GMS/internal/protocol"
)

// chatMaxLen is ChatHandler.GeneralChat's cap for non-GM chatter:
// `!chr2.isGM() && text.length() >= 80` -> drop. Java's String.length() counts
// UTF-16 code units, so the Go port measures the string the same way (a
// surrogate pair counts as two).
const chatMaxLen = 80

// whisper modes of ChatHandler.Whisper_Find (the leading byte).
const (
	whisperFind       = 5  // find a character (find-reply with map/channel)
	whisperFindBuddy  = 68 // same, buddy-list flavour (reply marker 72)
	whisperSend       = 6  // deliver a whisper
	whisperReplyFound = 1  // getWhisperReply "delivered"
	whisperReplyNone  = 0  // getWhisperReply "character not found"
)

// handleGeneralChat ports ChatHandler.GeneralChat.
//
// Order kept:
//
//	text = readMapleAsciiString(); unk = readByte()
//	if (Start.ConfigValuesMap.get("玩家聊天开关") > 0) return   // + serverNotice(1, ...)
//	if (!isGM() && text.length() >= 80) return
//	map.broadcastMessage(getChatText(cid, text, isGM(), unk), player.getPosition())
//
// Skipped (documented):
//   - CommandProcessor.processCommand (玩家指令) - the GM command processor is
//     P7; until then every line is chat.
//   - the hidden-GM broadcastGMMessage branch, the 仙人模式 / 雪球赛 / 彩旦
//     activity hooks and the chat-log file dump - none of those subsystems
//     exist in the Go port yet.
//
// The broadcast is the Point overload, so it is view-range filtered
// (maxViewRangeSq) and the chatter receives its own line (Java source=null).
func (h channelHandler) handleGeneralChat(s *netw.Session, r *protocol.Reader) {
	text := r.MapleAsciiString()
	unk := int(r.Byte())
	if r.Err != nil {
		h.cs.log.Debug("malformed GENERAL_CHAT", "err", r.Err, "remote", s.RemoteAddr)
		return
	}
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	// P4.5b: the 玩家聊天开关 gate, first thing after the character is known
	// (ChatHandler.GeneralChat:44). It silences public chat only - whispers
	// are a different handler and stay open.
	if h.cs.srv.switchOn("玩家聊天开关") {
		p.SendPacket(packet.ServerNoticePacket("管理员从后台关闭了聊天功能"))
		return
	}
	if !p.IsGM() && utf16Len(text) >= chatMaxLen {
		return
	}
	m := h.cs.lookupMap(p.MapID())
	if m == nil {
		return
	}
	x, y := p.Position()
	m.BroadcastRanged(packet.ChatTextPacket(p.ID, text, p.IsGM(), unk), x, y)
	// 2026-09-12 (GMS-P4.5b) - the per-player chat log is deliberately NOT
	// ported. Java writes it right here, gated the other way round (<= 0 =
	// logging on):
	//	if (Start.ConfigValuesMap.get("聊天记录开关") <= 0) {
	//	    FileoutputUtil.logToFile("服务端记录信息/玩家档案" + name + "/聊天记录.txt", ...);
	//	}
	// FileoutputUtil and the 服务端记录信息/ tree have no Go counterpart yet;
	// the 聊天记录开关 row is loaded into the switch map but stays unused
	// until that ops subsystem lands.
}

// handleFaceExpression ports PlayerHandler.ChangeEmotion(the emote int).
//
// Java: an emote > 7 needs the matching cash expression item
// (5159992 + emote) in the inventory, otherwise it is reported as a
// cheating offense and dropped. The inventory arrives with P5.2, so the port
// drops emote > 7 instead of granting effects the character does not own.
//
// The broadcast is the boolean overload - unlimited range, source excluded.
func (h channelHandler) handleFaceExpression(s *netw.Session, r *protocol.Reader) {
	emote := int(r.Int())
	if r.Err != nil {
		h.cs.log.Debug("malformed FACE_EXPRESSION", "err", r.Err, "remote", s.RemoteAddr)
		return
	}
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	if emote <= 0 || emote > 7 {
		return
	}
	p := c.player
	if m := h.cs.lookupMap(p.MapID()); m != nil {
		m.Broadcast(packet.FacialExpressionPacket(p.ID, emote), p)
	}
}

// handleWhisper ports ChatHandler.Whisper_Find: the leading byte selects
// "find a character" (5 / 68) or "send a whisper" (6).
//
// Skipped: the 禁言 (getCanTalk) gate - there is no mute system yet.
func (h channelHandler) handleWhisper(s *netw.Session, r *protocol.Reader) {
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	mode := int(r.Byte())
	if r.Err != nil {
		h.cs.log.Debug("malformed WHISPER", "err", r.Err, "remote", s.RemoteAddr)
		return
	}
	switch mode {
	case whisperFind, whisperFindBuddy:
		// P4.5b: 游戏找人开关, placed exactly where Java has it - inside the
		// 5/68 branch and *before* the recipient name is read
		// (ChatHandler.Whisper_Find:264, before `readMapleAsciiString`), so a
		// server that closed the feature never parses the name. Mode 6 (send a
		// whisper) is not gated by this switch, matching Java.
		if h.cs.srv.switchOn("游戏找人开关") {
			p.SendPacket(packet.DropMessagePacket(5, "找人功能被关闭"))
			return
		}
		name := r.MapleAsciiString()
		if r.Err != nil {
			return
		}
		h.whisperFind(c, name, mode == whisperFindBuddy)
	case whisperSend:
		name := r.MapleAsciiString()
		text := r.MapleAsciiString()
		if r.Err != nil {
			return
		}
		h.whisperSend(c, name, text)
	}
}

// whisperFind ports the 5/68 branch: the target is looked up on this channel
// first (Java PlayerStorage.getCharacterByName) and then through World.Find.
// A GM target is only revealed to a GM requester.
func (h channelHandler) whisperFind(c *client, name string, buddy bool) {
	p := c.player
	if t := h.cs.players.GetCharacterByName(name); t != nil {
		if canWhisper(p, t) {
			p.SendPacket(packet.FindReplyWithMapPacket(t.Name, t.MapID(), buddy))
		} else {
			p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyNone))
		}
		return
	}
	if e, ok := h.cs.srv.find.FindByName(name); ok {
		// Java: ChannelServer.getInstance(ch) ... if (player == null) break -
		// a stale registry entry produces no reply at all.
		t := h.cs.srv.playerOnChannel(name, e.Channel)
		if t == nil {
			return
		}
		if canWhisper(p, t) {
			p.SendPacket(packet.FindReplyPacket(name, e.Channel, buddy))
		} else {
			p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyNone))
		}
		return
	}
	// Java also answers for the cash shop (-10) and the MTS (-20); neither
	// exists in the Go server, so the branch falls through to "not found".
	p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyNone))
}

// whisperSend ports the mode 6 branch. The lookup goes through World.Find (the
// Java branch does not consult the local storage), the recipient gets
// getWhisper and the sender getWhisperReply - reply 0 also when a non-GM tries
// to whisper a GM (the Java "kill the reply" rule).
func (h channelHandler) whisperSend(c *client, name, text string) {
	p := c.player
	e, ok := h.cs.srv.find.FindByName(name)
	if !ok {
		p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyNone))
		return
	}
	t := h.cs.srv.playerOnChannel(name, e.Channel)
	if t == nil {
		return // Java: player == null -> break, no reply
	}
	t.SendPacket(packet.WhisperPacket(p.Name, h.cs.channel, text))
	if !p.IsGM() && t.IsGM() {
		p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyNone))
		return
	}
	p.SendPacket(packet.WhisperReplyPacket(name, whisperReplyFound))
}

// canWhisper ports the Java guard `!player.isGM() || c.getPlayer().isGM() &&
// player.isGM()`: a plain player never learns whether a GM is online.
func canWhisper(from, to *Player) bool {
	if !to.IsGM() {
		return true
	}
	return from.IsGM()
}

// playerOnChannel returns the online player with that name on a 1-based
// channel (Java ChannelServer.getInstance(ch).getPlayerStorage()
// .getCharacterByName(name)), or nil.
func (s *Server) playerOnChannel(name string, channel int) *Player {
	cs := s.Channel(channel)
	if cs == nil {
		return nil
	}
	return cs.Players().GetCharacterByName(name)
}

// utf16Len counts UTF-16 code units, matching Java String.length().
func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }
