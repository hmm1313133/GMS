// Package channel - P4.4 movement: MOVE_PLAYER (0x24) -> MovementParse ->
// broadcast movePlayer to the rest of the map + update the local position.
//
// Java sources (see docs/FILETRACK.md):
//   - handling/MapleServerHandler case MOVE_PLAYER -> handleMovePlayer
//   - handling/channel/handler/PlayerHandler.MovePlayer -> handleMovePlayer
//   - handling/channel/handler/MovementParse + server/movement -> internal/movement
package channel

import (
	"GMS/internal/movement"
	"GMS/internal/netw"
	"GMS/internal/packet"
	"GMS/internal/protocol"
)

// moveHeaderSkip is the v079 client prefix PlayerHandler.MovePlayer skips with
// slea.skip(33) before the movement list. The decompiled Zevms, the compare
// source and the 交流源码 all use the same 33.
const moveHeaderSkip = 33

// handleMovePlayer ports PlayerHandler.MovePlayer.
//
// Order kept:
//
//	r.skip(33)
//	moves = MovementParse.parseMovement(r, 1)        (drop on failure)
//	map.broadcastMessage(player, movePlayer(...), false)
//	MovementParse.updatePosition(moves, player, 0)
//	map.movePlayer(player, player.getPosition())
//	player.setOldPosition(player.getPosition())
//
// Skipped / deviated (documented):
//   - the 飞天检测 (fly-hack) map checks gate on a configvalues switch absent
//     from the 079-max2 dump (like the other configvalues gates).
//   - the `slea.available() < 13 || > 26` sanity check is not reproduced: it
//     is a validation heuristic whose trailing-byte window encodes v079 client
//     internals, and dropping a valid move on a wrong guess is worse than
//     forwarding it.
//   - the follow / clone re-broadcasts and the fall counter (needs footholds,
//     which arrive with P4.3b/P6).
func (h channelHandler) handleMovePlayer(s *netw.Session, r *protocol.Reader) {
	c, _ := s.State.Load().(*client)
	if c == nil || c.player == nil {
		return
	}
	p := c.player

	r.Skip(moveHeaderSkip)
	moves, ok := movement.Parse(r, 1)
	if !ok {
		h.cs.log.Debug("movement parse failed", "playerID", p.ID)
		return
	}

	// Java Map.broadcastMessage(player, movePlayer(...), false): everyone but
	// the mover. The boolean overload runs with an infinite range - the
	// maxViewRangeSq filtering lives in the Point overloads this handler does
	// not call.
	if m := h.cs.lookupMap(p.MapID()); m != nil {
		m.Broadcast(packet.MovePlayerPacket(p.ID, moves), p)
	}

	p.ApplyMovement(moves)
}
