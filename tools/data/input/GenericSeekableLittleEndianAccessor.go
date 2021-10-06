package input

type GenericSeekableLittleEndianAccessor struct {
	GenericLittleEndianAccessor
	Bs SeekableInputStreamByteStream
}

func (g *GenericSeekableLittleEndianAccessor) init(bs SeekableInputStreamByteStream) {
	g.Bs = bs
}

func (g *GenericSeekableLittleEndianAccessor) Seek(offset int) {
	g.Bs.Seek(offset)
}

func (g *GenericSeekableLittleEndianAccessor) GetPosition() int64 {
	return g.Bs.GetPosition()
}

func (g *GenericSeekableLittleEndianAccessor) Skip(n int) {
	g.Seek(int(g.GetPosition()) + n)
}
