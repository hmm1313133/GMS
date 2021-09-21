package data

type ByteInputStream interface {
	readByte() int
	getBytesRead() int64
	available() int64
}
