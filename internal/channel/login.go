// Package channel - P4.2 player assembly: PLAYER_LOGGEDIN -> character load
// -> channel registration -> WARP_TO_MAP (the client enters the map).
//
// Java sources (see docs/FILETRACK.md):
//   - handling/MapleServerHandler case PLAYER_LOGGEDIN -> handlePlayerLoggedIn
//   - handling/channel/handler/InterServerHandler.Loggedin/Loggedin2 -> loggedIn2
//   - client/MapleCharacter.loadCharFromDB (characters-row subset) -> loadCharacter
//   - handling/channel/ChannelServer.addPlayer / forceRemovePlayerByAccId
package channel

import (
	"context"
	"errors"
	"time"

	"GMS/internal/database"
	"GMS/internal/netw"
	"GMS/internal/packet"
	"GMS/internal/protocol"
)

// errNoStore is returned when the channel server has no character database
// wired (cmd/gms DB-degraded startup).
var errNoStore = errors.New("channel: no character store wired")

// client is the channel-side per-connection state - the slice of Java
// MapleClient the channel server needs (P4.2). It lives in Session.State.
type client struct {
	cs     *ChannelServer
	sess   *netw.Session
	player *Player
}

// handlePlayerLoggedIn ports MapleServerHandler's PLAYER_LOGGEDIN case
// (0x000B): the client reconnects to the channel port it got in SERVER_IP and
// announces the selected character id.
//
// Java InterServerHandler.Loggedin runs the 登陆验证开关 branch (the "*abc"
// robot challenge) before Loggedin2. That switch lives in the configvalues
// table, which is absent from the 079-max2 dump and classified SKIP/P7 - like
// the P2.5 login guard, the Go port goes straight to Loggedin2.
func (h channelHandler) handlePlayerLoggedIn(s *netw.Session, r *protocol.Reader) {
	playerID := int(r.Int())
	if r.Err != nil {
		h.cs.log.Warn("malformed PLAYER_LOGGEDIN", "err", r.Err, "remote", s.RemoteAddr)
		return
	}
	h.cs.loggedIn2(s, playerID)
}

// loggedIn2 ports InterServerHandler.Loggedin2.
//
// Order kept:
//
//	player = pending char (channel change) else loadCharFromDB(charid)
//	c.setPlayer / c.setAccID / c.loadAccountData   (P4.2: player + accID)
//	ChannelServer.forceRemovePlayerByAccId(c, accID)
//	channelServer.addPlayer(player)                (register + banner)
//	c.sendPacket(getCharInfo) + temporaryStats_Reset
//	player.getMap().addPlayer(player)
//
// Skipped Java steps (documented): the GM skill grants (管理隐身/管理加速
// switches, configvalues), c.updateLoginState(2) (the login server already
// wrote accounts.loggedin=2), and everything from the buddy/party/guild/
// family/messenger block - those subsystems arrive with P8.
func (cs *ChannelServer) loggedIn2(s *netw.Session, playerID int) {
	c := &client{cs: cs, sess: s}
	s.State.Store(c)

	p := cs.players.GetPendingCharacter(playerID)
	if p != nil {
		// Java ReconstructChr(transfer, c, true): rebuild without a DB
		// round-trip. The Go port re-attaches the session to the already
		// loaded player.
		p.Sess = s
	} else {
		chr, err := cs.loadCharacter(playerID)
		if err != nil {
			cs.log.Error("character load failed", "err", err, "playerID", playerID)
			s.Close("character load failed")
			return
		}
		if chr == nil {
			cs.log.Warn("PLAYER_LOGGEDIN for unknown character",
				"playerID", playerID, "remote", s.RemoteAddr)
			s.Close("unknown character")
			return
		}
		p = newPlayer(chr, s)
	}
	if p.Chr == nil {
		cs.log.Warn("PLAYER_LOGGEDIN for character without a row", "playerID", playerID)
		s.Close("character row missing")
		return
	}
	c.player = p

	// Java ChannelServer.forceRemovePlayerByAccId(c, c.getAccID()): a second
	// session of the same account (another channel) must not stay online.
	cs.srv.forceRemovePlayerByAccID(p.AccountID(), p)

	cs.players.RegisterPlayer(p)
	cs.log.Info("player logged in", "channel", cs.channel, "playerID", p.ID,
		"name", p.Name, "map", p.MapID(), "accID", p.AccountID())

	// Java ChannelServer.addPlayer appends the scrolling banner.
	if msg := cs.srv.cfg.ServerMessage; msg != "" {
		s.Write(packet.ServerMessagePacket(msg))
	}

	// c.sendPacket(MaplePacketCreator.getCharInfo(player)); then
	// c.sendPacket(MaplePacketCreator.temporaryStats_Reset()).
	s.Write(packet.CharInfoPacket(p.Chr, cs.channel, p.Rand))
	s.Write(packet.TemporaryStatsResetPacket())

	// player.getMap().addPlayer(player): registers the player and runs the
	// P4.3 spawn broadcast (everyone on the map sees the newcomer, the
	// newcomer sees everyone already there - MapleMap.addPlayer).
	cs.Map(p.MapID()).AddPlayer(p)
}

// loadCharacter reads the characters row for a channel session (Java
// MapleCharacter.loadCharFromDB channelserver path).
func (cs *ChannelServer) loadCharacter(id int) (*database.Character, error) {
	if cs.srv.store == nil {
		return nil, errNoStore
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cs.srv.store.GetCharacterByID(ctx, id)
}
