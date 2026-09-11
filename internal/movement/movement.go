// Package movement is the Go port of Java's server.movement package (the
// LifeMovementFragment tree) plus handling/channel/handler/MovementParse: the
// parse + rebroadcast of the client's MOVE_PLAYER movement list.
//
// Java sources (see docs/FILETRACK.md):
//   - server/movement/LifeMovementFragment   -> Fragment
//   - server/movement/StaticLifeMovement     -> Fragment.Serialize
//   - handling/channel/handler/MovementParse -> Parse / UpdatePosition
//   - tools/packet/PacketHelper.serializeMovementList -> SerializeMovementList
package movement

import "GMS/internal/protocol"

// Point is a 2D map coordinate (Java java.awt.Point; only x/y are used).
type Point struct{ X, Y int16 }

// Fragment is one command in a movement path. Java has a LifeMovement class
// per command family, but this source tree only ever instantiates
// StaticLifeMovement, so the Go port keeps a single struct.
//
// Field notes (Java quirks preserved - do not "fix"):
//   - NewFh is AbstractLifeMovement.newfh, i.e. the *fifth constructor
//     argument* of StaticLifeMovement. MovementParse passes `unk` for the
//     0/5/15/17 group and 0 for every other group, so NewFh is the foothold
//     MovementParse.updatePosition applies - it is NOT the serialised Fh.
//   - Duration is discarded (kept 0) for the 3/4/7/8/9/11 group: the parse
//     reads the short but constructs the fragment with duration 0, so the
//     rebroadcast writes 0 rather than the client's value.
type Fragment struct {
	Type     int
	Pos      *Point // set for the commands carrying an absolute position
	Wobble   *Point // Java pixelsPerSecond (relative velocity)
	Unk      int
	Fh       int // foothold serialised for types 14/15
	Wui      int // type 10 payload
	NewState int
	Duration int
	NewFh    int
}

// Parse ports MovementParse.parseMovement(lea, kind): read the command count
// and every fragment. ok is false where Java returns null (a truncated packet)
// or where the declared count does not match what was parsed.
//
// kind is 1 for a player (2 mob / 3 pet / 4 summon / 5 dragon); the parse shape
// is identical for every kind in this version.
func Parse(r *protocol.Reader, kind int) ([]Fragment, bool) {
	_ = kind
	// Java reads a signed byte and loops `for (byte i = 0; i < numCommands;
	// i++)`, so a count >= 128 never enters the loop and then fails the
	// `numCommands != res.size()` check.
	n := int(r.SByte())
	res := make([]Fragment, 0, 8)
	for i := 0; i < n; i++ {
		command := int(r.Byte())
		switch command {
		case 0, 5, 15, 17:
			x, y := r.Short(), r.Short()
			xw, yw := r.Short(), r.Short()
			unk := int(r.Short())
			fh := 0
			if command == 15 {
				fh = int(r.Short())
			}
			ns := int(r.Byte())
			dur := int(r.Short())
			res = append(res, Fragment{
				Type: command, Pos: &Point{x, y}, Wobble: &Point{xw, yw},
				Unk: unk, Fh: fh, NewState: ns, Duration: dur, NewFh: unk,
			})
		case 1, 2, 6, 12, 13, 16, 18, 19, 22:
			xw, yw := r.Short(), r.Short()
			ns := int(r.Byte())
			dur := int(r.Short())
			res = append(res, Fragment{
				Type: command, Wobble: &Point{xw, yw}, NewState: ns, Duration: dur,
			})
		case 3, 4, 7, 8, 9, 11:
			x, y := r.Short(), r.Short()
			unk := int(r.Short())
			ns := int(r.Byte())
			_ = r.Short() // duration: read but discarded (Java builds with 0)
			res = append(res, Fragment{
				Type: command, Pos: &Point{x, y}, Unk: unk, NewState: ns,
			})
		case 10:
			res = append(res, Fragment{Type: command, Wui: int(r.Byte())})
		case 14:
			xw, yw := r.Short(), r.Short()
			fh := int(r.Short())
			ns := int(r.Byte())
			dur := int(r.Short())
			res = append(res, Fragment{
				Type: command, Wobble: &Point{xw, yw}, Fh: fh,
				NewState: ns, Duration: dur,
			})
		default:
			ns := int(r.Byte())
			dur := int(r.Short())
			res = append(res, Fragment{Type: command, NewState: ns, Duration: dur})
		}
	}
	if r.Err != nil || n != len(res) {
		return nil, false
	}
	return res, true
}

// Serialize ports StaticLifeMovement.serialize.
func (f *Fragment) Serialize(w *protocol.Writer) {
	w.Byte(byte(f.Type))
	switch f.Type {
	case 0, 5, 15, 17:
		x, y := point(f.Pos)
		w.Pos(x, y)
		xw, yw := point(f.Wobble)
		w.Pos(xw, yw)
		w.Short(f.Unk)
		if f.Type == 15 {
			w.Short(f.Fh)
		}
	case 1, 2, 6, 12, 13, 16, 18, 19, 22:
		xw, yw := point(f.Wobble)
		w.Pos(xw, yw)
	case 3, 4, 7, 8, 9, 11:
		x, y := point(f.Pos)
		w.Pos(x, y)
		w.Short(f.Unk)
	case 14:
		xw, yw := point(f.Wobble)
		w.Pos(xw, yw)
		w.Short(f.Fh)
	}
	if f.Type != 10 {
		w.Byte(byte(f.NewState))
		w.Short(f.Duration)
	} else {
		w.Byte(byte(f.Wui))
	}
}

// SerializeMovementList ports PacketHelper.serializeMovementList: a length
// byte followed by each fragment.
func SerializeMovementList(w *protocol.Writer, moves []Fragment) {
	w.Byte(byte(len(moves)))
	for i := range moves {
		moves[i].Serialize(w)
	}
}

// Target is the position surface of Java AnimatedMapleMapObject that
// MovementParse.updatePosition drives.
type Target interface {
	SetPosition(x, y int16)
	SetFh(fh int)
	SetStance(stance int)
}

// UpdatePosition ports MovementParse.updatePosition: every fragment updates
// the foothold and stance (so the last one wins), and the last fragment that
// carries an absolute position moves the target.
func UpdatePosition(moves []Fragment, t Target, yoffset int) {
	for i := range moves {
		m := &moves[i]
		if m.Pos != nil {
			t.SetPosition(m.Pos.X, int16(int(m.Pos.Y)+yoffset))
		}
		t.SetFh(m.NewFh)
		t.SetStance(m.NewState)
	}
}

func point(p *Point) (int16, int16) {
	if p == nil {
		return 0, 0
	}
	return p.X, p.Y
}
