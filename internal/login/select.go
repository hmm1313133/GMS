// Package login - P4.1 character select: the login -> channel handoff.
//
// Java sources (see docs/FILETRACK.md):
//   - CharLoginHandler.Character_WithoutSecondPassword -> handleCharSelect
//     (RecvCHAR_SELECT 0x000A)
//
// The Java AUTH_SECOND_PASSWORD variant (Character_WithSecondPassword) is
// dead in v079 recvops (value -2, see the P2.4 notes) - accounts with a
// 2ndpassword row cannot leave the login screen there either; only the
// without-password path is ported.
package login

import (
	"context"
	"time"

	"GMS/internal/database"
	"GMS/internal/netw"
	"GMS/internal/protocol"
)

// handleCharSelect ports CharLoginHandler.Character_WithoutSecondPassword.
// Body after the 2-byte opcode: int charId. Order kept:
//
//	取账号在线状态(DB) != 2                -> getLoginFailed(7)
//	c.getLoginState() != 2 (in-memory)     -> getLoginFailed(7)
//	!isLoggedIn || loginFailCount ||
//	!login_Auth(charId)                    -> enableActions
//	channel not running || world != 0      -> close
//	putLoginAuth(charId, "ip:port", "", ch) + getServerIP(port, charId)
func (h handler) handleCharSelect(s *netw.Session, r *protocol.Reader) {
	c := h.loginClient(s)
	if c == nil {
		return // NeedsChecking gate (Java isLoggedIn below catches it too)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Java 取账号在线状态: the live accounts.loggedin column, not the
	// in-memory copy - a session that lost its DB state answers 7.
	if h.srv.store == nil {
		s.Write(LoginFailedPacket(loginAlreadyIn))
		return
	}
	st, err := h.srv.store.GetAccountState(ctx, c.accID)
	if err != nil || st.LoggedIn != database.LoginLoggedIn {
		s.Write(LoginFailedPacket(loginAlreadyIn))
		return
	}
	// Java c.getLoginState() != 2 - the in-memory login state (loggedIn
	// mirrors updateLoginState(2)).
	if !c.loggedIn {
		s.Write(LoginFailedPacket(loginAlreadyIn))
		return
	}

	charID := int(r.Int())
	if r.Err != nil {
		h.srv.log.Warn("malformed CHAR_SELECT", "err", r.Err, "remote", s.RemoteAddr)
		return
	}
	// Java: !c.isLoggedIn() || loginFailCount(c) || !c.login_Auth(charId)
	// -> enableActions (loginFailCount = >5 failed attempts).
	if c.loginAttempt > 5 || !c.allowedChars[charID] {
		h.srv.log.Info("char select rejected", "accID", c.accID, "charID", charID,
			"failCount", c.loginAttempt, "authorized", c.allowedChars[charID])
		s.Write(EnableActionsPacket())
		return
	}
	// Java: ChannelServer.getInstance(c.getChannel()) == null ||
	// c.getWorld() != 0 -> session close.
	if c.world != 0 {
		s.Close("char select on non-zero world")
		return
	}
	port, ok := h.srv.channelPort(c.channel)
	if !ok {
		s.Close("char select on stopped channel")
		return
	}

	// Java putLoginAuth(charId, ip.substring(1), c.getTempIP(), channel):
	// the login-session remote in "ip:port" form (Go RemoteAddr has no
	// leading '/'); tempIP stays empty on this path.
	if h.srv.registry != nil {
		h.srv.registry.PutLoginAuth(charID, s.RemoteAddr, "", c.channel)
	}
	h.srv.log.Info("char select ok", "accID", c.accID, "charID", charID,
		"channel", c.channel, "port", port)
	s.Write(ServerIPPacket(h.srv.externalIP, port, charID))
}
