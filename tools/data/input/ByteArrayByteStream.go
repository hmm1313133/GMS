package input

type ByteArrayByteStream struct {
	Pos       int64
	BytesRead int64
	Arr       []byte
}

func NewByteArrayByteStream(arr []byte) *ByteArrayByteStream {
	return &ByteArrayByteStream{
		Arr:       arr,
		Pos:       0,
		BytesRead: 0,
	}
}

func (ba *ByteArrayByteStream) GetPosition() int64 {
	return ba.Pos
}

func (ba *ByteArrayByteStream) Seek(n int) {
	ba.Pos = int64(n)
}

func (ba *ByteArrayByteStream) GetBytesRead() int64 {
	return ba.BytesRead
}

func (ba *ByteArrayByteStream) ReadByte() (byte, error) {
	ba.BytesRead++
	b := ba.Arr[ba.Pos] & 0xFF
	ba.Pos++
	return b, nil
}

func (ba *ByteArrayByteStream) ReadInt() (int, error) {
	ba.BytesRead++
	b := int(ba.Arr[ba.Pos]) & 0xFF
	ba.Pos++
	return b, nil
}

func (ba *ByteArrayByteStream) Available() int64 {
	return int64(len(ba.Arr)) - ba.Pos
}
