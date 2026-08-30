package protocol

import "testing"

// TestRecvKnownOpcodes pins values taken verbatim from the original
// recv.properties / Java RecvPacketOpcode defaults (079MAX2).
func TestRecvKnownOpcodes(t *testing.T) {
	cases := []struct {
		op   RecvOp
		name string
		val  uint16
	}{
		{RecvLOGIN_PASSWORD, "LOGIN_PASSWORD", 0x01},
		{RecvSERVERLIST_REQUEST, "SERVERLIST_REQUEST", 0x02},
		{RecvCHARLIST_REQUEST, "CHARLIST_REQUEST", 0x09},
		{RecvCHAR_SELECT, "CHAR_SELECT", 0x0A},
		{RecvCHECK_CHAR_NAME, "CHECK_CHAR_NAME", 0x0C},
		{RecvCREATE_CHAR, "CREATE_CHAR", 0x11},
		{RecvDELETE_CHAR, "DELETE_CHAR", 0x12},
		{RecvPONG, "PONG", 0x13},
		{RecvPLAYER_LOGGEDIN, "PLAYER_LOGGEDIN", 0x0B},
		{RecvCHANGE_MAP, "CHANGE_MAP", 0x21},
		{RecvMOVE_PLAYER, "MOVE_PLAYER", 0x24},
		{RecvENTER_CASH_SHOP, "ENTER_CASH_SHOP", 0x23},
	}
	for _, c := range cases {
		if uint16(c.op) != c.val {
			t.Errorf("%s = 0x%04X, want 0x%04X", c.name, uint16(c.op), c.val)
		}
		if got := c.op.Name(); got != c.name {
			t.Errorf("Name() = %q, want %q", got, c.name)
		}
	}
}

func TestSendKnownOpcodes(t *testing.T) {
	cases := []struct {
		op   SendOp
		name string
		val  uint16
	}{
		{SendLOGIN_STATUS, "LOGIN_STATUS", 0x00},
		{SendLICENSE_RESULT, "LICENSE_RESULT", 0x02},
		{SendSERVERSTATUS, "SERVERSTATUS", 0x06},
		{SendSERVERLIST, "SERVERLIST", 0x09},
		{SendCHARLIST, "CHARLIST", 0x0A},
		{SendSERVER_IP, "SERVER_IP", 0x0B},
		{SendCHAR_NAME_RESPONSE, "CHAR_NAME_RESPONSE", 0x0C},
		{SendADD_NEW_CHAR_ENTRY, "ADD_NEW_CHAR_ENTRY", 0x11},
		{SendPING, "PING", 0x14},
		{SendCHANGE_CHANNEL, "CHANGE_CHANNEL", 0x13},
		{SendUPDATE_STATS, "UPDATE_STATS", 0x22},
		{SendGIVE_BUFF, "GIVE_BUFF", 0x23},
		{SendCANCEL_BUFF, "CANCEL_BUFF", 0x24},
	}
	for _, c := range cases {
		if uint16(c.op) != c.val {
			t.Errorf("%s = 0x%04X, want 0x%04X", c.name, uint16(c.op), c.val)
		}
		if got := c.op.Name(); got != c.name {
			t.Errorf("Name() = %q, want %q", got, c.name)
		}
	}
}

// TestOpcodeCounts guards the table against accidental regeneration drift.
// 153/225 unique keys in the properties files; 147/211 unique VALUES
// (disabled opcodes share the 0xFF placeholder etc.).
func TestOpcodeCounts(t *testing.T) {
	if got := len(recvNames); got != 147 {
		t.Errorf("recvNames count = %d, want 147", got)
	}
	if got := len(sendNames); got != 211 {
		t.Errorf("sendNames count = %d, want 211", got)
	}
	if got := len(recvByNames); got != 153 {
		t.Errorf("recvByNames count = %d, want 153", got)
	}
	if got := len(sendByNames); got != 225 {
		t.Errorf("sendByNames count = %d, want 225", got)
	}
}

func TestReverseLookup(t *testing.T) {
	if op, ok := RecvByName("PONG"); !ok || op != RecvPONG {
		t.Errorf("RecvByName(PONG) = %v %v", op, ok)
	}
	if _, ok := RecvByName("NO_SUCH"); ok {
		t.Error("unknown name must not resolve")
	}
	if op, ok := SendByName("PING"); !ok || op != SendPING {
		t.Errorf("SendByName(PING) = %v %v", op, ok)
	}
}

func TestStringerFallback(t *testing.T) {
	var unknown RecvOp = 0x7F11
	if got := unknown.String(); got != "0x7F11" {
		t.Errorf("String() = %q, want 0x7F11", got)
	}
}
