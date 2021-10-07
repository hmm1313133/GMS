package input

import "bytes"

type GenericSeekableLittleEndianAccessor struct {
	GenericLittleEndianAccessor
	Si SeekableInputStreamByteStream
}

func NewGenericSeekableLittleEndianAccessor(bs *bytes.Reader, si SeekableInputStreamByteStream) *GenericSeekableLittleEndianAccessor {
	gslea := GenericSeekableLittleEndianAccessor{
		Si: si,
	}
	gslea.Bs = bs
	return &gslea
}

func (g *GenericSeekableLittleEndianAccessor) init(si SeekableInputStreamByteStream) {
	g.Si = si
}

func (g *GenericSeekableLittleEndianAccessor) Seek(offset int) {
	g.Si.Seek(offset)
}

func (g *GenericSeekableLittleEndianAccessor) GetPosition() int64 {
	return g.Si.GetPosition()
}

func (g *GenericSeekableLittleEndianAccessor) Skip(n int) {
	g.Seek(int(g.GetPosition()) + n)
}
