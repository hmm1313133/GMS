package login

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"strings"
	"unicode/utf16"
)

// LoginCrypto ports client/LoginCrypto.java (SHA-1 / salted SHA-512) and
// client/LoginCryptoLegacy.java (phpBB-style $H$ hashes) for P2.2 account
// password verification. Java sources:
//   - client/LoginCrypto.java        -> hexSha1 / salted SHA-512
//   - client/LoginCryptoLegacy.java  -> legacyCheckPassword / isLegacyPassword

// javaStrLen is Java String.length(): the number of UTF-16 code units.
func javaStrLen(s string) int { return len(utf16.Encode([]rune(s))) }

// hashWithDigest ports Java LoginCrypto.hashWithDigest, INCLUDING its
// truncation quirk: Digester.update(in.getBytes("UTF-8"), 0, in.length())
// digests only the first in.length() BYTES of the UTF-8 encoding (string
// length = UTF-16 units, not byte length). Identical for ASCII; for CJK it
// hashes a prefix. (The golden sha1_中文 vector additionally carries
// generator-side mojibake - see crypto_test.go.)
func hashWithDigest(in string, digest func([]byte) []byte) []byte {
	b := []byte(in) // UTF-8
	n := javaStrLen(in)
	if n > len(b) { // Java would throw AIOOBE; clamp defensively
		n = len(b)
	}
	return digest(b[:n])
}

func hexSHA1(in string) string {
	sum := hashWithDigest(in, func(b []byte) []byte {
		s := sha1.Sum(b)
		return s[:]
	})
	return hex.EncodeToString(sum)
}

func hexSHA512(in string) string {
	sum := hashWithDigest(in, func(b []byte) []byte {
		s := sha512.Sum512(b)
		return s[:]
	})
	return hex.EncodeToString(sum)
}

// checkSHA1Hash mirrors LoginCrypto.checkSha1Hash.
func checkSHA1Hash(hash, password string) bool {
	return hash == hexSHA1(password)
}

// makeSaltedSHA512Hash mirrors LoginCrypto.makeSaltedSha512Hash.
func makeSaltedSHA512Hash(password, salt string) string {
	return hexSHA512(password + salt)
}

// checkSaltedSHA512Hash mirrors LoginCrypto.checkSaltedSha512Hash.
func checkSaltedSHA512Hash(hash, password, salt string) bool {
	return hash == makeSaltedSHA512Hash(password, salt)
}

// iota64 is the LoginCryptoLegacy base64 alphabet.
var iota64 = []byte("./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// isLegacyPassword mirrors LoginCryptoLegacy.isLegacyPassword.
func isLegacyPassword(hash string) bool {
	return strings.HasPrefix(hash, "$H$")
}

// legacyCheckPassword mirrors LoginCryptoLegacy.checkPassword.
func legacyCheckPassword(password, hash string) bool {
	return legacyMyCrypt(password, hash) == hash
}

// legacyMyCrypt ports LoginCryptoLegacy.myCrypt(password, seed).
func legacyMyCrypt(password, seed string) string {
	if !strings.HasPrefix(seed, "$H$") {
		return ""
	}
	if len(seed) < 12 {
		return ""
	}
	salt := seed[4:12]
	if len(salt) != 8 {
		return ""
	}
	// Java: (salt + password).getBytes("iso-8859-1") - the whole byte string.
	pw := iso8859Bytes(password)
	saltPw := append(iso8859Bytes(salt), pw...)
	digest := sha1.Sum(saltPw)
	h := digest[:]
	// Java do { h = sha1(h + pw) } while (--count > 0) with count=8:
	// 8 more rounds after the initial digest (9 SHA-1s total).
	for count := 8; count > 0; count-- {
		combined := make([]byte, len(h)+len(pw))
		copy(combined, h)
		copy(combined[len(h):], pw)
		sum := sha1.Sum(combined)
		h = sum[:]
	}
	return seed[:12] + legacyEncode64(h)
}

// iso8859Bytes mirrors Java String.getBytes("iso-8859-1"): each UTF-16 code
// unit maps to its byte value, unmappable units (surrogates etc.) become '?'.
func iso8859Bytes(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, len(units))
	for i, u := range units {
		if u <= 0xFF {
			out[i] = byte(u)
		} else {
			out[i] = '?' // 0x3F, Java's default replacement
		}
	}
	return out
}

// legacyEncode64 ports LoginCryptoLegacy.encode64.
func legacyEncode64(input []byte) string {
	iLen := len(input)
	oDataLen := (iLen*4 + 2) / 3
	oLen := ((iLen + 2) / 3) * 4
	out := make([]byte, oLen)
	ip, op := 0, 0
	for ip < iLen {
		i0 := int(input[ip])
		ip++
		i1 := 0
		if ip < iLen {
			i1 = int(input[ip])
			ip++
		}
		i2 := 0
		if ip < iLen {
			i2 = int(input[ip])
			ip++
		}
		o0 := i0 >> 2
		o1 := ((i0 & 3) << 4) | (i1 >> 4)
		o2 := ((i1 & 0xf) << 2) | (i2 >> 6)
		o3 := i2 & 0x3F
		out[op] = iota64[o0]
		op++
		out[op] = iota64[o1]
		op++
		if op < oDataLen {
			out[op] = iota64[o2]
		} else {
			out[op] = '='
		}
		op++
		if op < oDataLen {
			out[op] = iota64[o3]
		} else {
			out[op] = '='
		}
		op++
	}
	return string(out)
}
