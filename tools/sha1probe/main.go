// sha1probe: figure out what byte string the golden sha1_中文 vector hashed.
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func hx(b []byte) string { s := sha1.Sum(b); return hex.EncodeToString(s[:]) }

// javaLen is Java String.length() (UTF-16 code units).
func javaLen(s string) int { return len(utf16.Encode([]rune(s))) }

func main() {
	utf8 := []byte("中文")

	// Scenario A: javac read the UTF-8 source as GBK -> literal mojibake.
	moji, err := simplifiedchinese.GBK.NewDecoder().Bytes(utf8)
	if err != nil {
		fmt.Println("gbk decode err:", err)
	}
	fmt.Printf("mojibake string: %q (javaLen=%d)\n", moji, javaLen(string(moji)))
	mojiUTF8 := []byte(string(moji))
	n := javaLen(string(moji))
	if n > len(mojiUTF8) {
		n = len(mojiUTF8)
	}
	fmt.Printf("moji utf8 full   %s\n", hx(mojiUTF8))
	fmt.Printf("moji utf8 first%d %s\n", n, hx(mojiUTF8[:n]))

	// Scenario B: source was GBK, compiled as GBK (clean "中文").
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文"))
	fmt.Printf("gbk(中文) full   %s\n", hx(gbk))
	// Java getBytes("UTF-8") of "中文" truncated to 2 units:
	fmt.Printf("utf8 first2      %s\n", hx(utf8[:2]))
}
