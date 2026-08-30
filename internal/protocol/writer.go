package protocol

import (
	"encoding/binary"
	"math"
)

// Writer is the Go port of Java tools.data.MaplePacketLittleEndianWriter.
// Packet bodies are built little-endian; strings are GB18030 (Java ASCII
// constant = Charset.forName("GB18030") quirk, kept for wire fidelity).
type Writer struct {
	buf []byte
}

// NewWriter returns a Writer with initial capacity.
func NewWriter(size int) *Writer {
	if size <= 0 {
		size = 32
	}
	return &Writer{buf: make([]byte, 0, size)}
}

// Bytes returns the written body (Java getPacket).
func (w *Writer) Bytes() []byte { return w.buf }

// Len returns bytes written so far.
func (w *Writer) Len() int { return len(w.buf) }

func (w *Writer) write(b ...byte) { w.buf = append(w.buf, b...) }

// Write appends raw bytes (Java write(byte[])).
func (w *Writer) Write(b []byte) { w.write(b...) }

// Byte appends one byte.
func (w *Writer) Byte(b byte) { w.write(b) }

// IntAsByte appends low 8 bits (Java write(int)).
func (w *Writer) IntAsByte(v int) { w.write(byte(v)) }

// Bool appends 1/0 (Java write(boolean)).
func (w *Writer) Bool(b bool) {
	if b {
		w.write(1)
	} else {
		w.write(0)
	}
}

// Zero writes n zero bytes (Java writeZeroBytes).
func (w *Writer) Zero(n int) {
	for i := 0; i < n; i++ {
		w.write(0)
	}
}

// Short writes int16 LE, truncating from int (Java writeShort(int)).
func (w *Writer) Short(v int) {
	w.write(byte(v), byte(v>>8))
}

// UShort writes uint16 LE.
func (w *Writer) UShort(v uint16) { w.Short(int(v)) }

// Int writes int32 LE.
func (w *Writer) Int(v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	w.write(b[:]...)
}

// UInt writes uint32 LE.
func (w *Writer) UInt(v uint32) { w.Int(int32(v)) }

// Long writes int64 LE.
func (w *Writer) Long(v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	w.write(b[:]...)
}

// ReversedInt writes the high 4 bytes of a long in ascending shift order
// (Java writeReversedInt - legacy quirk used by some packets).
func (w *Writer) ReversedInt(v int64) {
	w.write(
		byte(v>>32),
		byte(v>>40),
		byte(v>>48),
		byte(v>>56),
	)
}

// Float / Double write IEEE754 LE.
func (w *Writer) Float(v float32)  { w.UInt(math.Float32bits(v)) }
func (w *Writer) Double(v float64) { w.ULong(math.Float64bits(v)) }

// ULong writes uint64 LE.
func (w *Writer) ULong(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	w.write(b[:]...)
}

// AsciiString writes GB18030 bytes without length prefix.
func (w *Writer) AsciiString(s string) { w.Write(EncodeGB18030(s)) }

// AsciiStringMax writes exactly max bytes: truncated string + zero padding
// (Java writeAsciiString(String, int)).
func (w *Writer) AsciiStringMax(s string, max int) {
	b := EncodeGB18030(s)
	if len(b) > max {
		b = b[:max]
	}
	w.Write(b)
	for i := len(b); i < max; i++ {
		w.write(0)
	}
}

// MapleAsciiString writes short length + GB18030 bytes
// (Java writeMapleAsciiString).
func (w *Writer) MapleAsciiString(s string) {
	b := EncodeGB18030(s)
	w.Short(len(b))
	w.Write(b)
}

// Pos writes a Point (Java writePos).
func (w *Writer) Pos(x, y int16) {
	w.Short(int(x))
	w.Short(int(y))
}

// Op writes a send-opcode header (short) at the buffer start - helpers in
// the packet package call this first.
func (w *Writer) Op(op SendOp) { w.Short(int(op)) }
