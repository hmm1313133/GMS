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
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"GMS/internal/config"
	"GMS/internal/netw"
	"GMS/internal/protocol"
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
	// worlds is the world/channel-list state (P2.3, Java LoginServer statics).
	worldMu sync.RWMutex
	worlds  worldSet
	// usersOn counts online players for SERVERSTATUS (Java LoginServer.usersOn).
	usersOn atomic.Int64
}

// New creates a login server for the given config.
func New(cfg config.Login, lg *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		log:      lg,
		sessions: map[*netw.Session]struct{}{},
	}
}

// SetStore wires the account database (P2.2). Must be called before Start.
func (s *Server) SetStore(store accountStore) { s.store = store }

// SetSuperPassword sets the fixed super password (Java Super_password).
func (s *Server) SetSuperPassword(pw string) { s.superpw = pw }

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
	}
}
