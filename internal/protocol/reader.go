package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Reader is the Go port of Java tools.data.LittleEndianAccessor over a packet
// body (post-decryption). Reads are bounds-checked and panic-free: exhausted
// reads return zero values with ErrConsumed recorded (the Java version
// returned garbage; we prefer loud-but-safe).
type Reader struct {
	buf   []byte
	off   int
	Err   error // first read-past-end error; handlers check this
}

// NewReader wraps a decrypted packet body.
func NewReader(b []byte) *Reader { return &Reader{buf: b} }

// Len returns remaining unread bytes.
func (r *Reader) Len() int { return len(r.buf) - r.off }

func (r *Reader) fail(n int) {
	if r.Err == nil {
		r.Err = fmt.Errorf("read %d at offset %d beyond %d", n, r.off, len(r.buf))
	}
}

// Bytes returns the unread remainder (does not advance).
func (r *Reader) Bytes() []byte { return r.buf[r.off:] }

// Byte reads one byte.
func (r *Reader) Byte() byte {
	if r.Len() < 1 {
		r.fail(1)
		return 0
	}
	v := r.buf[r.off]
	r.off++
	return v
}

// SByte reads one byte as int8 (Java readByte).
func (r *Reader) SByte() int8 { return int8(r.Byte()) }

// Short reads int16 LE (Java readShort).
func (r *Reader) Short() int16 {
	if r.Len() < 2 {
		r.fail(2)
		return 0
	}
	v := int16(binary.LittleEndian.Uint16(r.buf[r.off:]))
	r.off += 2
	return v
}

// UShort reads uint16 as int (Java readShortAsInt / readUShort).
func (r *Reader) UShort() int {
	if r.Len() < 2 {
		r.fail(2)
		return 0
	}
	v := int(binary.LittleEndian.Uint16(r.buf[r.off:]))
	r.off += 2
	return v
}

// Int reads int32 LE.
func (r *Reader) Int() int32 {
	if r.Len() < 4 {
		r.fail(4)
		return 0
	}
	v := int32(binary.LittleEndian.Uint32(r.buf[r.off:]))
	r.off += 4
	return v
}

// Long reads int64 LE.
func (r *Reader) Long() int64 {
	if r.Len() < 8 {
		r.fail(8)
		return 0
	}
	v := int64(binary.LittleEndian.Uint64(r.buf[r.off:]))
	r.off += 8
	return v
}

// Float / Double read IEEE754 values (Java readFloat/readDouble).
func (r *Reader) Float() float32  { return math.Float32frombits(r.UInt()) }
func (r *Reader) Double() float64 { return math.Float64frombits(r.ULong()) }

// UInt / ULong are unsigned views.
func (r *Reader) UInt() uint32 {
	if r.Len() < 4 {
		r.fail(4)
		return 0
	}
	v := binary.LittleEndian.Uint32(r.buf[r.off:])
	r.off += 4
	return v
}
func (r *Reader) ULong() uint64 {
	if r.Len() < 8 {
		r.fail(8)
		return 0
	}
	v := binary.LittleEndian.Uint64(r.buf[r.off:])
	r.off += 8
	return v
}

// Read reads n raw bytes (Java read(num)).
func (r *Reader) Read(n int) []byte {
	if n < 0 || r.Len() < n {
		r.fail(n)
		if n < 0 {
			n = 0
		}
		if r.Len() < n {
			n = r.Len()
		}
	}
	v := r.buf[r.off : r.off+n]
	r.off += n
	return v
}

// Pos reads a 2-short position (Java readPos -> Point).
func (r *Reader) Pos() (x, y int16) {
	x = r.Short()
	y = r.Short()
	return
}

// MapleAsciiString reads short-length + bytes (Java readMapleAsciiString).
func (r *Reader) MapleAsciiString() string {
	n := int(r.Short())
	if n < 0 || r.Len() < n {
		r.fail(n)
		return ""
	}
	return GB18030(r.Read(n))
}

// AsciiString reads n bytes as string (Java readAsciiString).
func (r *Reader) AsciiString(n int) string {
	if n < 0 || r.Len() < n {
		r.fail(n)
		return ""
	}
	return GB18030(r.Read(n))
}

// Skip advances n bytes.
func (r *Reader) Skip(n int) { r.Read(n) }

// GB18030 decodes like Java new String(bytes, "GB18030").
func GB18030(b []byte) string { return charsetGB18030().Decode(b) }
