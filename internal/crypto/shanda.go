package crypto

// ShandaEncrypt ports Java MapleCustomEncryption.encryptData: the 6-round
// "Shanda" custom cipher applied before AES-OFB on every packet body.
//
// Java semantics kept:
//   - dataLength is a signed byte counter that decrements each step;
//     its SIGNED value is used as the roll count (rollX handles %8) and as
//     the addend via (byte) truncation.
//   - `remember = cur = (byte)(cur ^ remember)` chains the XOR key.
func ShandaEncrypt(data []byte) []byte {
	for j := 0; j < 6; j++ {
		var remember byte
		dataLength := byte(len(data) & 0xFF)
		if j%2 == 0 {
			for i := 0; i < len(data); i++ {
				cur := data[i]
				cur = RollLeft(cur, 3)
				cur = cur + dataLength        // (byte)(cur + dataLength)
				cur = cur ^ remember          // remember = cur = ...
				remember = cur
				cur = RollRight(cur, int(dataLength)) // Java: dataLength & 0xFF (unsigned 0..255)
				cur = ^cur                    // (byte)(~cur & 0xFF)
				cur = cur + 72
				dataLength--
				data[i] = cur
			}
		} else {
			for i := len(data) - 1; i >= 0; i-- {
				cur := data[i]
				cur = RollLeft(cur, 4)
				cur = cur + dataLength
				cur = cur ^ remember
				remember = cur
				cur = cur ^ 0x13
				cur = RollRight(cur, 3)
				dataLength--
				data[i] = cur
			}
		}
	}
	return data
}

// ShandaDecrypt ports Java MapleCustomEncryption.decryptData (inverse rounds,
// j runs 1..6 with the same even/odd selection as encrypt).
func ShandaDecrypt(data []byte) []byte {
	for j := 1; j <= 6; j++ {
		var remember, nextRemember byte
		dataLength := byte(len(data) & 0xFF)
		if j%2 == 0 {
			for i := 0; i < len(data); i++ {
				cur := data[i]
				cur = cur - 72
				cur = ^cur
				nextRemember = RollLeft(cur, int(dataLength)) // Java: dataLength & 0xFF (unsigned 0..255)
				cur = nextRemember
				cur = cur ^ remember
				remember = nextRemember
				cur = cur - dataLength
				cur = RollRight(cur, 3)
				data[i] = cur
				dataLength--
			}
		} else {
			for i := len(data) - 1; i >= 0; i-- {
				cur := data[i]
				cur = RollLeft(cur, 3)
				nextRemember = cur ^ 0x13
				cur = nextRemember
				cur = cur ^ remember
				remember = nextRemember
				cur = cur - dataLength
				cur = RollRight(cur, 4)
				data[i] = cur
				dataLength--
			}
		}
	}
	return data
}
