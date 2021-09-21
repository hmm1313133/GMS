package constants

type ServerConstants struct {
	UseFixedIv   bool
	MapleVersion int
	MaplePatch   int
}

func (sc *ServerConstants) Init() {
	sc.MapleVersion = 79
	sc.MaplePatch = 1
}
