package Handler

import "GMS/client"

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

func (cs *CharLogin) Login() {}
