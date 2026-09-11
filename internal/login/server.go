// Package login is the v079 login server (GMS-P2). This file: P1 smoke
// server - real getHello handshake + PING keepalive, enough for
// tools/protocoltest to verify the whole crypto/framing stack against.
//
// Java sources (see docs/FILETRACK.md):
//   - tools/packet/LoginPacket.getHello   -> helloPacket
//   - tools/packet/LoginPacket.getPing    -> pingPacket
//   - handling/login/LoginServer          -> Server (P2 will expand)
package login

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"GMS/internal/config"
	"GMS/internal/netw"
	"GMS/internal/protocol"
	"GMS/internal/world"
)

// Server is the login server skeleton.
type Server struct {
	cfg      config.Login
	log      *slog.Logger
	acceptor *netw.Acceptor
	mu       sync.Mutex
	sessions map[*netw.Session]struct{}
	// store is the account DB (nil in P1 smoke mode: LOGIN_PASSWORD then
	// always answers loginok=5 like an unknown account).
	store accountStore
	// superpw is the fixed super password (Java ServerConstants.superpw,
	// default empty = disabled).
	superpw string
	// serverName is the display name used in login popups
	// (Java MapleParty.开服名字, mirrored by LoginServer.serverName).
	serverName string
	// worlds is the world/channel-list state (P2.3, Java LoginServer statics).
	worldMu sync.RWMutex
	worlds  worldSet
	// usersOn counts online players for SERVERSTATUS (Java LoginServer.usersOn).
	usersOn atomic.Int64
	// registry is the login-ticket store shared with the channel servers
	// (P4.1; Java LoginServer.loginAuth/loginIPAuth statics live in
	// internal/world so login and channel do not depend on each other).
	registry *world.LoginRegistry
	// channelPort resolves a channel id to its listen port (Java
	// ChannelServer.getInstance(ch).getIP().split(":")[1]; nil = no channel
	// servers wired, CHAR_SELECT then closes the session).
	channelPort func(channel int) (int, bool)
	// externalIP is the address SERVER_IP hands to clients (Java
	// MapleParty.IP地址 through InetAddress.getByName).
	externalIP [4]byte
	// clients maps accID -> the live login session of that account (the
	// login-server slice of Java World.Client.getClients); unlockAcc uses it
	// to evict the previous session on a double login.
	clientMu sync.Mutex
	clients  map[int]*netw.Session
}

// New creates a login server for the given config.
func New(cfg config.Login, lg *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		log:      lg,
		sessions: map[*netw.Session]struct{}{},
		clients:  map[int]*netw.Session{},
	}
}

// registerClient records the live login session of an account (Java
// World.Client.addClient; set once updateLoginState(2) succeeded).
func (s *Server) registerClient(accID int, sess *netw.Session) {
	s.clientMu.Lock()
	s.clients[accID] = sess
	s.clientMu.Unlock()
}

// unregisterClient drops the mapping when it still points at this session
// (a newer login may already have replaced it).
func (s *Server) unregisterClient(accID int, sess *netw.Session) {
	s.clientMu.Lock()
	if s.clients[accID] == sess {
		delete(s.clients, accID)
	}
	s.clientMu.Unlock()
}

// clientSession returns the live login session of an account, or nil.
func (s *Server) clientSession(accID int) *netw.Session {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	return s.clients[accID]
}

// unlockAcc ports MapleClient.unlockAcc (P4.1): a double login was detected,
// so release whatever holds the account.
//   - a live session exists  -> unLockDisconnect: notice + close it
//     (Java waits 1s before closing; the notice is what matters to the
//     player, the delay is not observable here)
//   - no live session (crashed/killed client) -> clear accounts.loggedin so
//     the next attempt can proceed (the Java else-branch)
//
// The current attempt still answers loginok=7 in both cases, exactly like
// the Java flow (the client shows "already logged in" and retries).
func (s *Server) unlockAcc(ctx context.Context, accID int) {
	if old := s.clientSession(accID); old != nil {
		old.Write(ServerNoticeDialogPacket("与服务器断开连接，检测到其他登陆。"))
		// Java unLockDisconnect closes the socket from a helper thread one
		// second later (Thread.sleep(1000)) - without that gap the queued
		// notice would be dropped with the connection.
		time.AfterFunc(time.Second, func() { old.Close("double login") })
		s.unregisterClient(accID, old)
		s.log.Info("double login, live session evicted", "accID", accID)
		return
	}
	if s.store == nil {
		return
	}
	if err := s.store.ResetAccountLogin(ctx, accID); err != nil {
		s.log.Error("stale login state reset failed", "err", err, "accID", accID)
		return
	}
	s.log.Info("double login, stale loggedin reset", "accID", accID)
}

// SetStore wires the account database (P2.2). Must be called before Start.
func (s *Server) SetStore(store accountStore) { s.store = store }

// SetSuperPassword sets the fixed super password (Java Super_password).
func (s *Server) SetSuperPassword(pw string) { s.superpw = pw }

// SetServerName sets the display name used in login popups (P2.5
// auto-register notices; Java MapleParty.开服名字 / LoginServer.serverName).
func (s *Server) SetServerName(name string) { s.serverName = name }

// SetRegistry wires the login-ticket store shared with the channel servers
// (P4.1; Java LoginServer.loginAuth static map). Must be called before
// Start; nil disables ticket recording (CHAR_SELECT still replies).
func (s *Server) SetRegistry(r *world.LoginRegistry) { s.registry = r }

// SetChannelPortLookup wires the channel-port resolver (Java
// ChannelServer.getInstance(channel).getIP()). Returning ok=false for a
// channel makes CHAR_SELECT close the session, mirroring the Java null check.
func (s *Server) SetChannelPortLookup(fn func(channel int) (int, bool)) { s.channelPort = fn }

// SetExternalIP sets the address handed to clients in SERVER_IP (Java
// MapleParty.IP地址 - the ZEVMS default there is the unusable "0.0.0.0";
// the Go config defaults to loopback for local smoke tests).
func (s *Server) SetExternalIP(ip string) error {
	pip := net.ParseIP(ip)
	if pip == nil {
		return fmt.Errorf("invalid external ip %q", ip)
	}
	v4 := pip.To4()
	if v4 == nil {
		return fmt.Errorf("external ip %q is not IPv4", ip)
	}
	copy(s.externalIP[:], v4)
	return nil
}

// Start begins listening. Blocks callers only on error; serving is async.
func (s *Server) Start() error {
	s.acceptor = &netw.Acceptor{Handler: handler{srv: s}}
	s.acceptor.NewSession = func(conn net.Conn, h netw.Handler) *netw.Session {
		// Java MapleServerHandler.channelActive:
		//   serverRecv = {70, 114, 12, rand}; serverSend = {82, 48, 120, rand}
		ivRecv := [4]byte{70, 114, 12, byte(rngInt(255))}
		ivSend := [4]byte{82, 48, 120, byte(rngInt(255))}
		sendH, recvH := netw.NewCryptoPair(ivSend, ivRecv)
		sess := netw.NewSession(conn, h, sendH, recvH)
		// Java MaplePacketEncoder: when the channel has no MapleClient yet
		// (hello is written before the client attr is set), the packet goes
		// out RAW - no 4-byte frame header, no Shanda/AES. The client reads
		// these 15 bytes first to learn both IVs.
		if err := writeFull(conn, HelloPacket(79, ivSend, ivRecv)); err != nil {
			sess.Close("hello write: " + err.Error())
		}
		return sess
	}
	addr := netwJoinHostPort(s.cfg.Host, s.cfg.Port)
	if err := s.acceptor.Listen(addr); err != nil {
		return err
	}
	s.log.Info("login server listening", "addr", addr)
	return nil
}

// Stop shuts the listener and all sessions.
func (s *Server) Stop() {
	if s.acceptor != nil {
		s.acceptor.Close()
	}
	s.mu.Lock()
	sess := make([]*netw.Session, 0, len(s.sessions))
	for x := range s.sessions {
		sess = append(sess, x)
	}
	s.mu.Unlock()
	for _, x := range sess {
		x.Close("login stop")
	}
}

// handler implements netw.Handler for the login port.
type handler struct {
	srv *Server
}

func (h handler) OnOpen(s *netw.Session) {
	h.srv.mu.Lock()
	h.srv.sessions[s] = struct{}{}
	h.srv.mu.Unlock()
	h.srv.log.Info("login connection", "remote", s.RemoteAddr)
}

func (h handler) OnClose(s *netw.Session) {
	h.srv.mu.Lock()
	delete(h.srv.sessions, s)
	h.srv.mu.Unlock()
	// drop the account mapping so unlockAcc can tell a live session from a
	// dead one (Java World.Client.removeClient).
	if c, _ := s.State.Load().(*client); c != nil {
		h.srv.unregisterClient(c.accID, s)
	}
}

func (h handler) OnPacket(s *netw.Session, body []byte) {
	if len(body) < 2 {
		return
	}
	opcode := protocol.RecvOp(uint16(body[0]) | uint16(body[1])<<8)
	h.srv.log.Debug("login packet", "opcode", fmt.Sprintf("%04X", opcode), "len", len(body), "remote", s.RemoteAddr)
	switch opcode {
	case protocol.RecvPONG: // 0x13: reply PING (0x14) - Java getPing
		s.Write(PingPacket())
	case protocol.RecvLOGIN_PASSWORD: // 0x0001 - Java CharLoginHandler.login
		h.handleLoginPassword(s, protocol.NewReader(body[2:]))
	case protocol.RecvSERVERLIST_REQUEST, protocol.RecvLICENSE_REQUEST: // 0x0002 / 0x0003
		// Java CharLoginHandler.ServerListRequest; ZEV routes LICENSE_REQUEST
		// here too (MapleServerHandler case LICENSE_REQUEST).
		h.handleServerList(s)
	case protocol.RecvSERVERSTATUS_REQUEST: // 0x0005 - click-channel status
		h.handleServerStatus(s)
	case protocol.RecvSET_GENDER: // 0x0004 - Java CharLoginHandler.SetGenderRequest
		h.handleSetGender(s, protocol.NewReader(body[2:]))
	case protocol.RecvCHARLIST_REQUEST: // 0x0009 - Java CharLoginHandler.CharlistRequest
		h.handleCharlistRequest(s, protocol.NewReader(body[2:]))
	case protocol.RecvCHECK_CHAR_NAME: // 0x000C - Java CharLoginHandler.CheckCharName
		h.handleCheckCharName(s, protocol.NewReader(body[2:]))
	case protocol.RecvCREATE_CHAR: // 0x0011 - Java CharLoginHandler.CreateChar
		h.srv.log.Debug("create char raw", "body", fmt.Sprintf("% X", body))
		h.handleCreateChar(s, protocol.NewReader(body[2:]))
	case protocol.RecvDELETE_CHAR: // 0x0012 - Java CharLoginHandler.DeleteChar
		h.handleDeleteChar(s, protocol.NewReader(body[2:]))
	case protocol.RecvCHAR_SELECT: // 0x000A - Java Character_WithoutSecondPassword
		h.handleCharSelect(s, protocol.NewReader(body[2:]))
	}
}
