package crypto

// RollLeft is Java BitTools.rollLeft: rotate an 8-bit value left.
//
// Java 32-bit int semantics reproduced exactly: shift distance is masked to
// 5 bits (Java int-shift rule), overflow wraps, and the final >>8 is an
// arithmetic shift on int32. Production callers only pass 0..255 counts;
// negative behavior is preserved for fidelity with the original.
func RollLeft(in byte, count int) byte {
	tmp := int32(in)
	tmp <<= (int32(count%8) & 0x1F)
	return byte(tmp&0xFF | tmp>>8)
}

// RollRight is Java BitTools.rollRight: rotate an 8-bit value right.
// `<< 8 >>> count` in Java: unsigned shift, so uint32 here.
func RollRight(in byte, count int) byte {
	u := uint32(in) << 8
	u >>= (uint32(int32(count%8)) & 0x1F)
	return byte(u&0xFF | u>>8)
}

// MultiplyBytes is Java BitTools.multiplyBytes(in, count, mul):
// the first `count` bytes of in, repeated `mul` times.
func MultiplyBytes(in []byte, count, mul int) []byte {
	ret := make([]byte, count*mul)
	for x := 0; x < count*mul; x++ {
		ret[x] = in[x%count]
	}
	return ret
}
