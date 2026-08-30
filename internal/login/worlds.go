// Package login - P2.3 world/channel list (SERVERLIST / SERVERSTATUS).
//
// Java sources (see docs/FILETRACK.md):
//   - handling/login/LoginServer statics (serverName/eventMessage/userLimit/load)
//     -> WorldConfig + Server.worlds
//   - handling/login/handler/CharLoginHandler.ServerListRequest (20-world
//     switch loop) -> handleServerList
//   - CharLoginHandler.ServerStatusRequest -> handleServerStatus
//   - constants/GameConstants.getBalloons / handling.login.Balloon -> Balloon
//
// ZEV quirks kept:
//   - LICENSE_REQUEST (0x0003) also routes to the server-list handler
//     (Java MapleServerHandler case LICENSE_REQUEST).
//   - SERVERLIST_REQUEST and SERVERSTATUS_REQUEST sit behind the
//     RecvPacketOpcode.NeedsChecking() gate: silently dropped unless the
//     session completed LOGIN_PASSWORD (Java MapleClient.isLoggedIn()).
//   - All worlds share one serverName; channel entries are "name-<n>".
package login

import (
	"sort"

	"GMS/internal/config"
	"GMS/internal/netw"
)

// World is one enabled world entry (Java: the 20 <world>开关 GUI toggles,
// world ids 0..19 = 蓝蜗牛..花蘑菇).
type World struct {
	ID    int // serverId byte in SERVERLIST
	State int // ZEV.<world>状态: 0/1/2 heat display
}

// Balloon ports handling.login.Balloon (login-screen ad balloon).
type Balloon struct {
	X, Y    int
	Message string
}

// WorldConfig is the world-set wiring (Java LoginServer statics + load map).
type WorldConfig struct {
	ServerName   string    // MapleParty.开服名字
	EventMessage string    // LoginServer.eventMessage
	UserLimit    int       // LoginServer.userLimit
	Worlds       []World   // enabled worlds, ascending by ID
	ChannelCount int       // channels 1..N (ZEV.Count), load 0 (LoginServer.addChannel)
	Balloons     []Balloon // GameConstants.getBalloons (default: none)
}

// worldSet is the live server-side copy of WorldConfig plus the channel load
// map (Java LoginServer.load; channel -> player count).
type worldSet struct {
	serverName   string
	eventMessage string
	userLimit    int
	worlds       []World
	channelLoad  map[int]int
	balloons     []Balloon
}

// WorldConfigFrom maps the parsed TOML config onto the login world set
// (main.go wiring helper).
func WorldConfigFrom(cfg config.Root) WorldConfig {
	wc := WorldConfig{
		ServerName:   cfg.Server.WorldName,
		EventMessage: cfg.Server.EventMessage,
		UserLimit:    cfg.Server.UserLimit,
		Worlds:       make([]World, 0, len(cfg.Server.Worlds)),
		ChannelCount: cfg.Channel.Count,
		Balloons:     make([]Balloon, 0, len(cfg.Server.Balloons)),
	}
	for _, w := range cfg.Server.Worlds {
		wc.Worlds = append(wc.Worlds, World{ID: w.ID, State: w.State})
	}
	for _, b := range cfg.Server.Balloons {
		wc.Balloons = append(wc.Balloons, Balloon{X: b.X, Y: b.Y, Message: b.Message})
	}
	return wc
}

// SetWorlds wires the world set (Java LoginServer.run_startup_configurations;
// load = {channel: 0} per addChannel). Safe to call before Start.
func (s *Server) SetWorlds(wc WorldConfig) {
	sort.Slice(wc.Worlds, func(i, j int) bool { return wc.Worlds[i].ID < wc.Worlds[j].ID })
	load := make(map[int]int, wc.ChannelCount)
	for i := 1; i <= wc.ChannelCount; i++ {
		load[i] = 0
	}
	s.worldMu.Lock()
	s.worlds = worldSet{
		serverName:   wc.ServerName,
		eventMessage: wc.EventMessage,
		userLimit:    wc.UserLimit,
		worlds:       wc.Worlds,
		channelLoad:  load,
		balloons:     wc.Balloons,
	}
	s.worldMu.Unlock()
}

// worldSetSnapshot copies the world set for read-side use (entries are
// immutable once published).
func (s *Server) worldSetSnapshot() worldSet {
	s.worldMu.RLock()
	defer s.worldMu.RUnlock()
	return s.worlds
}

// SetUsersOn updates the online player count (Java LoginServer.setLoad;
// wired by the channel servers in P4; until then it stays 0).
func (s *Server) SetUsersOn(n int) { s.usersOn.Store(int64(n)) }

// UsersOn returns the online player count (Java LoginServer.getUsersOn).
func (s *Server) UsersOn() int { return int(s.usersOn.Load()) }

// handleServerList ports CharLoginHandler.ServerListRequest: one
// getServerList packet per enabled world (ascending id, mirroring the Java
// 0..19 switch order), then EndOfServerList. The NeedsChecking gate drops
// the request silently for not-logged-in sessions.
func (h handler) handleServerList(s *netw.Session) {
	c, _ := s.State.Load().(*client)
	if c == nil || !c.loggedIn {
		return
	}
	ws := h.srv.worldSetSnapshot()
	for _, w := range ws.worlds {
		s.Write(ServerListPacket(w.ID, ws.serverName, w.State, ws.eventMessage, ws.channelLoad, ws.balloons))
	}
	s.Write(EndOfServerListPacket())
}

// handleServerStatus ports CharLoginHandler.ServerStatusRequest: 2 = full,
// 1 = at/above half the user limit, 0 = normal. userLimit 0 makes any count
// "full" (same arithmetic as Java).
func (h handler) handleServerStatus(s *netw.Session) {
	c, _ := s.State.Load().(*client)
	if c == nil || !c.loggedIn {
		return
	}
	numPlayer := h.srv.UsersOn()
	userLimit := h.srv.worldSetSnapshot().userLimit
	status := 0
	switch {
	case numPlayer >= userLimit:
		status = 2
	case numPlayer*2 >= userLimit:
		status = 1
	}
	s.Write(ServerStatusPacket(status))
}