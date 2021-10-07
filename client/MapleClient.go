package client

type MapleClient struct {
	SerialVersionUID int64
	LoginAttempt     int8
	AccountName      string
}

func NewMapleClient() MapleClient {
	return MapleClient{
		SerialVersionUID: 9179541993413738569,
		LoginAttempt:     0,
	}
}
