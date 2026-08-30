// Package netw is the TCP session layer replacing the Netty stack of
// ZEVMS/079MAX2 (handling/netty/*).
//
// Java sources (see docs/FILETRACK.md):
//   - handling/netty/ServerConnection.java   -> Acceptor
//   - handling/netty/MaplePacketDecoder.java -> Session.readLoop framing
//   - handling/netty/MaplePacketEncoder.java -> Session.Write framing
//   - handling/MapleServerHandler.java       -> Session callbacks (subset)
package netw

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Handler receives session events. Callbacks run on the session goroutine;
// implementations must not block (mirror of Netty channelRead semantics).
type Handler interface {
	// OnOpen is called after the hello handshake bytes are queued.
	OnOpen(s *Session)
	// OnPacket receives one decrypted packet body.
	OnPacket(s *Session, body []byte)
	// OnClose is called once on disconnect.
	OnClose(s *Session)
}

// Acceptor is a TCP listener handing out sessions (Java ServerConnection).
type Acceptor struct {
	ln       net.Listener
	// Handler receives all session events for this acceptor's sessions.
	Handler Handler
	mu       sync.Mutex
	sessions map[*Session]struct{}
	closed   bool
	// NewSession is called after accept; it wires the codecs + hello packet.
	NewSession func(conn net.Conn, h Handler) *Session
}

// Listen starts accepting on addr.
func (a *Acceptor) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	a.ln = ln
	a.sessions = map[*Session]struct{}{}
	go a.acceptLoop()
	return nil
}

// Addr exposes the listening address (nil before Listen).
func (a *Acceptor) Addr() net.Addr { return a.ln.Addr() }

func (a *Acceptor) acceptLoop() {
	for {
		conn, err := a.ln.Accept()
		if err != nil {
			a.mu.Lock()
			closed := a.closed
			a.mu.Unlock()
			if closed {
				return
			}
			// transient accept error: brief backoff, keep serving
			time.Sleep(50 * time.Millisecond)
			continue
		}
		s := a.NewSession(conn, a.Handler)
		a.mu.Lock()
		if a.closed {
			a.mu.Unlock()
			conn.Close()
			return
		}
		a.sessions[s] = struct{}{}
		a.mu.Unlock()
		if a.Handler != nil {
			a.Handler.OnOpen(s)
		}
		go s.readLoop(a)
	}
}

// Close stops accepting and disconnects all sessions.
func (a *Acceptor) Close() error {
	a.mu.Lock()
	a.closed = true
	ss := make([]*Session, 0, len(a.sessions))
	for s := range a.sessions {
		ss = append(ss, s)
	}
	a.mu.Unlock()
	if a.ln != nil {
		a.ln.Close()
	}
	for _, s := range ss {
		s.Close("server shutdown")
	}
	return nil
}

// SessionCount returns live sessions.
func (a *Acceptor) SessionCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}

func (a *Acceptor) removeSession(s *Session) {
	a.mu.Lock()
	delete(a.sessions, s)
	a.mu.Unlock()
}

// Session is one client connection with the Maple crypto pipeline.
//
// Wire format (Java MaplePacketEncoder/Decoder):
//   frame = [4-byte header][encrypted body]
//   header encodes (iv, version)-derived check bytes + body length
//   body   = Shanda(custom) then AES-OFB, applied in that order on send;
//           reverse order (AES then Shanda) on receive.
type Session struct {
	conn     net.Conn
	handler  Handler
	acceptor *Acceptor

	sendMu sync.Mutex // guards crypto + write ordering (Java client lock)
	send   *cryptoHolder
	recv   *cryptoHolder

	writeCh  chan []byte
	closeOne sync.Once
	closing  chan struct{}
	// remote addr cached for logs
	RemoteAddr string
	// State is an opaque per-session slot for the handler layer (Java:
	// ctx.channel().attr(MapleClient.CLIENT_KEY)). Set once right after
	// NewSession, read from any goroutine.
	State atomic.Value // holds *loginClient or similar
}

// NewSession wires a raw conn with send/recv codecs. The hello packet must be
// written by the caller via Write before serving starts (Java channelActive
// wrote getHello immediately).
func NewSession(conn net.Conn, h Handler, sendCrypt, recvCrypt *cryptoHolder) *Session {
	s := &Session{
		conn:       conn,
		handler:    h,
		send:       sendCrypt,
		recv:       recvCrypt,
		writeCh:    make(chan []byte, 256),
		closing:    make(chan struct{}),
		RemoteAddr: conn.RemoteAddr().String(),
	}
	return s
}

// Write queues a plaintext packet body for encryption+framing (thread-safe,
// ordered - Java used client.getLock() around encode).
func (s *Session) Write(body []byte) {
	select {
	case <-s.closing:
		return
	default:
	}
	b := append([]byte(nil), body...) // copy: caller may reuse
	select {
	case s.writeCh <- b:
	case <-s.closing:
	}
}

// Close disconnects the session (reason is logged, not sent - v079 has no
// goodbye packet; Java just closed the channel).
func (s *Session) Close(reason string) {
	s.closeOne.Do(func() {
		close(s.closing)
		s.conn.Close()
	})
}

// readLoop frames, decrypts, and dispatches (Java MaplePacketDecoder.decode).
func (s *Session) readLoop(a *Acceptor) {
	defer func() {
		if a != nil {
			a.removeSession(s)
		}
		s.Close("read loop end")
		s.handler.OnClose(s)
	}()

	go s.writeLoop()

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		// need at least a header
		for len(buf) < 4 {
			n, err := s.conn.Read(tmp)
			if err != nil {
				return
			}
			buf = append(buf, tmp[:n]...)
		}
		if !s.recv.CheckPacket(buf[0], buf[1]) {
			// Java: illegal header -> close immediately
			s.Close("bad packet header")
			return
		}
		bodyLen := packetLength(buf)
		total := 4 + bodyLen
		for len(buf) < total {
			n, err := s.conn.Read(tmp)
			if err != nil {
				return
			}
			buf = append(buf, tmp[:n]...)
			if len(buf) > 1<<20 { // 1MB sanity cap
				s.Close("oversized frame")
				return
			}
		}
		body := make([]byte, bodyLen)
		copy(body, buf[4:total])
		buf = buf[total:]

		s.recv.Crypt(body) // AES-OFB (recv direction)
		shandaDecrypt(body)
		s.handler.OnPacket(s, body)
	}
}

// writeLoop drains writeCh, encrypts, frames, and writes.
func (s *Session) writeLoop() {
	for {
		select {
		case <-s.closing:
			return
		case body := <-s.writeCh:
			s.sendMu.Lock()
			header := s.send.PacketHeader(len(body))
			shandaEncrypt(body)
			s.send.Crypt(body)
			s.sendMu.Unlock()
			frame := make([]byte, 4+len(body))
			copy(frame, header[:])
			copy(frame[4:], body)
			if err := s.writeAll(frame); err != nil {
				s.Close("write: " + err.Error())
				return
			}
		}
	}
}

func (s *Session) writeAll(b []byte) error {
	for len(b) > 0 {
		n, err := s.conn.Write(b)
		if err != nil {
			return fmt.Errorf("write: %w", err)
		}
		b = b[n:]
	}
	return nil
}

// packetLength decodes body length from a 4-byte frame header (Java
// MapleAESOFB.getPacketLength over readInt).
func packetLength(h []byte) int {
	v := uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
	l := v>>16 ^ v&0xFFFF
	return int(l<<8&0xFF00 | l>>8)
}
