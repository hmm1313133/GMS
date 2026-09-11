package client

import (
	"github.com/defaultmagi/maplelib"
	"github.com/panjf2000/gnet"
)

type MapleClient struct {
	SerialVersionUID int64
	LoginAttempt     int8
	AccountName      string
	Session          *gnet.Conn
	RC               maplelib.Crypt //Receive Crypt
	SC               maplelib.Crypt //Send Crypt
	Channels         int
	Send             [4]byte
	Receive          [4]byte
}

func NewMapleClient(session *gnet.Conn) MapleClient {
	return MapleClient{
		SerialVersionUID: 9179541993413738569,
		LoginAttempt:     0,
		Session:          session,
	}
}
