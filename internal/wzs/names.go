package wzs

import (
	"errors"
	"strconv"
	"strings"
)

// Default fallbacks used by the Java factories when a name is missing.
const (
	NoItemName = "NO-NAME"   // MapleItemInformationProvider.getAllItems
	NoNPCName  = "MISSINGNO" // MapleLifeFactory.getNPC
)

// Names caches the display-name tables the server keeps hot.
//
// Java equivalents (each factory parses these images into static fields at
// class-load time):
//   - server.MapleItemInformationProvider: Cash.img / Consume.img /
//     Eqp.img->"Eqp" / Etc.img->"Etc" / Ins.img / Pet.img
//   - server.life.MapleLifeFactory: String.wz Mob.img, Npc.img
//   - client.SkillFactory.getName: String.wz Skill.img
//   - server.maps.MapleMapFactory: String.wz Map.img (mapName + streetName)
//   - handling.login.LoginInformationProvider: Etc.wz ForbiddenName.img
//
// Deliberate deviation: Java looks names up by computed path
// (SkillFactory pads the id to 7 digits, MapleMapFactory maps the map id to a
// region name first). That breaks on this distribution - Skill.img carries an
// 8-digit key (10000018) and Map.img has no "china" region although
// MapleMapFactory asks for one - so Go builds one id-keyed index per table
// instead. Names that Java would have missed are now found.
type Names struct {
	Item      map[int]ItemName
	Map       map[int]MapName
	Mob       map[int]string
	NPC       map[int]string
	Skill     map[int]string
	Forbidden []string

	Counts NameCounts
	Errors []string // non-fatal image problems (missing image, ...), capped
}

// ItemName is the item display name and tooltip description.
type ItemName struct {
	Name string
	Desc string
}

// MapName is a map's display name plus the street (region) it belongs to.
type MapName struct {
	Name   string // mapName
	Street string // streetName
}

// NameCounts reports how many entries each table ended up with.
type NameCounts struct {
	Items, Maps, Mobs, NPCs, Skills, Forbidden int
}

// itemSources lists the String.wz images holding item names. The second field
// is the sub-path Java descends into ("Eqp.img" -> "Eqp").
var itemSources = []struct{ image, sub string }{
	{"Cash.img", ""},
	{"Consume.img", ""},
	{"Eqp.img", "Eqp"},
	{"Etc.img", "Etc"},
	{"Ins.img", ""},
	{"Pet.img", ""},
}

// LoadNames reads every name table. A missing/unreadable image is recorded in
// Errors and skipped; only a completely unusable String.wz fails the call.
func LoadNames(r *Root) (*Names, error) {
	n := &Names{
		Item:  map[int]ItemName{},
		Map:   map[int]MapName{},
		Mob:   map[int]string{},
		NPC:   map[int]string{},
		Skill: map[int]string{},
	}

	strWZ, err := r.WZ("String.wz")
	if err != nil {
		return nil, err
	}

	for _, src := range itemSources {
		root, err := strWZ.Data(src.image)
		if err != nil {
			n.note(src.image + ": " + err.Error())
			continue
		}
		if src.sub != "" {
			root = root.ChildByPath(src.sub)
			if root == nil {
				n.note(src.image + ": missing sub-path " + src.sub)
				continue
			}
		}
		// Eqp/Etc nest one more level (equip category / etc category)
		for _, cat := range root.Children() {
			for _, item := range cat.Children() {
				id, ok := parseID(item.Name)
				if !ok {
					continue
				}
				n.Item[id] = ItemName{
					Name: GetStringPathDef("name", item, NoItemName),
					Desc: GetStringPathDef("desc", item, ""),
				}
			}
			if id, ok := parseID(cat.Name); ok {
				n.Item[id] = ItemName{
					Name: GetStringPathDef("name", cat, NoItemName),
					Desc: GetStringPathDef("desc", cat, ""),
				}
			}
		}
	}

	if root, err := strWZ.Data("Map.img"); err == nil {
		for _, region := range root.Children() {
			for _, m := range region.Children() {
				id, ok := parseID(m.Name)
				if !ok {
					continue
				}
				n.Map[id] = MapName{
					Name:   GetStringPathDef("mapName", m, ""),
					Street: GetStringPathDef("streetName", m, ""),
				}
			}
		}
	} else {
		n.note("Map.img: " + err.Error())
	}

	n.Mob = loadNameTable(strWZ, "Mob.img", n)
	n.NPC = loadNameTable(strWZ, "Npc.img", n)
	n.Skill = loadNameTable(strWZ, "Skill.img", n)

	if etcWZ, err := r.WZ("Etc.wz"); err == nil {
		if root, err := etcWZ.Data("ForbiddenName.img"); err == nil {
			for _, c := range root.Children() {
				if c.Type == TypeString && c.sv != "" {
					n.Forbidden = append(n.Forbidden, c.sv)
				}
			}
		} else {
			n.note("Etc.wz/ForbiddenName.img: " + err.Error())
		}
	} else {
		n.note("Etc.wz: " + err.Error())
	}

	n.Counts = NameCounts{
		Items:     len(n.Item),
		Maps:      len(n.Map),
		Mobs:      len(n.Mob),
		NPCs:      len(n.NPC),
		Skills:    len(n.Skill),
		Forbidden: len(n.Forbidden),
	}
	return n, nil
}

func loadNameTable(p *Provider, image string, n *Names) map[int]string {
	out := map[int]string{}
	root, err := p.Data(image)
	if err != nil {
		n.note(image + ": " + err.Error())
		return out
	}
	for _, c := range root.Children() {
		id, ok := parseID(c.Name)
		if !ok {
			continue // Skill.img carries a "000" bookName entry
		}
		if name, ok := stringValue(c); ok {
			out[id] = name
		}
	}
	return out
}

func (n *Names) note(msg string) {
	if len(n.Errors) < 20 {
		n.Errors = append(n.Errors, msg)
	}
}

// ItemName returns the cached item name/desc table entry.
func (n *Names) ItemName(id int) (ItemName, bool) {
	v, ok := n.Item[id]
	return v, ok
}

// MapName returns the cached mapName/streetName pair.
func (n *Names) MapName(id int) (MapName, bool) {
	v, ok := n.Map[id]
	return v, ok
}

// MobName returns a monster name, Java's "NO-NAME" fallback when unknown.
func (n *Names) MobName(id int) string {
	if v, ok := n.Mob[id]; ok {
		return v
	}
	return NoItemName
}

// NPCName returns an NPC name, Java's "MISSINGNO" fallback when unknown.
func (n *Names) NPCName(id int) string {
	if v, ok := n.NPC[id]; ok {
		return v
	}
	return NoNPCName
}

// SkillName returns a skill name ("" when unknown - Java returns null there).
func (n *Names) SkillName(id int) string { return n.Skill[id] }

// IsForbiddenName mirrors LoginInformationProvider.isForbiddenName: true when
// the name contains any forbidden substring.
func (n *Names) IsForbiddenName(name string) bool {
	for _, bad := range n.Forbidden {
		if strings.Contains(name, bad) {
			return true
		}
	}
	return false
}

func stringValue(c *Node) (string, bool) {
	if c == nil {
		return "", false
	}
	if c.Type == TypeString {
		return c.sv, true
	}
	if child := c.Child("name"); child != nil && child.Type == TypeString {
		return child.sv, true
	}
	return "", false
}

func parseID(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ErrNoNames is returned when a name table cannot be loaded at all.
var ErrNoNames = errors.New("wzs: name tables unavailable")
