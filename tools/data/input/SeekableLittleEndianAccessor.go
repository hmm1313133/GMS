package input

type SeekableLittleEndianAccessor interface {
	LittleEndianAccessor
	Seek(n int)
	GetPosition() int64
}
