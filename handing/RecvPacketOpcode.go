package handing

type RecvPacketOpcode struct {
	RecvPacketOpcodeType
}

type RecvPacketOpcodeType int

const (
	PONG                 RecvPacketOpcodeType = 0x13
	LOGIN_PASSWORD       RecvPacketOpcodeType = 0x13
	SERVERLIST_REQUEST   RecvPacketOpcodeType = 0x13
	LICENSE_REQUEST      RecvPacketOpcodeType = 0x13
	SERVERLIST_REREQUEST RecvPacketOpcodeType = 0x13
	CHARLIST_REQUEST     RecvPacketOpcodeType = 0x13
	SET_GENDER           RecvPacketOpcodeType = 0x13
	SERVERSTATUS_REQUEST RecvPacketOpcodeType = 0x13
	AFTER_LOGIN          RecvPacketOpcodeType = 0x13
	REGISTER_PIN         RecvPacketOpcodeType = 0x13
	PLAYER_DC            RecvPacketOpcodeType = 0x13
	VIEW_ALL_CHAR        RecvPacketOpcodeType = 0x13
	PICK_ALL_CHAR        RecvPacketOpcodeType = 0x13
	CHAR_SELECT          RecvPacketOpcodeType = 0x13
	CHECK_CHAR_NAME      RecvPacketOpcodeType = 0x13
	CREATE_CHAR          RecvPacketOpcodeType = 0x13
	DELETE_CHAR          RecvPacketOpcodeType = 0x13
	ERROR_LOG            RecvPacketOpcodeType = 0x13
	RELOG                RecvPacketOpcodeType = 0x13
	PLAYER_LOGGEDIN      RecvPacketOpcodeType = 0x13
	STRANGE_DATA         RecvPacketOpcodeType = 0x13
	CHANGE_MAP           RecvPacketOpcodeType = 0x13
	CHANGE_CHANNEL       RecvPacketOpcodeType = 0x13
	ENTER_CASH_SHOP      RecvPacketOpcodeType = 0x13
	MOVE_PLAYER          RecvPacketOpcodeType = 0x13
	CANCEL_CHAIR         RecvPacketOpcodeType = 0x13
	USE_CHAIR            RecvPacketOpcodeType = 0x13
	CLOSE_RANGE_ATTACK   RecvPacketOpcodeType = 0x13
	RANGED_ATTACK        RecvPacketOpcodeType = 0x13
	MAGIC_ATTACK         RecvPacketOpcodeType = 0x13
	PASSIVE_ENERGY       RecvPacketOpcodeType = 0x13
	TAKE_DAMAGE          RecvPacketOpcodeType = 0x13
	GENERAL_CHAT         RecvPacketOpcodeType = 0x13
	CLOSE_CHALKBOARD     RecvPacketOpcodeType = 0x13
	FACE_EXPRESSION      RecvPacketOpcodeType = 0x13
	USE_ITEMEFFECT       RecvPacketOpcodeType = 0x13
	WHEEL_OF_FORTUNE     RecvPacketOpcodeType = 0x13
	MONSTER_BOOK_COVER   RecvPacketOpcodeType = 0x13
	NPC_TALK             RecvPacketOpcodeType = 0x13
	NPC_TALK_MORE        RecvPacketOpcodeType = 0x13
	NPC_SHOP             RecvPacketOpcodeType = 0x13
	STORAGE              RecvPacketOpcodeType = 0x13
	USE_HIRED_MERCHANT   RecvPacketOpcodeType = 0x13
	MERCH_ITEM_STORE     RecvPacketOpcodeType = 0x13
	DUEY_ACTION          RecvPacketOpcodeType = 0x13
	ITEM_SORT            RecvPacketOpcodeType = 0x13
	ITEM_GATHER          RecvPacketOpcodeType = 0x13
	ITEM_MOVE            RecvPacketOpcodeType = 0x13
	USE_ITEM             RecvPacketOpcodeType = 0x13
	CANCEL_ITEM_EFFECT   RecvPacketOpcodeType = 0x13
	USE_FISHING_ITEM     RecvPacketOpcodeType = 0x13
	USE_SUMMON_BAG       RecvPacketOpcodeType = 0x13
)
