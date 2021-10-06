package input

type SeekableInputStreamByteStream interface {
	ByteInputStream
	Seek(n int)
	GetPosition() int64
}
