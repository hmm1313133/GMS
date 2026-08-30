package crypto

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestGoldenAgainstOriginalJar replays every vector produced by
// tools/goldengen/GoldenGen.java running against the ORIGINAL 079MAX2.jar
// (K:\079MAX2服务端\dist). This is the J1 byte-exactness proof.
func TestGoldenAgainstOriginalJar(t *testing.T) {
	golden, err := loadGolden()
	if err != nil {
		t.Skipf("golden vectors unavailable: %v", err)
	}
	if len(golden) < 40 {
		t.Fatalf("golden file suspiciously small: %d lines", len(golden))
	}

	fixed := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i*7 + 3)
		}
		return b
	}
	ivRecv := [4]byte{70, 114, 12, 199}
	ivSend := [4]byte{82, 48, 120, 55}
	sizes := []int{1, 8, 16, 100, 1500}

	check := func(name, got string) {
		want, ok := golden[name]
		if !ok {
			t.Errorf("golden line %q missing", name)
			return
		}
		if got != want {
			t.Errorf("%s mismatch:\n got  %s\n want %s", name, got, want)
		}
	}
	hx := func(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }

	// 1. Shanda
	for _, n := range sizes {
		buf := fixed(n)
		ShandaEncrypt(buf)
		check(fmt.Sprintf("shanda_enc_%d", n), hx(buf))

		buf2 := fixed(n)
		ShandaEncrypt(buf2)
		ShandaDecrypt(buf2)
		check(fmt.Sprintf("shanda_roundtrip_%d", n), hx(buf2))
	}

	// 2. AES-OFB recv-style (version 79)
	for _, n := range sizes {
		ofb := NewAESOFB(ivRecv, 79)
		buf := fixed(n)
		ofb.Crypt(buf)
		check(fmt.Sprintf("aesofb79_%d", n), hx(buf))
	}
	ofbA := NewAESOFB(ivRecv, 79)
	ofbA.Crypt(fixed(16))
	ivAfterA := ofbA.IV()
	check("aesofb79_iv_after", hx(ivAfterA[:]))

	// 3. AES-OFB send-style (version -80, the real server quirk)
	for _, n := range sizes {
		ofb := NewAESOFB(ivSend, -80)
		buf := fixed(n)
		ofb.Crypt(buf)
		check(fmt.Sprintf("aesofbSend_%d", n), hx(buf))
	}
	ofbB := NewAESOFB(ivSend, -80)
	b1, b2 := fixed(32), fixed(32)
	ofbB.Crypt(b1)
	ofbB.Crypt(b2)
	check("aesofbSend_chain_1", hx(b1))
	check("aesofbSend_chain_2", hx(b2))
	ivAfterB := ofbB.IV()
	check("aesofbSend_iv_after2", hx(ivAfterB[:]))

	// 4. Headers
	hdr := NewAESOFB(ivSend, -80)
	for _, n := range []int{1, 100, 1500, 0} {
		h := hdr.PacketHeader(n)
		check(fmt.Sprintf("hdr_send_%d", n), hx(h[:]))
	}
	hdr2 := NewAESOFB(ivRecv, 79)
	h2 := hdr2.PacketHeader(100)
	check("hdr_recv_100", hx(h2[:]))

	// 5. IV chain
	iv := ivSend
	for i := 1; i <= 5; i++ {
		iv = GetNewIv(iv)
		check(fmt.Sprintf("newiv_%d", i), hx(iv[:]))
	}

	// 6. End-to-end frames (header + shanda + crypt)
	for _, n := range sizes {
		enc := NewAESOFB(ivSend, -80)
		body := fixed(n)
		header := enc.PacketHeader(len(body))
		ShandaEncrypt(body)
		enc.Crypt(body)
		full := append(header[:], body...)
		check(fmt.Sprintf("e2e_%d", n), hx(full))
	}

	// 7. checkPacket / getPacketLength
	chk := NewAESOFB(ivRecv, 79)
	h := chk.PacketHeader(777)
	asInt := HeaderAsUint32(h)
	if got := PacketLengthFromHeader(asInt); got != 777 {
		t.Errorf("PacketLengthFromHeader = %d, want 777", got)
	}
	if golden["chk_777_len"] != "777" {
		t.Errorf("golden len line = %q", golden["chk_777_len"])
	}
	if golden["chk_777_ok"] != "true" || golden["chk_bad_ok"] != "false" {
		t.Errorf("golden check lines = %q / %q", golden["chk_777_ok"], golden["chk_bad_ok"])
	}
	if !chk.CheckPacket(h[0], h[1]) {
		t.Error("CheckPacket must accept freshly built header")
	}

	// 8. BitTools rolls incl. negative counts (Java uses count % 8; Java % can
	// keep the sign, e.g. -5 % 8 == -5, and shift by negative is masked to 0..31)
	checkInt := func(name string, got int) {
		if want, ok := golden[name]; !ok || strconv.Itoa(got) != want {
			t.Errorf("%s = %d, want %v", name, got, want)
		}
	}
	checkInt("rollLeft_0xAB_3", int(RollLeft(0xAB, 3)))
	checkInt("rollRight_0xAB_3", int(RollRight(0xAB, 3)))
	checkInt("rollRight_0xAB_neg5", int(RollRight(0xAB, negMod(-5, 8))))
	checkInt("rollLeft_0xAB_neg3", int(RollLeft(0xAB, negMod(-3, 8))))

	// 9. funnyBytes / key transcription
	check("funnybytes", hx(funnyBytes[:]))
	check("key", hx(mapleAESKey))
}

// negMod mirrors Java's % for negative operands (Go's % already matches Java
// for ints: -5 % 8 == -5). Kept explicit for documentation.
func negMod(a, b int) int { return a % b }

func loadGolden() (map[string]string, error) {
	f, err := os.Open("testdata/golden.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // e2e_1500 line is large
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
