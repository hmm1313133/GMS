package login

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestLoginCryptoGolden replays every vector produced by
// tools/goldengen/LoginCryptoGen.java running against the ORIGINAL 079MAX2.jar
// (K:\079MAX2服务端\dist) - the P2.2 LoginCrypto byte-exactness proof.
//
// Golden file: testdata/golden_login.txt, one `key=value` per line (values
// have no spaces; keys may contain CJK; the legacy hash is one random draw
// because Java seeded with currentTimeMillis - Go only verifies, never
// generates, so one fixed vector is enough).
func TestLoginCryptoGolden(t *testing.T) {
	golden, err := loadLoginGolden()
	if err != nil {
		t.Skipf("golden vectors unavailable: %v", err)
	}
	if len(golden) < 9 {
		t.Fatalf("golden file suspiciously small: %d lines", len(golden))
	}

	want := func(key string) string {
		v, ok := golden[key]
		if !ok {
			t.Errorf("golden line %q missing", key)
			return ""
		}
		return v
	}

	// 1. LoginCrypto.hexSha1 - Java digests UTF-8 bytes but only the first
	// in.length() BYTES (Java String.length() = UTF-16 units): identical for
	// ASCII, a prefix for CJK.
	//
	// The CJK golden vector's actual input is NOT plain "中文": the golden
	// generator (LoginCryptoGen.java, UTF-8 source, GBK-default javac) baked
	// in the mojibake of "中文" read-as-GBK - "涓\uFFFD枃", 3 UTF-16 units /
	// 9 UTF-8 bytes. Replayed verbatim it pins both quirks: UTF-8 byte
	// hashing + the length() truncation (first 3 of 9 bytes, i.e. sha1 of
	// "涓"). A real CJK password "中文" would hash sha1(E4 B8) instead.
	sha1Cases := []struct{ key, in string }{
		{"sha1_goldenpassword", "goldenpassword"},
		{"sha1_empty", ""},
		{"sha1_中文", "涓\uFFFD枃"},
	}
	for _, c := range sha1Cases {
		got := hexSHA1(c.in)
		if got != want(c.key) {
			t.Errorf("hexSHA1(%q) mismatch:\n got  %s\n want %s", c.in, got, golden[c.key])
		}
	}

	// 2. LoginCrypto.checkSha1Hash / makeSaltedSha1Hash (same function).
	if !checkSHA1Hash(want("sha1_goldenpassword"), "goldenpassword") {
		t.Error("checkSHA1Hash must accept the golden sha1 vector")
	}
	if checkSHA1Hash(want("sha1_goldenpassword"), "wrong") {
		t.Error("checkSHA1Hash must reject a wrong password")
	}

	// 3. LoginCrypto.makeSaltedSha512Hash = hexSha512(password + salt).
	// The key `sha512_empty_salt` is misleadingly named: the Java generator
	// hashed the EMPTY PASSWORD with the same 32-char salt. Kept as-is for
	// traceability with LoginCryptoGen.java.
	const salt = "0123456789abcdef0123456789abcdef"
	if got := makeSaltedSHA512Hash("goldenpassword", salt); got != want("sha512_goldenpassword_0123456789abcdef0123456789abcdef") {
		t.Errorf("makeSaltedSHA512Hash(pw,salt) mismatch:\n got  %s\n want %s", got, golden["sha512_goldenpassword_0123456789abcdef0123456789abcdef"])
	}
	if got := makeSaltedSHA512Hash("", salt); got != want("sha512_empty_salt") {
		t.Errorf("makeSaltedSHA512Hash(\"\",salt) mismatch:\n got  %s\n want %s", got, golden["sha512_empty_salt"])
	}
	if !checkSaltedSHA512Hash(want("sha512_goldenpassword_0123456789abcdef0123456789abcdef"), "goldenpassword", salt) {
		t.Error("checkSaltedSHA512Hash must accept the golden sha512 vector")
	}
	if checkSaltedSHA512Hash(want("sha512_goldenpassword_0123456789abcdef0123456789abcdef"), "wrong", salt) {
		t.Error("checkSaltedSHA512Hash must reject a wrong password")
	}

	// 4. LoginCryptoLegacy: $H$c + 8-char salt + 28-char encode64 digest.
	legacyHash, pw := want("legacy_hash"), want("legacy_pw")
	if got := strconv.FormatBool(legacyCheckPassword(pw, legacyHash)); got != want("legacy_check_ok") {
		t.Errorf("legacyCheckPassword(pw, hash) = %s, want %s", got, golden["legacy_check_ok"])
	}
	if got := strconv.FormatBool(legacyCheckPassword("wrong", legacyHash)); got != want("legacy_check_bad") {
		t.Errorf("legacyCheckPassword(\"wrong\", hash) = %s, want %s", got, golden["legacy_check_bad"])
	}
	if got := strconv.FormatBool(isLegacyPassword(legacyHash)); got != want("legacy_is_legacy") {
		t.Errorf("isLegacyPassword(hash) = %s, want %s", got, golden["legacy_is_legacy"])
	}
	if isLegacyPassword("deadbeef") {
		t.Error("isLegacyPassword must reject non-$H$ hashes")
	}
	// The legacy hash layout itself: "$H$" + 1 iota64 char + 8-char salt
	// (12-char seed) + 28-char encode64 digest = 40 chars.
	if len(legacyHash) != 12+28 || !strings.HasPrefix(legacyHash, "$H$") {
		t.Errorf("legacy hash layout wrong: %q (len %d)", legacyHash, len(legacyHash))
	}
}

// loadLoginGolden parses testdata/golden_login.txt (key=value; values may
// contain '=' padding, keys may contain CJK).
func loadLoginGolden() (map[string]string, error) {
	f, err := os.Open("testdata/golden_login.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimPrefix(line, "\uFEFF")
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out, sc.Err()
}
