package data

import (
	"bytes"
	"strconv"
)

type GenericLittleEndianAccessor struct {
	Bs *bytes.Reader
}

func (g *GenericLittleEndianAccessor) ReadByteAsInt() int {
	temp, _ := g.Bs.ReadByte()
	return int(temp)
}

func (g *GenericLittleEndianAccessor) ReadByte() (temp byte, err error) {
	readByte, err := g.Bs.ReadByte()
	if err != nil {
		return 0, err
	}
	temp = readByte
	return
}

func (g *GenericLittleEndianAccessor) ReadChar() string {
	return strconv.Itoa(g.ReadShort())
}

func (g *GenericLittleEndianAccessor) ReadShort() int {
	byte1, _ := g.Bs.ReadByte()
	byte2, _ := g.Bs.ReadByte()
	return (int(byte2) << 8) + int(byte1)
}

func (g *GenericLittleEndianAccessor) ReadInt() int {
	byte1, _ := g.Bs.ReadByte()
	byte2, _ := g.Bs.ReadByte()
	byte3, _ := g.Bs.ReadByte()
	byte4, _ := g.Bs.ReadByte()
	return (int(byte4) << 24) + (int(byte3) << 16) + (int(byte2) << 8) + int(byte1)
}

func (g *GenericLittleEndianAccessor) ReadLong() int64 {
	byte1, _ := g.Bs.ReadByte()
	byte2, _ := g.Bs.ReadByte()
	byte3, _ := g.Bs.ReadByte()
	byte4, _ := g.Bs.ReadByte()
	byte5, _ := g.Bs.ReadByte()
	byte6, _ := g.Bs.ReadByte()
	byte7, _ := g.Bs.ReadByte()
	byte8, _ := g.Bs.ReadByte()
	return (int64(byte8) << 56) + (int64(byte7) << 48) + (int64(byte6) << 40) + (int64(byte5) << 32) +
		(int64(byte4) << 24) + (int64(byte3) << 16) + (int64(byte2) << 8) + int64(byte1)
}

func (g *GenericLittleEndianAccessor) Skip(n int) {
	for x := 0; x < n; x++ {
		_, err := g.ReadByte()
		if err != nil {
			return
		}
	}
	return
}

func (g *GenericLittleEndianAccessor) Read(n int) []byte {
	var ret = make([]byte, n)
	for x := 0; x < n; x++ {
		ret[x], _ = g.ReadByte()
	}
	return ret
}

func (g *GenericLittleEndianAccessor) ReadAsciiString(n int) string {
	var ret = make([]byte, n)
	for i := 0; i < n; i++ {
		temp, _ := g.Bs.ReadByte()
		ret[i] = temp
	}
	return string(ret)
}

func (g *GenericLittleEndianAccessor) ReadMapleAsciiString() string {
	return g.ReadAsciiString(g.ReadShort())
}

func (g *GenericLittleEndianAccessor) ReadPos() (int, int) {
	x := g.ReadShort()
	y := g.ReadShort()
	return x, y
}

func (g *GenericLittleEndianAccessor) Available() int {
	return g.Bs.Len()
}

func NewGenericLittleEndianAccessor(bs *bytes.Reader) *InputStreamByteStream {
	is := &InputStreamByteStream{
		Bs:   bs,
		Read: 0,
	}
	return is
}
