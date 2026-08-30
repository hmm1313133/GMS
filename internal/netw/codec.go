package netw

import (
	"GMS/internal/crypto"
)

// cryptoHolder adapts internal/crypto.AESOFB to the session interfaces
// (kept indirect so netw tests can stub codecs).
type cryptoHolder struct {
	ofb *crypto.AESOFB
}

func (c *cryptoHolder) Crypt(data []byte) { c.ofb.Crypt(data) }

func (c *cryptoHolder) PacketHeader(length int) [4]byte { return c.ofb.PacketHeader(length) }

func (c *cryptoHolder) CheckPacket(b0, b1 byte) bool { return c.ofb.CheckPacket(b0, b1) }

// NewCryptoPair builds send/recv holders for the standard server wiring:
// send cipher with version -80, recv cipher with 79 (Java MapleServerHandler
// channelActive quirk).
func NewCryptoPair(ivSend, ivRecv [4]byte) (send, recv *cryptoHolder) {
	return &cryptoHolder{crypto.NewAESOFB(ivSend, -80)},
		&cryptoHolder{crypto.NewAESOFB(ivRecv, 79)}
}

// shandaEncrypt/shandaDecrypt delegate to the crypto package.
func shandaEncrypt(b []byte)   { crypto.ShandaEncrypt(b) }
func shandaDecrypt(b []byte)   { crypto.ShandaDecrypt(b) }
