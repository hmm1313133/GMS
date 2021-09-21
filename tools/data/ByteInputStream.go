package data

type ByteInputStream interface {
	ReadByte() (byte, error)
	GetBytesRead() int64
	Available() int64
}
