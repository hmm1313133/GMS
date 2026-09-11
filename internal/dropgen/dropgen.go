// Package dropgen rebuilds a wz-derived monster drop table: the Go port of
// tools/wztosql/MonsterDropCreator.java (GMS-P3.5).
//
// Scope warning (read before using this for anything):
//
//	The live 079MAX2 database ships hand-tuned drop_data rows (14,243 rows
//	over 900 monsters) that do NOT come from this generator - someone else
//	built them. The database is the source of truth; this package produces a
//	baseline to diff against, or a starting point for a fresh server. It is
//	wired into tools/wztosql, which never writes to a database by itself.
//
// Three sources of rows, in Java's order:
//
//  1. hand-written per-monster lists (getDropsNotInMonsterBook)
//  2. String.wz/MonsterBook.img/<mobid>/reward - the monster-book rewards
//  3. monster cards (item ids 2380000..2388070) matched to monsters by name
package dropgen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"GMS/internal/wzs"
)

// NoItemName is the fallback Java used for unnamed entries.
const NoItemName = "NO-NAME"

// CardID range of the monster-book cards (Java lastmonstercardid = 2388070).
const (
	CardIDMin = 2380000
	CardIDMax = 2388070
)

// Entry is one row to insert into drop_data.
type Entry struct {
	MonsterID int
	ItemID    int
	Chance    int
	Minimum   int
	Maximum   int
	QuestID   int
	Note      string // item/monster name, only used for the SQL comments
}

// Card is one row of the monstercarddata table (book card -> monster).
type Card struct {
	Seq       int
	CardID    int
	MonsterID int
}

// Result is everything the generator produced.
type Result struct {
	Entries []Entry
	Cards   []Card
	Stats   Stats
	Errors  []string // non-fatal: unreadable Mob.wz image, unparsable node name
}

// Options tunes the generation.
type Options struct {
	// CardSuffix is stripped from a monster-card item name before it is
	// matched against a monster name. Java hard-coded " Card" (English wz);
	// this distribution is Chinese, where the cards are named "蜗牛卡片", so
	// the default only ever matches 1 card while the live database carries
	// 422 card drops. tools/wztosql exposes -card-suffix 卡片 for that.
	CardSuffix string
}

// DefaultCardSuffix is the Java hard-coded suffix.
const DefaultCardSuffix = " Card"

// Stats counts what the wz export yielded.
type Stats struct {
	Items        int
	Mobs         int
	Bosses       int
	MonsterBooks int
}

// itemRef is Java's itemNameCache entry (id + display name).
type itemRef struct {
	ID   int
	Name string
}

// mobRef is Java's mobCache entry.
type mobRef struct {
	ID    int
	Name  string
	Boss  int
	Rare  int // info/rareItemDropLevel
}

// Generate walks the wz export and returns the drop rows it implies, using
// Java's own options.
func Generate(root *wzs.Root) (*Result, error) {
	return GenerateWithOptions(root, Options{CardSuffix: DefaultCardSuffix})
}

// GenerateWithOptions is Generate with tunable options (see Options).
func GenerateWithOptions(root *wzs.Root, opt Options) (*Result, error) {
	if opt.CardSuffix == "" {
		opt.CardSuffix = DefaultCardSuffix
	}
	strWZ, err := root.WZ("String.wz")
	if err != nil {
		return nil, err
	}

	res := &Result{}
	items, err := loadItemCache(strWZ)
	if err != nil {
		return nil, err
	}
	res.Stats.Items = len(items)

	mobs, err := loadMobCache(root)
	if err != nil {
		return nil, err
	}
	res.Stats.Mobs = len(mobs)
	for _, m := range mobs {
		if m.Boss > 0 {
			res.Stats.Bosses++
		}
	}

	res.Entries = append(res.Entries, hardcodedDrops(mobs)...)

	book := monsterBookDrops(strWZ, mobs, res)
	res.Entries = append(res.Entries, book...)
	res.Stats.MonsterBooks = len(book)

	cardEntries, cards := monsterCardDrops(items, mobs, opt.CardSuffix)
	res.Entries = append(res.Entries, cardEntries...)
	res.Cards = cards

	return res, nil
}

// itemSources mirrors Java's getAllItems: the String.wz images holding item
// names, in this exact order (the order matters, see monsterCardDrops: the
// first monster whose name matches a card wins).
var itemSources = []struct{ image, sub string }{
	{"Cash.img", ""},
	{"Consume.img", ""},
	{"Eqp.img", "Eqp"},
	{"Etc.img", "Etc"},
	{"Ins.img", ""},
	{"Pet.img", ""},
}

func loadItemCache(str *wzs.Provider) ([]itemRef, error) {
	var out []itemRef
	for _, src := range itemSources {
		root, err := str.Data(src.image)
		if err != nil {
			return nil, err
		}
		cur := root
		if src.sub != "" {
			cur = cur.ChildByPath(src.sub)
			if cur == nil {
				continue
			}
			// Eqp/Etc are one level deeper: <category>/<itemid>.
			for _, cat := range cur.Children() {
				out = append(out, collectItems(cat)...)
			}
			continue
		}
		out = append(out, collectItems(cur)...)
	}
	return out, nil
}

func collectItems(parent *wzs.Node) []itemRef {
	var out []itemRef
	for _, node := range parent.Children() {
		id, err := strconv.Atoi(node.Name)
		if err != nil {
			continue // Java would have thrown; a non-numeric key here is junk
		}
		out = append(out, itemRef{
			ID:   id,
			Name: wzs.GetStringPathDef("name", node, NoItemName),
		})
	}
	return out
}

// loadMobCache is Java's getAllMobs: every String.wz/Mob.img entry, enriched
// with info/boss and info/rareItemDropLevel from Mob.wz.
//
// Mob.wz is opened uncached on purpose: 1,614 monster images would otherwise
// stay resident, and only two ints are needed from each.
func loadMobCache(root *wzs.Root) ([]mobRef, error) {
	str, err := root.WZ("String.wz")
	if err != nil {
		return nil, err
	}
	mobImg, err := str.Data("Mob.img")
	if err != nil {
		return nil, err
	}
	mobWZ, err := wzs.OpenUncached(filepath.Join(root.Dir(), "Mob.wz"))
	if err != nil {
		return nil, err
	}

	var out []mobRef
	for _, node := range mobImg.Children() {
		id, err := strconv.Atoi(node.Name)
		if err != nil {
			continue
		}
		img, err := mobWZ.Data(leftPad(strconv.Itoa(id)+".img", '0', 11))
		if err != nil {
			continue // Java wrapped this in try/catch and skipped the monster
		}
		boss := wzs.GetIntConvertPathDef("info/boss", img, 0)
		if id == 8810018 {
			boss = 1 // Pink Bean is not flagged in Mob.wz but is a boss
		}
		out = append(out, mobRef{
			ID:   id,
			Name: wzs.GetStringPathDef("name", node, NoItemName),
			Boss: boss,
			Rare: wzs.GetIntConvertPathDef("info/rareItemDropLevel", img, 0),
		})
	}
	return out, nil
}

// hardcodedDrops is getDropsNotInMonsterBook: eight event monsters whose
// drops the upstream author typed in by hand. Duplicated item ids are
// intentional - Java inserted one row per occurrence.
func hardcodedDrops(mobs []mobRef) []Entry {
	var out []Entry
	for _, h := range hardcodedMobDrops {
		boss := isBoss(mobs, h.mob)
		for _, itemID := range h.items {
			rate := getChance(itemID, h.mob, boss)
			if rate <= 100000 {
				switch h.mob {
				case 9400121:
					rate *= 5
				case 9400112, 9400113, 9400300:
					rate *= 10
				}
			}
			out = append(out, expandDrops(h.mob, itemID, rate)...)
		}
	}
	return out
}

var hardcodedMobDrops = []struct {
	mob   int
	items []int
}{
	{9400112, []int{4000139, 2002011, 2002011, 2002011, 2000004, 2000004}},
	{9400113, []int{4000140, 2022027, 2022027, 2000004, 2000004, 2002008, 2002008}},
	{9400300, []int{4000141, 2000004, 2040813, 2041030, 2041040, 1072238, 1032026, 1372011}},
	{9400013, []int{
		4000225, 2000006, 2000004, 2070013, 2002005, 2022018, 2040306, // 2040306 was 0x1F21F2
		2043704, 2044605, 2041034, 1032019, 1102013, 1322026, 1092015,
		1382016, 1002276, 1002403, 1472027,
	}},
	{8800002, []int{1372049}},
	{8810018, []int{4001094, 2290125}},
	{9400121, []int{
		4000138, 4010006, 2000006, 2000011, 2020016, 2022024, 2022026,
		2043705, 2040716, 2040908, 2040510, 1072239, 1422013, 1402016,
		1442020, 1432011, 1332022, 1312015, 1382010, 1372009, 1082085,
		1332022, 1472033,
	}},
	{9400545, []int{
		4032024, 4032025, 4020006, 4020008, 4010001, 4004001, 2070006,
		2044404, 2044702, 2044305, 1102029, 1032023, 1402004, 1072210,
		1040104, 1060092, 1082129, 1442008, 1072178, 1050092, 1002271,
		1051053, 1382008, 1002275, 1051082, 1050064, 1472028, 1072193,
		1072172, 1002285,
	}},
}

// monsterBookDrops walks String.wz/MonsterBook.img/<mobid>/reward.
func monsterBookDrops(str *wzs.Provider, mobs []mobRef, res *Result) []Entry {
	book, err := str.Data("MonsterBook.img")
	if err != nil {
		res.Errors = append(res.Errors, "MonsterBook.img: "+err.Error())
		return nil
	}

	var out []Entry
	for _, mobNode := range book.Children() {
		id, err := strconv.Atoi(mobNode.Name)
		if err != nil {
			continue
		}
		// 9400408 has no Mob.wz entry of its own; its rewards are logged
		// against 9400409 (Java: idtoLog).
		idToLog := id
		if id == 9400408 {
			idToLog = 9400409
		}
		reward := mobNode.ChildByPath("reward")
		if reward == nil || len(reward.Children()) == 0 {
			continue // Java NPE'd here; skipping is the sane port
		}
		for _, drop := range reward.Children() {
			itemID := wzs.GetIntConvert(drop)
			rate := getChance(itemID, idToLog, isBoss(mobs, idToLog))
			rate = adjustMonsterBookRate(rate, idToLog, mobs)
			out = append(out, expandDrops(idToLog, itemID, rate)...)
		}
	}
	return out
}

// adjustMonsterBookRate applies Java's boss/rarity multipliers. The Java
// original looped over the whole monster cache looking for the id and used
// `break`/`continue block20` to skip the trailing "rate *= 10"; the semantics
// are preserved here.
func adjustMonsterBookRate(rate, monsterID int, mobs []mobRef) int {
	for _, m := range mobs {
		if m.ID != monsterID {
			continue
		}
		if m.Boss <= 0 || rate > 100000 {
			break
		}
		if m.Rare == 2 {
			rate *= 10
			break
		}
		if m.Rare == 3 {
			switch monsterID {
			case 8810018:
				rate *= 48
				fallthrough // Java had no break after case 8810018
			case 8800002:
				rate *= 45
			default:
				rate *= 30
			}
		}
		switch monsterID {
		case 8860010, 9400265, 9400270, 9400273:
			rate *= 10
			continue
		case 9400294:
			rate *= 24
			continue
		case 9420522:
			rate *= 29
			continue
		case 9400409:
			rate *= 35
			continue
		case 9400287:
			rate *= 60
			continue
		}
		rate *= 10
	}
	return rate
}

// monsterCardDrops is the "怪物卡數據" pass: card items are matched to a
// monster by (case-insensitive) name, the " Card" suffix stripped.
func monsterCardDrops(items []itemRef, mobs []mobRef, suffix string) ([]Entry, []Card) {
	var entries []Entry
	var cards []Card

	bookName := func(name string) string {
		// Java: if (name.contains(" Card")) delete the last 5 characters.
		return strings.TrimSuffix(name, suffix)
	}

	findMob := func(name string) (mobRef, bool) {
		for _, m := range mobs {
			if strings.EqualFold(m.Name, name) {
				return m, true
			}
		}
		return mobRef{}, false
	}

	seq := 1
	lastCardID := 0
	for _, it := range items {
		if it.ID < CardIDMin || it.ID > CardIDMax {
			continue
		}
		name := bookName(it.Name)
		if m, ok := findMob(name); ok {
			rate := 1000
			if m.Boss > 0 {
				rate *= 25
			}
			entries = append(entries, Entry{
				MonsterID: m.ID, ItemID: it.ID, Chance: rate,
				Minimum: 1, Maximum: 1, Note: it.Name,
			})
		}
		// second Java pass: monstercarddata
		if it.ID == lastCardID {
			continue
		}
		if m, ok := findMob(name); ok {
			cards = append(cards, Card{Seq: seq, CardID: it.ID, MonsterID: m.ID})
			lastCardID = it.ID
			seq++
		}
	}
	return entries, cards
}

// expandDrops writes one row per copy of a multi-drop item, honouring
// IncrementRate per copy (Java: multipleDropsIncrement + IncrementRate).
func expandDrops(monsterID, itemID, rate int) []Entry {
	n := multipleDropsIncrement(itemID, monsterID)
	out := make([]Entry, 0, n)
	for i := 0; i < n; i++ {
		r := incrementRate(itemID, i)
		if r == -1 {
			r = rate
		}
		out = append(out, Entry{
			MonsterID: monsterID, ItemID: itemID, Chance: r,
			Minimum: 1, Maximum: 1,
		})
	}
	return out
}

func isBoss(mobs []mobRef, id int) bool {
	for _, m := range mobs {
		if m.ID == id {
			return m.Boss > 0
		}
	}
	return false
}

// leftPad is tools.StringUtil.getLeftPaddedStr: pad s on the left until it
// is `width` runes long (Mob.wz file names are 0000100100.img for mob 100100).
func leftPad(s string, pad byte, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(string(pad), width-len(s)) + s
}

// Monsters reports the distinct monster ids in the result (useful for the
// tool's stats line).
func (r *Result) Monsters() int {
	seen := map[int]bool{}
	for _, e := range r.Entries {
		seen[e.MonsterID] = true
	}
	return len(seen)
}

// Summary is the one-line human summary the tool prints.
func (r *Result) Summary() string {
	return fmt.Sprintf("items=%d mobs=%d (boss %d) monsterbook rows=%d "+
		"entries=%d monsters-with-drops=%d cards=%d",
		r.Stats.Items, r.Stats.Mobs, r.Stats.Bosses, r.Stats.MonsterBooks,
		len(r.Entries), r.Monsters(), len(r.Cards))
}

// SortEntries orders rows the way the SQL writer emits them: by monster, then
// item, then chance (stable, so diffs stay readable).
func (r *Result) SortEntries() {
	sort.SliceStable(r.Entries, func(i, j int) bool {
		a, b := r.Entries[i], r.Entries[j]
		if a.MonsterID != b.MonsterID {
			return a.MonsterID < b.MonsterID
		}
		if a.ItemID != b.ItemID {
			return a.ItemID < b.ItemID
		}
		return a.Chance < b.Chance
	})
}
