package input

type ByteInputStream interface {
	ReadByte() (byte, error)
	ReadInt() (int, error)
	GetBytesRead() int64
	Available() int64
}
