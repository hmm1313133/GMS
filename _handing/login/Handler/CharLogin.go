package Handler

import (
	"GMS/client"
	"GMS/tools/data/input"
)

type CharLogin struct {
}

func (cs *CharLogin) Welcome() {}

func (cs *CharLogin) LoginFailCount(c client.MapleClient) bool {
	c.LoginAttempt++
	if c.LoginAttempt > 5 {
		return true
	}
	return false
}

func (cs *CharLogin) Login(slea input.SeekableLittleEndianAccessor, c client.MapleClient) {
	login := slea.ReadMapleAsciiString()
	//pwd := slea.ReadMapleAsciiString()

	c.AccountName = login

	bytes := make([]byte, 6)

	for i := 0; i < len(bytes); i++ {
		bytes[i] = byte(slea.ReadByteAsInt())
	}

}
