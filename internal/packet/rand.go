package packet

import (
	"math/rand/v2"

	"GMS/internal/protocol"
)

// RandStream ports client.PlayerRandomStream: the CRand32 triple the client
// and server share to roll damage / miss checks. The channel server seeds one
// per character and serialises it into WARP_TO_MAP (Java
// MaplePacketCreator.getCharInfo -> chr.CRand().connectData).
//
// Faithful quirk (the ZEVMS decompile and the readable 交流源码 agree):
// connectData calls CRand32__Random() three times, but that method only reads
// seed1..3 and writes the "_" copies, so all three returns - and all three
// written ints - are identical. The ported code below reproduces exactly that
// instead of "fixing" it.
//
// The client's initial seed is random (Java Randomizer.nextLong), so the
// emitted bytes can never match the Java server value-for-value; what matters
// is that client and server later draw from the same stream.
type RandStream struct {
	// seed1..3 are the "public" seeds written by ConnectData's re-seed;
	// seed1c..3c are the copies CRand32__Random advances (Java seed1_, ...).
	seed1, seed2, seed3    uint32
	seed1c, seed2c, seed3c uint32
}

// NewRandStream ports the PlayerRandomStream constructor:
// CRand32__Seed(Randomizer.nextLong(), 803157710, 803157710). The two magic
// constants are 1170746341*5-755606699 with Java's 32-bit int overflow (the
// readable 交流源码 spells the overflowing expression out; the ZEVMS decompile
// already shows the wrapped value).
func NewRandStream() *RandStream {
	r := &RandStream{}
	r.seed(uint32(rand.Int64()), 803157710, 803157710)
	return r
}

// seed ports CRand32__Seed: each state gets the fixed bit its algorithm needs.
func (r *RandStream) seed(s1, s2, s3 uint32) {
	r.seed1, r.seed1c = s1|0x100000, s1|0x100000
	r.seed2, r.seed2c = s2|0x1000, s2|0x1000
	r.seed3, r.seed3c = s3|0x10, s3|0x10
}

// random ports CRand32__Random (the variant connectData uses).
func (r *RandStream) random() uint32 {
	v4, v5, v6 := r.seed1, r.seed2, r.seed3
	v8 := ((v4 & 0xFFFFFFFE) << 12) ^ (((r.seed1 & 0x7FFC0) ^ (v4 >> 13)) >> 6)
	v9 := (16 * (v5 & 0xFFFFFFF8)) ^ (((v5 >> 2) ^ (v5 & 0x3F800000)) >> 23)
	v10 := ((v6 & 0xFFFFFFF0) << 17) ^ (((v6 >> 3) ^ (v6 & 0x1FFFFF00)) >> 8)
	r.seed1c, r.seed2c, r.seed3c = v8, v9, v10
	return v8 ^ v9 ^ v10
}

// ConnectData ports PlayerRandomStream.connectData: draw three values (all
// identical - see the type doc), re-seed from them, then write the three ints.
func (r *RandStream) ConnectData(w *protocol.Writer) {
	a, b, c := r.random(), r.random(), r.random()
	r.seed(a, b, c)
	w.Int(int32(a))
	w.Int(int32(b))
	w.Int(int32(c))
}
