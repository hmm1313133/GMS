package dropgen

import (
	"os"
	"strings"
	"testing"

	"GMS/internal/wzs"
)

// ---- chance tables (Java MonsterDropCreator.getChance) ----

func TestGetChance(t *testing.T) {
	cases := []struct {
		id, mobid int
		boss      bool
		want      int
	}{
		{1002357, 1, false, 300000}, // case 100 special
		{1002067, 1, false, 1500},   // case 100 default
		{1032062, 1, false, 100},    // case 103 special
		{1032000, 1, false, 1000},
		{1092049, 1, false, 100}, // case 105/109 special
		{1050092, 1, false, 700},
		{1072369, 1, false, 300000}, // case 104/106/107 special
		{1040104, 1, false, 800},
		{1082085, 1, false, 1000},
		{1122000, 1, false, 300000},
		{1122011, 1, false, 800000},
		// case 112 fallthrough into 130/131/132/137 -> 700
		{1122267, 1, false, 700},
		{1372049, 1, false, 999999},
		{1302000, 1, false, 700},
		{1402004, 1, false, 700},
		{1472027, 1, false, 500},
		{2049000, 1, false, 150},
		{2040306, 1, false, 300},
		{2050000, 1, false, 50000},
		{2060011, 1, false, 30000},
		{2290096, 1, false, 800000},
		{2290125, 1, false, 100000},
		{2330007, 1, false, 50},
		{4000021, 1, false, 50000},
		{4001094, 1, false, 999999},
		{4001024, 1, false, 999999}, // 0x3D0D00 in Java
		{4001126, 1, false, 500000},
		{4000000, 1, false, 600000},   // id/1000 == 4000
		{4003000, 1, false, 200000},   // id/1000 == 4003
		{4005000, 1, false, 1000},     // id/1000 == 4005
		{4009999, 1, false, 9000},     // case 400 fallthrough into 401/402
		{4010000, 1, false, 9000},     // id/1000 == 4001 would be 600000, but 4001000 is exact? no: 4010000/1000=4010
		{4020009, 1, false, 5000},     // case 401/402 special
		{4021010, 1, false, 300000},   // case 401/402 special
		{4010006, 1, false, 9000},     // case 401/402 default
		{4032181, 1, false, 300000},   // boss-dependent
		{4032181, 1, true, 999999},    // boss-dependent
		{4032024, 1, false, 50000},    // case 403 special
		// 4031456 is listed inside Java's case 400, but 4031456/10000 == 403,
		// so it is unreachable dead code there and lands in case 403 -> 300.
		{4031456, 1, false, 300},
		{4039999, 1, false, 300}, // case 403 default
		{2000004, 1, false, 20000},    // case 2 non-boss
		{2000004, 1, true, 999999},    // case 2 boss
		{2000006, 9420540, false, 50000}, // mobid-specific
		{2000006, 1, false, 20000},
		{2070019, 1, false, 100},
		{2210006, 1, false, 999999},
		{2999999, 1, false, 20000},  // case 2 default
		{3010007, 1, false, 500},    // case 3 special
		{3999999, 1, false, 2000},   // case 3 default
		{5000000, 1, false, 999999}, // unhandled -> 999999
		{1902347, 1, false, 999999}, // id/10000 == 190 -> falls to id/1000000 == 1
	}
	for _, c := range cases {
		if got := getChance(c.id, c.mobid, c.boss); got != c.want {
			t.Errorf("getChance(%d, %d, boss=%v) = %d, want %d",
				c.id, c.mobid, c.boss, got, c.want)
		}
	}
}

func TestMultipleDropsIncrement(t *testing.T) {
	cases := []struct {
		item, mob, want int
	}{
		{1002357, 1, 5},
		{1002390, 1, 5},
		{1122000, 1, 4},
		{4021010, 1, 7},
		{1002972, 1, 2},
		{4000172, 7220001, 8},
		{4000172, 1, 1},
		{4000019, 2220000, 3},
		{4000019, 3220001, 3}, // 0x312221 in Java
		{4000019, 4220000, 3}, // 0x406460 in Java
		{4000019, 4000119, 3}, // the original monster-id typo
		{4000019, 1, 1},
		{4000019, 100100, 1},
	}
	for _, c := range cases {
		if got := multipleDropsIncrement(c.item, c.mob); got != c.want {
			t.Errorf("multipleDropsIncrement(%d, %d) = %d, want %d", c.item, c.mob, got, c.want)
		}
	}
}

func TestIncrementRate(t *testing.T) {
	cases := []struct {
		item, times, want int
	}{
		{1002357, 0, 999999},
		{1002357, 1, 999999},
		{1002357, 2, 300000},
		{1002357, 3, 300000},
		{1002357, 4, 300000},
		{1002357, 5, -1},
		{1122000, 0, 999999},
		{1122000, 1, 999999},
		{1122000, 2, 300000},
		{1122000, 3, -1},
		{1002972, 0, 999999},
		{1002972, 1, 300000},
		{1002972, 2, -1},
		{4000019, 0, -1},
	}
	for _, c := range cases {
		if got := incrementRate(c.item, c.times); got != c.want {
			t.Errorf("incrementRate(%d, %d) = %d, want %d", c.item, c.times, got, c.want)
		}
	}
}

func TestExpandDrops(t *testing.T) {
	// 1122000: 4 copies, the first two forced to 999999, the third to 300000,
	// the fourth keeps the base rate.
	got := expandDrops(9400013, 1122000, 700)
	want := []int{999999, 999999, 300000, 700}
	if len(got) != len(want) {
		t.Fatalf("expandDrops returned %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Chance != want[i] {
			t.Errorf("copy %d chance = %d, want %d", i, got[i].Chance, want[i])
		}
		if got[i].Minimum != 1 || got[i].Maximum != 1 || got[i].QuestID != 0 {
			t.Errorf("copy %d = %+v, want 1/1/0 quantities", i, got[i])
		}
	}
}

// ---- monster cards ----

func TestMonsterCardSuffix(t *testing.T) {
	items := []itemRef{
		{ID: 2380000, Name: "蜗牛卡片"},
		{ID: 2380001, Name: "Snail Card"},
	}
	mobs := []mobRef{{ID: 100100, Name: "蜗牛"}, {ID: 100120, Name: "Snail"}}

	// Java's hard-coded English suffix only finds the English card.
	entries, _ := monsterCardDrops(items, mobs, DefaultCardSuffix)
	if len(entries) != 1 || entries[0].ItemID != 2380001 || entries[0].MonsterID != 100120 {
		t.Fatalf("english suffix: got %+v, want the Snail Card row", entries)
	}
	if entries[0].Chance != 1000 {
		t.Errorf("non-boss card chance = %d, want 1000", entries[0].Chance)
	}

	// The Chinese suffix is what this distribution actually needs.
	entries, cards := monsterCardDrops(items, mobs, "卡片")
	if len(entries) != 1 || entries[0].ItemID != 2380000 || entries[0].MonsterID != 100100 {
		t.Fatalf("chinese suffix: got %+v, want the 蜗牛卡片 row", entries)
	}
	if len(cards) != 1 || cards[0].Seq != 1 || cards[0].CardID != 2380000 {
		t.Fatalf("cards = %+v, want one row for 2380000", cards)
	}
}

func TestMonsterCardBossRate(t *testing.T) {
	items := []itemRef{{ID: 2380100, Name: "Boss Card"}}
	mobs := []mobRef{{ID: 8810018, Name: "Boss", Boss: 1}}
	entries, _ := monsterCardDrops(items, mobs, " Card")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Chance != 25000 { // 1000 * 25
		t.Errorf("boss card chance = %d, want 25000", entries[0].Chance)
	}
}

func TestAdjustMonsterBookRate(t *testing.T) {
	mobs := []mobRef{{ID: 100, Name: "x", Boss: 1, Rare: 0}}
	// boss, base rate under the 100000 threshold -> *10
	if got := adjustMonsterBookRate(1000, 100, mobs); got != 10000 {
		t.Errorf("boss plain = %d, want 10000", got)
	}
	// rareItemDropLevel 2 -> *10 and stop (no second *10)
	mobs[0].Rare = 2
	if got := adjustMonsterBookRate(1000, 100, mobs); got != 10000 {
		t.Errorf("rare=2 = %d, want 10000", got)
	}
	// rareItemDropLevel 3 -> *30, then the trailing *10
	mobs[0].Rare = 3
	if got := adjustMonsterBookRate(1000, 100, mobs); got != 300000 {
		t.Errorf("rare=3 = %d, want 300000", got)
	}
	// 8810018 keeps the Java fallthrough (48 then 45) and then still eats the
	// trailing *10, because the `break` only leaves the rarity switch:
	// 1000 * 48 * 45 * 10.
	if got := adjustMonsterBookRate(1000, 8810018, []mobRef{{ID: 8810018, Boss: 1, Rare: 3}}); got != 21600000 {
		t.Errorf("8810018 = %d, want 21600000", got)
	}
	// 8800002 breaks inside the rarity switch, so it gets 45 then the *10.
	if got := adjustMonsterBookRate(1000, 8800002, []mobRef{{ID: 8800002, Boss: 1, Rare: 3}}); got != 450000 {
		t.Errorf("8800002 = %d, want 450000", got)
	}
	// 9400287 short-circuits before the trailing *10.
	if got := adjustMonsterBookRate(1000, 9400287, []mobRef{{ID: 9400287, Boss: 1}}); got != 60000 {
		t.Errorf("9400287 = %d, want 60000", got)
	}
	// rate above the threshold is left alone
	mobs[0].Rare = 0
	if got := adjustMonsterBookRate(100001, 100, mobs); got != 100001 {
		t.Errorf("rate>100000 = %d, want unchanged", got)
	}
	// non-boss monsters get no multipliers
	if got := adjustMonsterBookRate(1000, 100, []mobRef{{ID: 100, Boss: 0}}); got != 1000 {
		t.Errorf("non-boss = %d, want 1000", got)
	}
}

func TestLeftPad(t *testing.T) {
	if got := leftPad("100100.img", '0', 11); got != "0100100.img" {
		t.Errorf("leftPad = %q, want %q", got, "0100100.img")
	}
	if got := leftPad("12345678901.img", '0', 11); got != "12345678901.img" {
		t.Errorf("leftPad of a long string = %q, want unchanged", got)
	}
}

// ---- real wz export (skipped when I:\GMS\wz is absent) ----

func TestGenerateRealWZ(t *testing.T) {
	root, err := wzs.OpenRoot("../../wz")
	if err != nil {
		t.Skipf("no wz export at ../../wz: %v", err)
	}

	res, err := GenerateWithOptions(root, Options{CardSuffix: "卡片"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Mobs == 0 || res.Stats.Items == 0 {
		t.Fatalf("empty result: %s", res.Summary())
	}
	if len(res.Entries) == 0 {
		t.Fatal("no drop entries generated")
	}

	// 蜗牛 (100100) has a monster-book entry with reward 4000019.
	var found bool
	for _, e := range res.Entries {
		if e.MonsterID == 100100 && e.ItemID == 4000019 {
			found = true
			break
		}
	}
	if !found {
		t.Error("100100/4000019 missing from the generated drops")
	}

	// The Chinese card suffix must actually resolve cards on this
	// distribution (the English one matches a single item).
	if len(res.Cards) < 300 {
		t.Errorf("only %d monster cards matched with suffix 卡片, want several hundred", len(res.Cards))
	}
	for _, c := range res.Cards {
		if c.MonsterID == 0 {
			t.Errorf("card %d matched no monster", c.CardID)
		}
	}
	t.Log(res.Summary())
}

func TestWriteSQL(t *testing.T) {
	res := &Result{
		Entries: []Entry{
			{MonsterID: 100100, ItemID: 4000019, Chance: 360000, Minimum: 1, Maximum: 1, Note: "蜗牛的触角"},
			{MonsterID: 100100, ItemID: 2000000, Chance: 20000, Minimum: 1, Maximum: 1},
			{MonsterID: 100120, ItemID: 4000019, Chance: 360000, Minimum: 1, Maximum: 1},
		},
		Cards:  []Card{{Seq: 1, CardID: 2380000, MonsterID: 100100}},
		Errors: []string{"dummy warning"},
	}
	var b strings.Builder
	if err := WriteSQL(&b, res, "drop_data"); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"INSERT INTO `drop_data`",
		"(100100, 4000019, 1, 1, 0, 360000)",
		"(100120, 4000019, 1, 1, 0, 360000)",
		"-- 蜗牛的触角",
		"-- monstercarddata",
		"-- dummy warning",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SQL output missing %q", want)
		}
	}
	// one INSERT per monster
	if n := strings.Count(out, "INSERT INTO `drop_data`"); n != 2 {
		t.Errorf("got %d INSERT statements, want 2 (one per monster)", n)
	}
	_ = os.Stdout
}
