package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestReaderRoundTrip(t *testing.T) {
	w := NewWriter(64)
	w.Short(-2)
	w.Int(0x01020304)
	w.Long(0x0102030405060708)
	w.Byte(0xAB)
	w.Bool(true)
	w.MapleAsciiString("你好ABC")
	w.Pos(-100, 200)

	r := NewReader(w.Bytes())
	if got := r.Short(); got != -2 {
		t.Fatalf("Short = %d", got)
	}
	if got := r.Int(); got != 0x01020304 {
		t.Fatalf("Int = %X", got)
	}
	if got := r.Long(); got != 0x0102030405060708 {
		t.Fatalf("Long = %X", got)
	}
	if got := r.Byte(); got != 0xAB {
		t.Fatalf("Byte = %X", got)
	}
	if got := r.Byte(); got != 1 {
		t.Fatalf("Bool byte = %d", got)
	}
	if got := r.MapleAsciiString(); got != "你好ABC" {
		t.Fatalf("MapleAsciiString = %q", got)
	}
	x, y := r.Pos()
	if x != -100 || y != 200 {
		t.Fatalf("Pos = %d,%d", x, y)
	}
	if r.Err != nil {
		t.Fatalf("unexpected err: %v", r.Err)
	}
	if r.Len() != 0 {
		t.Fatalf("remaining %d", r.Len())
	}
}

func TestReaderOverrun(t *testing.T) {
	r := NewReader([]byte{1, 2})
	_ = r.Int() // only 2 bytes
	if r.Err == nil {
		t.Fatal("expected overrun error")
	}
	if got := r.Int(); got != 0 {
		t.Fatalf("post-error reads must be zero, got %d", got)
	}
	if got := r.MapleAsciiString(); got != "" {
		t.Fatalf("post-error string = %q", got)
	}
}

func TestGB18030Strings(t *testing.T) {
	// 好你 in GB18030
	gb := []byte{0xBA, 0xC3, 0xC4, 0xE3}
	if got := GB18030(gb); got != "好你" {
		t.Fatalf("GB18030 = %q", got)
	}
	back := EncodeGB18030("好你")
	if !bytes.Equal(back, gb) {
		t.Fatalf("roundtrip = % X", back)
	}

	// fixed-width padded string
	w := NewWriter(8)
	w.AsciiStringMax("AB", 4)
	if got := hex.EncodeToString(w.Bytes()); !strings.EqualFold(got, "41420000") {
		t.Fatalf("padded = %s", got)
	}

	// truncation
	w2 := NewWriter(4)
	w2.AsciiStringMax("abcdef", 3)
	if len(w2.Bytes()) != 3 {
		t.Fatalf("truncated len = %d", len(w2.Bytes()))
	}
}

func TestWriterHexes(t *testing.T) {
	w := NewWriter(16)
	w.Short(0x1234)
	w.Int(0x0A0B0C0D)
	// Java writeReversedInt(l): bytes (l>>>32)&FF, (l>>>40)&FF, (l>>>48)&FF, (l>>>56)&FF
	// For l = 0x0011223344556677: 33 22 11 00
	w.ReversedInt(0x0011223344556677)
	got := strings.ToUpper(hex.EncodeToString(w.Bytes()))
	want := "34120D0C0B0A33221100"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestOpWriter(t *testing.T) {
	w := NewWriter(8)
	w.Op(SendPING)
	if got := w.Bytes(); len(got) != 2 || got[0] != 0x14 || got[1] != 0x00 {
		t.Fatalf("op bytes = % X", got)
	}
}
