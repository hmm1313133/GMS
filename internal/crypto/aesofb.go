package crypto

import (
	"crypto/aes"
	"encoding/binary"
)

// mapleAESKey is Java MapleAESOFB.skey (the cipher-side key; note the first
// byte 19 - MAPLE_AES_KEY exported with 20 is a different, unused-there quirk).
var mapleAESKey = []byte{
	19, 0, 0, 0, 8, 0, 0, 0, 6, 0, 0, 0, 180, 0, 0, 0,
	27, 0, 0, 0, 15, 0, 0, 0, 51, 0, 0, 0, 82, 0, 0, 0,
}

// MapleAESKey is Java MapleAESOFB.MAPLE_AES_KEY (public constant, first byte 20).
var MapleAESKey = []byte{
	20, 0, 0, 0, 8, 0, 0, 0, 6, 0, 0, 0, 180, 0, 0, 0,
	27, 0, 0, 0, 15, 0, 0, 0, 51, 0, 0, 0, 82, 0, 0, 0,
}

// funnyBytes is Java MapleAESOFB.funnyBytes (used by the IV update dance).
var funnyBytes = [256]byte{
	236, 63, 119, 164, 69, 208, 113, 191, 183, 152, 32, 252, 75, 233, 179, 225,
	92, 34, 247, 12, 68, 27, 129, 189, 99, 141, 212, 195, 242, 16, 25, 224,
	251, 161, 110, 102, 234, 174, 214, 206, 6, 24, 78, 235, 120, 149, 219, 186,
	182, 66, 122, 42, 131, 11, 84, 103, 109, 232, 101, 231, 47, 7, 243, 170,
	39, 123, 133, 176, 38, 253, 139, 169, 250, 190, 168, 215, 203, 204, 146, 218,
	249, 147, 96, 45, 221, 210, 162, 155, 57, 95, 130, 33, 76, 105, 248, 49,
	135, 238, 142, 173, 140, 106, 188, 181, 107, 89, 19, 241, 4, 0, 246, 90,
	53, 121, 72, 143, 21, 205, 151, 87, 18, 62, 55, 255, 157, 79, 81, 245,
	163, 112, 187, 20, 117, 194, 184, 114, 192, 237, 125, 104, 201, 46, 13, 98,
	70, 23, 17, 77, 108, 196, 126, 83, 193, 37, 199, 154, 28, 136, 88, 44,
	137, 220, 2, 100, 64, 1, 93, 56, 165, 226, 175, 85, 213, 239, 26, 124,
	167, 91, 166, 111, 134, 159, 115, 230, 10, 222, 43, 153, 74, 71, 156, 223,
	9, 118, 158, 48, 14, 228, 178, 148, 160, 59, 52, 29, 40, 15, 54, 227,
	35, 180, 3, 216, 144, 200, 60, 254, 94, 50, 36, 80, 31, 58, 67, 138,
	150, 65, 116, 172, 82, 51, 240, 217, 41, 128, 177, 22, 211, 171, 145, 185,
	132, 127, 97, 30, 207, 197, 209, 86, 61, 202, 244, 5, 198, 229, 8, 73,
}

// AESOFB is the Go port of Java tools.MapleAESOFB.
//
// Quirk faithfully kept from the original server (see
// handling/MapleServerHandler.java line 204): the send cipher is constructed
// with mapleVersion = -80, the receive cipher with 79. Both are byte-swapped
// at construction exactly like the Java code.
type AESOFB struct {
	iv      [4]byte
	block   interface{ Encrypt(dst, src []byte) }
	version uint16 // byte-swapped mapleVersion
}

// NewAESOFB mirrors `new MapleAESOFB(iv, mapleVersion)`.
func NewAESOFB(iv [4]byte, mapleVersion int16) *AESOFB {
	a, err := aes.NewCipher(mapleAESKey)
	if err != nil { // 32-byte key can't fail at runtime
		panic("gms: maple AES key invalid: " + err.Error())
	}
	return &AESOFB{
		iv:      iv,
		block:   a,
		version: byteSwap16(uint16(mapleVersion)),
	}
}

// IV returns the current IV (Java getIv).
func (a *AESOFB) IV() [4]byte { return a.iv }

// Crypt encrypts/decrypts in place (the transform is symmetric XOR) and then
// rolls the IV - Java MapleAESOFB.crypt.
//
// Chunking quirk kept: first chunk 1456 bytes, following chunks 1460; the IV
// stream (iv repeated 4x = 16 bytes, AES-chained every 16 bytes) RESTARTS from
// the base IV at each chunk boundary. IV update happens once, after all data.
func (a *AESOFB) Crypt(data []byte) {
	llength := 1456
	start := 0
	remaining := len(data)
	for remaining > 0 {
		myIv := MultiplyBytes(a.iv[:], 4, 4) // 16 bytes
		if remaining < llength {
			llength = remaining
		}
		for x := start; x < start+llength; x++ {
			if (x-start)%len(myIv) == 0 {
				var dst [16]byte
				a.block.Encrypt(dst[:], myIv)
				copy(myIv, dst[:])
			}
			data[x] ^= myIv[(x-start)%len(myIv)]
		}
		start += llength
		remaining -= llength // before llength reset (Java bytecode order: isub, then sipush 1460)
		llength = 1460
	}
	a.iv = GetNewIv(a.iv)
}

// PacketHeader builds the 4-byte frame header for a body of `length` bytes -
// Java getPacketHeader. Wire order: [iiv>>8, iiv, mlen>>8, mlen].
func (a *AESOFB) PacketHeader(length int) [4]byte {
	iiv := (int(a.iv[3]) | int(a.iv[2])<<8) ^ int(a.version)
	mlength := ((length << 8) & 0xFF00 | length >> 8) ^ iiv
	return [4]byte{
		byte(iiv >> 8 & 0xFF),
		byte(iiv & 0xFF),
		byte(mlength >> 8 & 0xFF),
		byte(mlength & 0xFF),
	}
}

// CheckPacket validates the first 2 bytes of a raw frame header against the
// current IV and version - Java checkPacket(byte[]).
func (a *AESOFB) CheckPacket(b0, b1 byte) bool {
	return int8(b0)^int8(a.iv[2]) == int8(a.version>>8) &&
		int8(b1)^int8(a.iv[3]) == int8(a.version&0xFF)
}

// GetNewIv rolls the IV via the funnyShit dance - Java MapleAESOFB.getNewIv.
func GetNewIv(oldIv [4]byte) [4]byte {
	in := [4]byte{0xF2, 0x53, 0x50, 0xC6} // {-14, 83, 80, -58}
	for x := 0; x < 4; x++ {
		funnyShit(oldIv[x], &in)
	}
	return in
}

// PacketLengthFromHeader decodes the body length from the 4 header bytes as
// one big-endian uint32 - Java getPacketLength(int).
func PacketLengthFromHeader(packetHeader uint32) int {
	packetLength := packetHeader>>16 ^ packetHeader&0xFFFF
	packetLength = packetLength<<8&0xFF00 | packetLength>>8
	return int(packetLength)
}

// HeaderAsUint32 is the inverse view used by the decoder: 4 header bytes read
// as big-endian (Netty readInt), matching Java semantics.
func HeaderAsUint32(h [4]byte) uint32 {
	return binary.BigEndian.Uint32(h[:])
}

func byteSwap16(v uint16) uint16 {
	// Java: (short)(v >> 8 & 0xFF | v << 8 & 0xFF00)
	return v>>8 | v<<8
}

// funnyShit is the IV update step - Java MapleAESOFB.funnyShit. All Java
// (byte) casts are reproduced via explicit uint8 truncation.
func funnyShit(inputByte byte, in *[4]byte) {
	elina := in[1]
	anna := inputByte
	var moritz byte

	moritz = funnyBytes[elina]
	moritz = moritz - inputByte // (byte)(moritz - inputByte)
	in[0] = in[0] + moritz

	moritz = in[2]
	moritz = moritz ^ funnyBytes[anna]
	elina = elina - moritz // (byte)(elina - (moritz & 0xFF))
	in[1] = elina

	moritz = in[3]
	elina = in[3]
	elina = elina - in[0] // (byte)(elina - (in[0] & 0xFF))
	moritz = funnyBytes[moritz]
	moritz = moritz + inputByte
	in[2] = moritz ^ in[2]
	in[3] = elina + funnyBytes[anna] // (byte)(elina + (funnyBytes[anna] & 0xFF))

	merry := uint32(in[0]) |
		uint32(in[1])<<8 |
		uint32(in[2])<<16 |
		uint32(in[3])<<24
	retValue := merry >> 29
	retValue |= merry << 3 // rotate-left-3
	in[0] = byte(retValue)
	in[1] = byte(retValue >> 8)
	in[2] = byte(retValue >> 16)
	in[3] = byte(retValue >> 24)
}
