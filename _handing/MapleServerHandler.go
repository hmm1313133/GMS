package handing

import (
	"GMS/client"
	"GMS/opcode/RecvPacketOpcode"
	"GMS/tools/data/input"
)

func handlePacket(header RecvPacketOpcode.RecvPacketOpcodeType, slea input.SeekableLittleEndianAccessor, c client.MapleClient, cs bool) {

	switch header {
	case RecvPacketOpcode.PONG:

		return
	case RecvPacketOpcode.LOGIN_PASSWORD:

	}
}
