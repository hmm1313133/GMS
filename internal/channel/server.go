// Package channel is the v079 channel server skeleton (GMS-P4.1). Each
// channel is one TCP listener on port base+channel-1 (Java: 7574 + channel,
// channel ids start at 1), sharing the MapleServerHandler handshake with the
// login server. The in-world packet surface (PLAYER_LOGGEDIN and friends)
// grows in P4.2+.
//
// Java sources (see docs/FILETRACK.md):
//   - handling/channel/ChannelServer      -> Server / ChannelServer
//   - handling/netty/ServerConnection     -> netw.Acceptor per channel
//   - handling/MapleServerHandler         -> handler (channel branch)
//   - tools/packet/LoginPacket.getHello   -> shared hello (via netw)
package channel

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"GMS/internal/config"
	"GMS/internal/database"
	"GMS/internal/mapp"
	"GMS/internal/netw"
	"GMS/internal/protocol"
	"GMS/internal/world"
)

// characterStore is the DB surface the channel server needs (implemented by
// *database.DB; faked in tests).
type characterStore interface {
	GetCharacterByID(ctx context.Context, id int) (*database.Character, error)
}

// configValueStore is the DB surface behind the ZEVMS switch table
// (Java gui/Start.GetConfigValues -> Start.ConfigValuesMap). Optional: a nil
// store means "no switches configured", i.e. every switch reads 0 = on.
type configValueStore interface {
	ConfigValues(ctx context.Context) (map[string]int, error)
}

// Config covers one channel-server process (Java run_startup_configurations
// reads ZEV.* properties per channel; Go takes them from gms.toml).
type Config struct {
	Host string
	// BasePort is the port of channel 1 (Java DEFAULT_PORT 7574 + channel 1;
	// gms.toml channel.port = 7575). Channel i listens on BasePort+i-1.
	BasePort int
	Count    int // channels 1..N (Java ZEV.Count, capped 10 by startChannel_Main)
	ExpRate  int // clamped to 100 (Java: >100 -> 100)
	MesoRate int
	DropRate int
	// ServerMessage is the scrolling banner (Java Game.私服滚动公告).
	ServerMessage string
	ServerName    string
	// ExternalIP is the advertised address (Java MapleParty.IP地址): the
	// channel builds its "ip:port" identity from it (ChannelServer.ip). The
	// login server hands the port to clients in SERVER_IP.
	ExternalIP string
}

// ConfigFrom maps the parsed TOML onto the channel config (main.go wiring
// helper, mirroring login.WorldConfigFrom).
func ConfigFrom(cfg config.Root) Config {
	c := Config{
		Host:          cfg.Channel.Host,
		BasePort:      cfg.Channel.Port,
		Count:         cfg.Channel.Count,
		ExpRate:       cfg.Game.ExpRate,
		MesoRate:      cfg.Game.MesoRate,
		DropRate:      cfg.Game.DropRate,
		ServerMessage: cfg.Game.ServerMessage,
		ServerName:    cfg.Server.WorldName,
		ExternalIP:    cfg.Server.ExternalIP,
	}
	if c.ExternalIP == "" {
		c.ExternalIP = "127.0.0.1"
	}
	return c
}

// Server owns all channel listeners (Java: the static instances map plus the
// startChannel_Main loop).
type Server struct {
	cfg Config
	log *slog.Logger
	// mu guards the channel map and the optional configvalues store (the
	// latter is only written at startup, before the listeners accept).
	mu       sync.Mutex
	channels map[int]*ChannelServer
	// onLoad reports live player counts to the login server (Java:
	// ChannelServer.getChannelLoad polled by LoginWorker; Go pushes).
	onLoad func(channel, players int)
	// find is the cross-channel online registry (Java World.Find static).
	find *world.Finder
	// store loads character rows for PLAYER_LOGGEDIN (P4.2; Java
	// MapleCharacter.loadCharFromDB). nil = DB-degraded startup.
	store characterStore
	// configStore loads the ZEVMS switch table (P4.5b; Java
	// gui/Start.GetConfigValues). nil = every switch reads 0 = on.
	configStore configValueStore
	// configVals is the loaded switch map (Java Start.ConfigValuesMap),
	// published by ReloadConfigValues and read - lock-free - from every
	// player's packet goroutine. The map behind the pointer is never mutated
	// after publication; a reload swaps in a fresh one.
	configVals atomic.Pointer[map[string]int]
}

// New creates the channel-server fleet for the given config.
func New(cfg Config, lg *slog.Logger) *Server {
	// Java startChannel_Main caps ZEV.Count at 10 channels;
	// run_startup_configurations clamps the rates at 100.
	if cfg.Count > 10 {
		cfg.Count = 10
	}
	if cfg.ExpRate > 100 {
		cfg.ExpRate = 100
	}
	if cfg.MesoRate > 100 {
		cfg.MesoRate = 100
	}
	if cfg.DropRate > 100 {
		cfg.DropRate = 100
	}
	return &Server{
		cfg:      cfg,
		log:      lg,
		channels: map[int]*ChannelServer{},
		find:     world.NewFinder(),
	}
}

// SetLoadReporter wires the login-server load sink (Java LoginServer.load /
// setLoad). Called with the true player count each time it changes.
func (s *Server) SetLoadReporter(fn func(channel, players int)) { s.onLoad = fn }

// Finder exposes the cross-channel online registry (Java World.Find).
func (s *Server) Finder() *world.Finder { return s.find }

// SetStore wires the character database (Java MapleCharacter.loadCharFromDB's
// connection). A nil store keeps the server running but PLAYER_LOGGEDIN closes
// the session (cmd/gms DB-degraded startup).
func (s *Server) SetStore(store characterStore) { s.store = store }

// SetConfigValues wires the ZEVMS switch table (Java gui/Start.GetConfigValues'
// connection). Like SetStore it is optional: with no store wired every switch
// reads 0, i.e. every gated feature stays on.
func (s *Server) SetConfigValues(store configValueStore) {
	s.mu.Lock()
	s.configStore = store
	s.mu.Unlock()
}

// ConfigValues returns the loaded switch map (Java Start.ConfigValuesMap). It
// is never nil once a reload succeeded, but a server without a store - or one
// whose load failed - returns nil, and a nil map reads 0 for every key, which
// is exactly the Java "switch absent = enabled" behaviour.
//
// The returned map is a read-only snapshot: reloads publish a new map instead
// of mutating this one, so callers may read it from any goroutine.
func (s *Server) ConfigValues() map[string]int {
	if m := s.configVals.Load(); m != nil {
		return *m
	}
	return nil
}

// ReloadConfigValues loads the switch table once and caches it, reporting how
// many switches were read. Java calls gui/Start.GetConfigValues exactly once
// at boot (Start.startServer:132); cmd/gms does the same here, and the
// explicit reload exists for the P8 ops panel.
//
// It never panics and never blocks a packet handler: with no store wired it is
// a no-op, and a DB error is returned to the caller (cmd/gms logs it as a
// warning) while the previously loaded map stays in place - a failed reload
// must not silently switch features back on.
func (s *Server) ReloadConfigValues(ctx context.Context) (int, error) {
	s.mu.Lock()
	store := s.configStore
	s.mu.Unlock()
	if store == nil {
		return len(s.ConfigValues()), nil
	}
	vals, err := store.ConfigValues(ctx)
	if err != nil {
		return 0, fmt.Errorf("channel: load configvalues: %w", err)
	}
	s.configVals.Store(&vals)
	return len(vals), nil
}

// switchOn reports whether a ZEVMS switch disables its feature. The Java
// convention is uniform: `Start.ConfigValuesMap.get(name) > 0` means OFF, and
// a key the map does not carry (including every key when nothing was loaded)
// reads 0 = enabled.
func (s *Server) switchOn(name string) bool {
	m := s.configVals.Load()
	return m != nil && (*m)[name] > 0
}

// forceRemovePlayerByAccID ports ChannelServer.forceRemovePlayerByAccId(c,
// accid): when one account logs in again, its previous character - still
// online on another channel - is disconnected and removed. except is the
// player that just logged in (Java skips the live client).
func (s *Server) forceRemovePlayerByAccID(accID int, except *Player) {
	if accID == 0 {
		return
	}
	s.mu.Lock()
	chs := make([]*ChannelServer, 0, len(s.channels))
	for _, cs := range s.channels {
		chs = append(chs, cs)
	}
	s.mu.Unlock()
	for _, cs := range chs {
		for _, old := range cs.players.GetAllCharacters() {
			if old == except || old.AccountID() != accID {
				continue
			}
			if old.Sess != nil {
				old.Sess.Close("duplicate account login")
			}
			cs.players.DeregisterPlayer(old)
			if m := cs.lookupMap(old.MapID()); m != nil {
				m.RemovePlayer(old)
			}
			s.log.Info("duplicate account player removed", "accID", accID,
				"playerID", old.ID, "channel", cs.channel)
		}
	}
}

// Start boots all channels (Java startChannel_Main: newInstance(i+1) for
// i in 0..count-1). Fails on the first listener that cannot bind.
func (s *Server) Start() error {
	for i := 1; i <= s.cfg.Count; i++ {
		cs := newChannelServer(s, i)
		if err := cs.start(); err != nil {
			s.Stop()
			return err
		}
		s.mu.Lock()
		s.channels[i] = cs
		s.mu.Unlock()
		// Java setChannel registers into the instances map and calls
		// LoginServer.addChannel(channel) - the login server learns the
		// channel set from the shared config; the load starts at 0.
		if s.onLoad != nil {
			s.onLoad(i, 0)
		}
	}
	return nil
}

// Stop shuts every channel listener down (Java shutdown()).
func (s *Server) Stop() {
	s.mu.Lock()
	chs := make([]*ChannelServer, 0, len(s.channels))
	for _, cs := range s.channels {
		chs = append(chs, cs)
	}
	s.channels = map[int]*ChannelServer{}
	s.mu.Unlock()
	for _, cs := range chs {
		cs.stop()
	}
}

// Channel returns the running channel server with that 1-based id (Java
// ChannelServer.getInstance(channel); nil when absent).
func (s *Server) Channel(id int) *ChannelServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channels[id]
}

// ChannelServer is one channel listener (Java ChannelServer instance).
type ChannelServer struct {
	srv      *Server
	channel  int // 1-based (Java this.channel)
	port     int // 7574 + channel (Java this.port)
	ip       string
	log      *slog.Logger
	acceptor *netw.Acceptor
	players  *PlayerStorage
	shutdown atomic.Bool

	// maps holds this channel's map instances (Java
	// ChannelServer.getMapFactory()), created on first use.
	mapMu sync.Mutex
	maps  map[int]*mapp.Map
}

func newChannelServer(s *Server, channel int) *ChannelServer {
	port := s.cfg.BasePort + channel - 1 // Java: 7574 + channel
	cs := &ChannelServer{
		srv:     s,
		channel: channel,
		port:    port,
		log:     s.log,
		maps:    map[int]*mapp.Map{},
	}
	// Java this.ip = MapleParty.IP地址 + ":" + port (the advertised address,
	// not the bind address).
	cs.ip = net.JoinHostPort(s.cfg.ExternalIP, strconv.Itoa(port))
	cs.players = newPlayerStorage(channel, s.find, s.reportLoad)
	return cs
}

// Channel returns the 1-based channel id (Java getChannel).
func (cs *ChannelServer) Channel() int { return cs.channel }

// Port returns the listen port (Java getIP().split(":")[1] - the login
// server reads the port back out of the ip string).
func (cs *ChannelServer) Port() int { return cs.port }

// IP returns the advertised "host:port" (Java getIP; the login SERVER_IP
// packet only uses the port).
func (cs *ChannelServer) IP() string { return cs.ip }

// Players exposes the channel player storage (Java getPlayerStorage).
func (cs *ChannelServer) Players() *PlayerStorage { return cs.players }

// Map returns the channel's instance of a map id, creating it on first use
// (Java ChannelServer.getMapFactory().getMap(id)).
func (cs *ChannelServer) Map(id int) *mapp.Map {
	cs.mapMu.Lock()
	defer cs.mapMu.Unlock()
	m := cs.maps[id]
	if m == nil {
		m = mapp.New(id)
		cs.maps[id] = m
	}
	return m
}

// lookupMap returns an existing map instance without creating it.
func (cs *ChannelServer) lookupMap(id int) *mapp.Map {
	cs.mapMu.Lock()
	defer cs.mapMu.Unlock()
	return cs.maps[id]
}

// MapCount returns how many distinct map instances this channel has touched.
func (cs *ChannelServer) MapCount() int {
	cs.mapMu.Lock()
	defer cs.mapMu.Unlock()
	return len(cs.maps)
}

// start begins listening (Java run_startup_configurations' acceptor.run()).
func (cs *ChannelServer) start() error {
	cs.acceptor = &netw.Acceptor{Handler: channelHandler{cs: cs}}
	cs.acceptor.NewSession = func(conn net.Conn, h netw.Handler) *netw.Session {
		// Java MapleServerHandler.channelActive (login/channel/cashshop share
		// this): serverRecv = {70, 114, 12, rand}; serverSend = {82, 48, 120,
		// rand}; then getHello(79, ivSend, ivRecv) is written RAW.
		ivRecv := [4]byte{70, 114, 12, byte(rngInt(255))}
		ivSend := [4]byte{82, 48, 120, byte(rngInt(255))}
		sendH, recvH := netw.NewCryptoPair(ivSend, ivRecv)
		sess := netw.NewSession(conn, h, sendH, recvH)
		if err := writeFull(conn, helloPacket(79, ivSend, ivRecv)); err != nil {
			sess.Close("hello write: " + err.Error())
		}
		return sess
	}
	addr := hostPort(cs.srv.cfg.Host, cs.port)
	if err := cs.acceptor.Listen(addr); err != nil {
		return err
	}
	cs.log.Info("channel server listening", "channel", cs.channel, "addr", addr, "ip", cs.ip)
	return nil
}

// stop closes the listener and its sessions (Java shutdown()).
func (cs *ChannelServer) stop() {
	cs.shutdown.Store(true)
	if cs.acceptor != nil {
		cs.acceptor.Close()
	}
}

// reportLoad pushes the live count to the login server (Java LoginWorker
// sweeps ChannelServer.getChannelLoad every 10 minutes; Go reports on
// change).
func (s *Server) reportLoad(channel, players int) {
	if s.onLoad != nil {
		s.onLoad(channel, players)
	}
}

// channelHandler implements netw.Handler for a channel port (Java
// MapleServerHandler with this.channel > -1).
type channelHandler struct {
	cs *ChannelServer
}

func (h channelHandler) OnOpen(s *netw.Session) {
	// Java channelActive: when the channel is shutting down the socket is
	// closed right away (ChannelServer.isShutdown check).
	if h.cs.shutdown.Load() {
		s.Close("channel shutdown")
		return
	}
	h.cs.log.Info("channel connection", "channel", h.cs.channel, "remote", s.RemoteAddr)
}

func (h channelHandler) OnClose(s *netw.Session) {
	// Java MapleClient.disconnect -> ChannelServer.removePlayer +
	// map.removePlayer (the player tracking is P4.2, the despawn broadcast -
	// REMOVE_PLAYER_FROM_MAP to everyone left - is P4.3; the save sweep is P5).
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player
	if h.cs.players.GetPlayerByID(p.ID) == p {
		h.cs.players.DeregisterPlayer(p)
	}
	if m := h.cs.lookupMap(p.MapID()); m != nil {
		// RemovePlayer is a no-op for a player that already left (the
		// duplicate-account eviction path removes it first).
		m.RemovePlayer(p)
	}
	h.cs.log.Info("player logged out", "channel", h.cs.channel, "playerID", p.ID, "name", p.Name)
}

func (h channelHandler) OnPacket(s *netw.Session, body []byte) {
	if len(body) < 2 {
		return
	}
	opcode := protocol.RecvOp(uint16(body[0]) | uint16(body[1])<<8)
	// NB: keep the %X verb off the RecvOp value itself - it implements
	// fmt.Stringer, so fmt would hex-encode the ASCII *name* and log
	// "47454E4552414C5F43484154" instead of the opcode number.
	h.cs.log.Debug("channel packet", "opcode", fmt.Sprintf("0x%04X", uint16(opcode)), "name", opcode.String(), "len", len(body), "remote", s.RemoteAddr)
	switch opcode {
	case protocol.RecvPONG: // 0x13: reply PING (0x14) - Java getPing
		s.Write(pingPacket())
	case protocol.RecvPLAYER_LOGGEDIN: // 0x0B - Java InterServerHandler.Loggedin
		h.handlePlayerLoggedIn(s, protocol.NewReader(body[2:]))
	case protocol.RecvMOVE_PLAYER: // 0x24 - Java PlayerHandler.MovePlayer
		h.handleMovePlayer(s, protocol.NewReader(body[2:]))
	case protocol.RecvGENERAL_CHAT: // 0x2D - Java ChatHandler.GeneralChat
		h.handleGeneralChat(s, protocol.NewReader(body[2:]))
	case protocol.RecvFACE_EXPRESSION: // 0x2F - Java PlayerHandler.ChangeEmotion
		h.handleFaceExpression(s, protocol.NewReader(body[2:]))
	case protocol.RecvWHISPER: // 0x75 - Java ChatHandler.Whisper_Find
		h.handleWhisper(s, protocol.NewReader(body[2:]))
	}
}

// ---- small helpers shared with the login server (kept local; the login
// package has identical unexported ones) ----

// rngInt returns a pseudo-random int in [0, n) (Java Randomizer.nextInt).
func rngInt(n int) int { return rand.IntN(n) }

// hostPort mirrors login.netwJoinHostPort ("0.0.0.0" listens on all
// interfaces - Java ServerConnection binds the raw port).
func hostPort(host string, port int) string {
	if host == "" || host == "0.0.0.0" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// writeFull writes b to conn completely.
func writeFull(conn net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := conn.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return net.ErrClosed
		}
		b = b[n:]
	}
	return nil
}

// helloPacket ports Java LoginPacket.getHello (identical to the login
// server's - both come from the same MapleServerHandler.channelActive):
// writeShort(13); writeShort(version); write(0,0); write(recvIv);
// write(sendIv); write(4).
func helloPacket(version int, sendIV, recvIV [4]byte) []byte {
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

// pingPacket ports Java LoginPacket.getPing.
func pingPacket() []byte {
	w := protocol.NewWriter(2)
	w.Op(protocol.SendPING)
	return w.Bytes()
}
