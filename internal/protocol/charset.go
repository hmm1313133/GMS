package protocol

import (
	"golang.org/x/text/encoding/simplifiedchinese"
)

// charsetGB18030 mirrors Java new String(bytes, "GB18030"): the ZEVMS 079
// protocol carries GBK/GB18030-encoded Chinese in packet strings.
func charsetGB18030() *gB18030Codec { return &gB18030Codec{} }

type gB18030Codec struct{}

// Decode converts GB18030 bytes to a UTF-8 string. Invalid sequences become
// the replacement char, matching Java's CharsetDecoder default action.
func (gB18030Codec) Decode(b []byte) string {
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil {
		// Java replaces unmappable input rather than failing; x/text already
		// substitutes U+FFFD for invalid bytes, so err only means truncation.
		_ = err
	}
	return string(out)
}

// EncodeGB18030 converts a UTF-8 string to GB18030 bytes (Java
// String.getBytes("GB18030") in the writer).
func EncodeGB18030(s string) []byte {
	out, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
	if err != nil {
		_ = err
	}
	return out
}
