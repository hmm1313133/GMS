package data

import "bytes"

type InputStreamByteStream struct {
	Read int64
	Bs   *bytes.Reader
}

func (is *InputStreamByteStream) ReadByte() (temp byte, err error) {
	readByte, err := is.Bs.ReadByte()
	if err != nil {
		return 0, err
	}
	temp = readByte
	is.Read++
	return
}

func (is *InputStreamByteStream) GetBytesRead() (read int64) {
	read = is.Read
	return
}

func (is *InputStreamByteStream) Available() (read int64) {
	read = int64(is.Bs.Len())
	return
}

func NewInputStreamByteStream(bs *bytes.Reader) *InputStreamByteStream {
	is := &InputStreamByteStream{
		Bs:   bs,
		Read: 0,
	}
	return is
}
