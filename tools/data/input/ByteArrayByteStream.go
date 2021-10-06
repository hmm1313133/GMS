package input

type ByteArrayByteStream struct {
	pos       int64
	BytesRead int64
	Arr       []byte
}

func (ba *ByteArrayByteStream) GetPosition() int64 {
	return ba.pos
}

func (ba *ByteArrayByteStream) Seek(n int) {
	ba.pos = int64(n)
}

func (ba *ByteArrayByteStream) GetBytesRead() int64 {
	return ba.BytesRead
}

func (ba *ByteArrayByteStream) ReadByte() (byte, error) {
	ba.BytesRead++
	b := ba.Arr[ba.pos] & 0xFF
	ba.pos++
	return b, nil
}

func (ba *ByteArrayByteStream) ReadInt() (int, error) {
	ba.BytesRead++
	b := int(ba.Arr[ba.pos]) & 0xFF
	ba.pos++
	return b, nil
}
