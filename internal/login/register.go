// Package login - P2.5 auto-registration + IP/MAC ban lists.
//
// Java sources (see docs/FILETRACK.md):
//   - handling/login/handler/AutoRegister.java     -> autoRegister / createAccount
//   - client/MapleClient.hasBannedIP / isBannedMac -> bannedIP / bannedMac
//
// Documented deviations (docs/SESSION_STATE.md 断点 E):
//   - IP bans: Java matches `? LIKE CONCAT(ip,'%')` in SQL against Netty's
//     "/1.2.3.4:port" string. Go prefix-matches the loaded prefixes against
//     both "ip:port" and the bare IP, so rows written in either shape hit and
//     the query stays driver-neutral (SQLite has no CONCAT).
//   - Java sends the "注册成功" popup even when the per-MAC cap silently
//     skipped the INSERT (AutoRegister.mac is never read back by the caller);
//     Go answers with a dedicated cap/failure notice instead of asserting a
//     registration that did not happen. Packet sequence is unchanged.
package login

import (
	"context"
	"strings"
	"time"

	"GMS/internal/netw"
)

// zeroMAC is the machine code of a client with no NIC / a VM image; Java
// never bans it and never persists it (MapleClient.isBannedMac / updateMacs).
const zeroMAC = "00-00-00-00-00-00"

// bannedIP ports MapleClient.hasBannedIP: prefix match of the peer address
// against the `ipbans` table. A lookup error is swallowed exactly like the
// Java SQLException catch (treated as "not banned").
func (srv *Server) bannedIP(ctx context.Context, remote string) bool {
	ips, err := srv.store.BannedIPs(ctx)
	if err != nil {
		srv.log.Error("ip ban lookup failed", "err", err)
		return false
	}
	host := sessionIP(remote)
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		if strings.HasPrefix(remote, ip) || (host != "" && strings.HasPrefix(host, ip)) {
			srv.log.Info("login blocked by ip ban", "rule", ip, "remote", remote)
			return true
		}
	}
	return false
}

// bannedMac ports MapleClient.isBannedMac(macData): exact `macbans` match,
// with the Java guard that an all-zero or malformed machine code is never
// banned (mac.equalsIgnoreCase("00-00-00-00-00-00") || mac.length() != 17).
func (srv *Server) bannedMac(ctx context.Context, mac string) bool {
	if strings.EqualFold(mac, zeroMAC) || len(mac) != 17 {
		return false
	}
	yes, err := srv.store.IsBannedMac(ctx, mac)
	if err != nil {
		srv.log.Error("mac ban lookup failed", "err", err)
		return false
	}
	if yes {
		srv.log.Info("login blocked by mac ban", "mac", mac)
	}
	return yes
}

// autoRegister ports the Java CharLoginHandler.login registration branch:
//
//	if (AutoRegister.autoRegister && !accountExists(login) && !banned) {
//	    if (pwd is "disconnect"|"fixme") notice 此密码无效。   + getLoginFailed(1)
//	    else if (账号注册开关 <= 0)      createAccount(...)    + notice 注册成功
//	    else                            notice 管理员未开启注册功能
//	    getLoginFailed(1); return
//	}
//
// The reply is always [SERVERMESSAGE popup, LOGIN_STATUS reason 1]: the client
// shows the popup and drops back to the login dialog, so the player has to
// connect again with the account that was just created.
func (srv *Server) autoRegister(s *netw.Session, login, pwd, macData, remote string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name := srv.serverName
	switch {
	case strings.EqualFold(pwd, "disconnect"), strings.EqualFold(pwd, "fixme"):
		// Java rejects these two client special-cases before touching the DB.
		s.Write(ServerNoticeDialogPacket("此密码无效。"))
	case !srv.cfg.RegisterEnabled:
		s.Write(ServerNoticeDialogPacket("<<" + name + ">>\r\n\r\n管理员未开启注册功能。"))
	default:
		s.Write(ServerNoticeDialogPacket("<<" + name + ">>\r\n\r\n" +
			srv.createAccount(ctx, login, pwd, macData, remote)))
	}
	s.Write(LoginFailedPacket(1))
}

// createAccount ports AutoRegister.createAccount and returns the popup text
// for the player. Java's INSERT runs only while fewer than ACCOUNTS_PER_MAC
// accounts share the machine code.
func (srv *Server) createAccount(ctx context.Context, login, pwd, macData, remote string) string {
	n, err := srv.store.CountAccountsByMac(ctx, macData)
	if err != nil {
		srv.log.Error("auto-register: count accounts by mac failed", "err", err, "login", login)
		return "账号注册失败，请稍后再试。"
	}
	if limit := srv.cfg.AccountsPerMac; limit > 0 && n >= limit {
		srv.log.Warn("auto-register blocked: per-mac account limit",
			"mac", macData, "accounts", n, "limit", limit)
		return "同一机器码注册账号数量已达上限。"
	}
	if err := srv.store.InsertAutoRegisterAccount(ctx, login, hexSHA1(pwd), sessionIP(remote), macData); err != nil {
		srv.log.Error("auto-register failed", "err", err, "login", login)
		return "账号注册失败，请稍后再试。"
	}
	srv.log.Info("auto-registered account", "login", login, "mac", macData)
	return "账号注册成功，请重新登录，即可进入游戏。"
}
