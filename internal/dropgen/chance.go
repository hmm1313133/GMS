package dropgen

// Port of the drop-rate tables in
// tools/wztosql/MonsterDropCreator.java (GMS-P3.5).
//
// These numbers are the upstream generator's *author choice*, not data: the
// live 079MAX2 database was seeded by a third party with hand-tuned
// drop_data rows, so this package exists to rebuild a wz-derived baseline and
// to diff it against the database - never to overwrite it.
//
// Everything here is a pure function of (item id, monster id, is-boss), so it
// is unit-tested against the Java switch tables (chance_test.go), including
// the two fallthroughs the decompiler flattened:
//
//   - case 112 with no match falls into the 130/131/132/137 block (=> 700)
//   - case 400 with no match falls into the 401/402 block (=> 9000)

// getChance is MonsterDropCreator.getChance: the base chance of an item, in
// the drop_data `chance` unit where 1000000 means "always"
// (MapleMap compares Randomizer.nextInt(999999) against it).
//
// id is the item id, mobid the monster that would drop it, boss whether that
// monster is flagged as a boss (rare items get a guaranteed-ish 999999).
func getChance(id, mobid int, boss bool) int {
	// First table: by item category (id / 10000).
	switch id / 10000 {
	case 100:
		switch id {
		case 1002357, 1002390, 1002430, 1002905, 1002906, 1002926, 1002927, 1002972:
			return 300000
		}
		return 1500
	case 103:
		if id == 1032062 {
			return 100
		}
		return 1000
	case 105, 109:
		if id == 1092049 {
			return 100
		}
		return 700
	case 104, 106, 107:
		if id == 1072369 {
			return 300000
		}
		return 800
	case 108, 110:
		return 1000
	case 112:
		switch id {
		case 1122000:
			return 300000
		case 1122011, 1122012:
			return 800000
		}
		// Java fallthrough into the 130/131/132/137 block.
		fallthrough
	case 130, 131, 132, 137:
		if id == 1372049 {
			return 999999
		}
		return 700
	case 138, 140, 141, 142, 144:
		return 700
	case 133, 143, 145, 146, 147, 148, 149:
		return 500
	case 204:
		if id == 2049000 {
			return 150
		}
		return 300
	case 205:
		return 50000
	case 206:
		return 30000
	case 228:
		return 30000
	case 229:
		switch id {
		case 2290096:
			return 800000
		case 2290125:
			return 100000
		}
		return 500
	case 233:
		if id == 2330007 {
			return 50
		}
		return 500
	case 400:
		switch id {
		case 4000021:
			return 50000
		case 4001094:
			return 999999
		case 4001000:
			return 5000
		case 4000157:
			return 100000
		case 4001023, 4001024: // 4001024 was written 0x3D0D00 in the Java source
			return 999999
		case 4000244, 4000245:
			return 2000
		case 4001005:
			return 5000
		case 4001006:
			return 10000
		case 4000017, 4000082:
			return 40000
		case 4000446, 4000451, 4000456:
			return 10000
		case 4000459:
			return 20000
		case 4000030:
			return 60000
		case 4000339:
			return 70000
		case 4000313, 4007000, 4007001, 4007002, 4007003, 4007004,
			4007005, 4007006, 4007007, 4031456: // yes, 4031456 lives in case 400
			return 100000
		case 4001126:
			return 500000
		}
		switch id / 1000 {
		case 4000, 4001:
			return 600000
		case 4003:
			return 200000
		case 4004, 4006:
			return 10000
		case 4005:
			return 1000
		}
		// Java fallthrough into the 401/402 block.
		fallthrough
	case 401, 402:
		switch id {
		case 4020009:
			return 5000
		case 4021010:
			return 300000
		}
		return 9000
	case 403:
		switch id {
		case 4032024:
			return 50000
		case 4032181:
			if boss {
				return 999999
			}
			return 300000
		case 4032025, 4032155, 4032156, 4032159, 4032161, 4032163:
			return 600000
		case 4032166, 4032167, 4032168:
			return 10000
		case 4032151, 4032158, 4032164, 4032180:
			return 2000
		case 4032152, 4032153, 4032154:
			return 4000
		}
		return 300
	case 413:
		return 6000
	case 416:
		return 6000
	}

	// Second table: by item id leading digit (id / 1000000).
	switch id / 1000000 {
	case 1: // EQUIP
		return 999999
	case 2: // USE
		switch id {
		case 2000004, 2000005:
			if boss {
				return 999999
			}
			return 20000
		case 2000006:
			if mobid == 9420540 {
				return 50000
			}
			if boss {
				return 999999
			}
			return 20000
		case 2022345:
			if boss {
				return 999999
			}
			return 3000
		case 2012002:
			return 6000
		case 2020013, 2020015:
			if boss {
				return 999999
			}
			return 20000
		case 2060000, 2060001, 2061000, 2061001:
			return 25000
		case 2070000, 2070001, 2070002, 2070003, 2070004,
			2070008, 2070009, 2070010:
			return 500
		case 2070005:
			return 400
		case 2070006, 2070007:
			return 200
		case 2070012, 2070013:
			return 1500
		case 2070019:
			return 100
		case 2210006:
			return 999999
		}
		return 20000
	case 3: // SETUP
		switch id {
		case 3010007, 3010008:
			return 500
		}
		return 2000
	}
	// Java printed "未處理的數據, ID : " + id here and returned 999999.
	return 999999
}

// multipleDropsIncrement is MonsterDropCreator.multipleDropsIncrement: rare
// items are inserted several times (one row per copy) so several can drop.
func multipleDropsIncrement(itemid, mobid int) int {
	switch itemid {
	case 1002357, 1002390, 1002430, 1002926, 1002927:
		return 5
	case 1122000:
		return 4
	case 4021010:
		return 7
	case 1002972:
		return 2
	case 4000172:
		if mobid == 7220001 {
			return 8
		}
		return 1
	case 4000000, 4000003, 4000005, 4000016, 4000018, 4000019,
		4000021, 4000026, 4000029, 4000031, 4000032, 4000033,
		4000043, 4000044, 4000073, 4000074, 4000113, 4000114,
		4000115, 4000117, 4000118, 4000119, 4000166, 4000167,
		4000195, 4000268, 4000269, 4000270, 4000283, 4000284,
		4000285, 4000289, 4000298, 4000329, 4000330, 4000331,
		4000356, 4000364, 4000365:
		// 3220001 and 4220000 were written 0x312221 / 0x406460 in the Java
		// source; 4000119 is a monster id here (the original typo, kept).
		switch mobid {
		case 2220000, 3220000, 3220001, 4220000, 5220000, 5220002,
			5220003, 6220000, 4000119, 7220000, 7220002, 8220000,
			8220002, 8220003:
			return 3
		}
		return 1
	}
	return 1
}

// incrementRate is MonsterDropCreator.IncrementRate: overrides the chance of
// copy number `times` for the rare multi-drop items. -1 means "use the base
// rate".
func incrementRate(itemid, times int) int {
	rare := itemid == 1002357 || itemid == 1002926 || itemid == 1002927
	switch times {
	case 0:
		if rare {
			return 999999
		}
		if itemid == 1122000 {
			return 999999
		}
		if itemid == 1002972 {
			return 999999
		}
	case 1:
		if rare {
			return 999999
		}
		if itemid == 1122000 {
			return 999999
		}
		if itemid == 1002972 {
			return 300000
		}
	case 2:
		if rare {
			return 300000
		}
		if itemid == 1122000 {
			return 300000
		}
	case 3, 4:
		if rare {
			return 300000
		}
	}
	return -1
}
