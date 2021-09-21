package data

type LittleEndianAccessor interface {
	ReadByte() (byte, error)
	ReadByteAsInt() int
	ReadChar() string
	ReadShort() int
	ReadInt() int
	ReadLong() int64
	Skip(n int)
	Read(n int) []byte
	ReadAsciiString(n int) string
	ReadMapleAsciiString() string
	ReadPos() (int, int)
	Available() int
}
