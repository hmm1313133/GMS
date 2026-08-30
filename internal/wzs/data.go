package wzs

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Point is the Go stand-in for java.awt.Point, the payload of a VECTOR node.
type Point struct{ X, Y int }

// Canvas is the Go stand-in for provider.MapleCanvas.
//
// The 079MAX2 wz export ships no PNG files at all (39,986 files, all .xml -
// nothing on the server renders art), so only the declared size is stored.
// PNGPath reproduces what provider.WzXML.FileStoredPngMapleCanvas would have
// probed, purely so tooling can report the same path Java would.
type Canvas struct {
	Width, Height int
	PNGPath       string
}

// Node is a wz data node: the Go port of provider.MapleData as implemented by
// provider.WzXML.XMLDomMapleData over the exported wz XML.
//
// A Node is immutable once parsed, so it is safe to share between goroutines
// (the child index is built at the end of parsing, never lazily).
type Node struct {
	Name     string
	Type     DataType
	children []*Node
	parent   *Node
	index    map[string]*Node // built at parse end when there are enough children
	iv       int64            // SHORT / INT / LONG
	fv       float64          // FLOAT / DOUBLE
	sv       string           // STRING / UOL
	pt       Point            // VECTOR
	cv       Canvas           // CANVAS (dimensions only)
	baseDir  string           // root node only: directory holding the .xml file
}

// indexThreshold: below this many children a linear scan beats a map.
const indexThreshold = 8

// Parent returns the containing node, nil at the document root.
func (n *Node) Parent() *Node {
	if n == nil {
		return nil
	}
	return n.parent
}

// Children returns the child nodes in document order (Java getChildren).
func (n *Node) Children() []*Node {
	if n == nil {
		return nil
	}
	return n.children
}

// Child returns the first child with the given name, nil when absent.
// Duplicate names keep the first occurrence, matching the Java XML lookup.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	if n.index != nil {
		return n.index[name]
	}
	for _, c := range n.children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ChildByPath resolves a "/"-separated path (Java getChildByPath).
//
// Java only special-cases a leading ".." (it delegates to the parent); a ".."
// in any other position is looked up as a literal child name there. Go keeps
// that rule so wz data that really contains a node named ".." still resolves
// the same way.
func (n *Node) ChildByPath(path string) *Node {
	if n == nil {
		return nil
	}
	if path == ".." {
		return n.parent
	}
	if strings.HasPrefix(path, "../") {
		if n.parent == nil {
			return nil
		}
		return n.parent.ChildByPath(strings.TrimPrefix(path, "../"))
	}
	cur := n
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || seg == "." {
			continue
		}
		cur = cur.Child(seg)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// Data returns the typed payload, mirroring XMLDomMapleData.getData:
//
//	DOUBLE -> float64, FLOAT -> float32, INT -> int, LONG -> int64,
//	SHORT -> int16, STRING/UOL -> string, VECTOR -> Point, CANVAS -> Canvas,
//	everything else (PROPERTY, CONVEX, SOUND, null, ...) -> nil
func (n *Node) Data() any {
	if n == nil {
		return nil
	}
	switch n.Type {
	case TypeDouble:
		return n.fv
	case TypeFloat:
		return float32(n.fv)
	case TypeInt:
		return int(n.iv)
	case TypeLong:
		return n.iv
	case TypeShort:
		return int16(n.iv)
	case TypeString, TypeUOL:
		return n.sv
	case TypeVector:
		return n.pt
	case TypeCanvas:
		return Canvas{Width: n.cv.Width, Height: n.cv.Height, PNGPath: n.PNGPath()}
	}
	return nil
}

// PNGPath reconstructs the PNG location Java would have probed for a canvas
// node: <xml dir>/<root name>/.../<parent name>/<name>.png. Empty when the
// node was parsed without a file backing (see Provider.Data).
func (n *Node) PNGPath() string {
	base := n.baseDirOf()
	if base == "" {
		return ""
	}
	names := make([]string, 0, 8)
	for cur := n; cur != nil; cur = cur.parent {
		names = append(names, cur.Name)
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	dir := filepath.Join(append([]string{base}, names[:len(names)-1]...)...)
	return filepath.Join(dir, n.Name+".png")
}

func (n *Node) baseDirOf() string {
	root := n
	for root.parent != nil {
		root = root.parent
	}
	return root.baseDir
}

// Walk visits n and every descendant in document order, depth first.
func (n *Node) Walk(fn func(*Node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.children {
		c.Walk(fn)
	}
}

func (n *Node) addChild(c *Node) {
	c.parent = n
	n.children = append(n.children, c)
}

// finalize is called when the element closes: it builds the name index for
// wide nodes so Child() stays O(1) on things like String.wz/Eqp.img.
func (n *Node) finalize() {
	if len(n.children) >= indexThreshold {
		n.index = make(map[string]*Node, len(n.children))
		for _, c := range n.children {
			if _, dup := n.index[c.Name]; !dup {
				n.index[c.Name] = c
			}
		}
	}
}

// ---------------------------------------------------------------------------
// MapleDataTool
// ---------------------------------------------------------------------------

// The Java helpers (provider.MapleDataTool) throw NPE / ClassCastException on
// missing nodes and type mismatches; the Go versions return the default /
// zero value instead, which is what every call site actually wanted (they all
// pass a fallback or immediately null-check).

// GetString returns the STRING/UOL payload (Java getString).
func GetString(d *Node) string {
	if d == nil {
		return ""
	}
	return d.sv
}

// GetStringDef returns def when d is nil or holds no string.
func GetStringDef(d *Node, def string) string {
	if d == nil {
		return def
	}
	if d.Type != TypeString && d.Type != TypeUOL {
		return def
	}
	return d.sv
}

// GetStringPath resolves path then reads the string (Java getString(String, MapleData)).
func GetStringPath(path string, d *Node) string {
	return GetString(d.ChildByPath(path))
}

// GetStringPathDef resolves path then reads the string, falling back to def.
func GetStringPathDef(path string, d *Node, def string) string {
	return GetStringDef(d.ChildByPath(path), def)
}

// GetInt reads an INT node (Java getInt).
func GetInt(d *Node) int { return GetIntDef(d, 0) }

// GetIntDef mirrors Java getInt(MapleData, int): a STRING payload is parsed
// (wz stores some numbers as strings), SHORT/INT/LONG are widened, anything
// missing yields def.
func GetIntDef(d *Node, def int) int {
	if d == nil || d.Data() == nil {
		return def
	}
	switch d.Type {
	case TypeString:
		v, err := strconv.Atoi(strings.TrimSpace(d.sv))
		if err != nil {
			return def
		}
		return v
	case TypeShort:
		return int(int16(d.iv))
	case TypeInt, TypeLong:
		return int(d.iv)
	case TypeFloat, TypeDouble:
		return int(d.fv)
	}
	return def
}

// GetIntPath resolves path then reads the int (Java getInt(String, MapleData)).
// A missing node yields 0 - use GetIntPathDef when a fallback is needed.
func GetIntPath(path string, d *Node) int { return GetIntPathDef(path, d, 0) }

// GetIntPathDef resolves path then reads the int with a fallback.
func GetIntPathDef(path string, d *Node, def int) int {
	return GetIntDef(d.ChildByPath(path), def)
}

// GetIntConvert is Java getIntConvert: STRING payloads are parsed, everything
// else goes through GetIntDef.
func GetIntConvert(d *Node) int {
	if d != nil && d.Type == TypeString {
		if v, err := strconv.Atoi(strings.TrimSpace(d.sv)); err == nil {
			return v
		}
	}
	return GetIntDef(d, 0)
}

// GetIntConvertPath is Java getIntConvert(String, MapleData).
func GetIntConvertPath(path string, d *Node) int { return GetIntConvert(d.ChildByPath(path)) }

// GetIntConvertPathDef is Java getIntConvert(String, MapleData, int): an
// unparseable string falls back to def instead of throwing.
//
// Note: Java also carries getIntConvert2 with byte-for-byte identical
// behaviour - Go keeps one implementation.
func GetIntConvertPathDef(path string, d *Node, def int) int {
	child := d.ChildByPath(path)
	if child == nil {
		return def
	}
	if child.Type == TypeString {
		v, err := strconv.Atoi(strings.TrimSpace(child.sv))
		if err != nil {
			return def
		}
		return v
	}
	return GetIntDef(child, def)
}

// GetLong reads a LONG node (Java getLong).
func GetLong(d *Node) int64 {
	if d == nil || d.Data() == nil {
		return 0
	}
	if d.Type == TypeString {
		v, err := strconv.ParseInt(strings.TrimSpace(d.sv), 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return d.iv
}

// GetLongConvertPath is Java getLongConvert(String, MapleData): STRING is
// parsed, INT is widened.
func GetLongConvertPath(path string, d *Node) int64 {
	child := d.ChildByPath(path)
	if child == nil {
		return 0
	}
	if child.Type == TypeString {
		if v, err := strconv.ParseInt(strings.TrimSpace(child.sv), 10, 64); err == nil {
			return v
		}
		return 0
	}
	return GetLong(child)
}

// GetFloat reads a FLOAT node (Java getFloat).
func GetFloat(d *Node) float32 { return GetFloatDef(d, 0) }

// GetFloatDef reads a FLOAT node with a fallback.
func GetFloatDef(d *Node, def float32) float32 {
	if d == nil || d.Data() == nil {
		return def
	}
	if d.Type == TypeString {
		v, err := strconv.ParseFloat(strings.TrimSpace(d.sv), 32)
		if err != nil {
			return def
		}
		return float32(v)
	}
	return float32(d.fv)
}

// GetDouble reads a DOUBLE node (Java getDouble).
func GetDouble(d *Node) float64 {
	if d == nil || d.Data() == nil {
		return 0
	}
	if d.Type == TypeString {
		v, err := strconv.ParseFloat(strings.TrimSpace(d.sv), 64)
		if err != nil {
			return 0
		}
		return v
	}
	return d.fv
}

// GetPoint reads a VECTOR node (Java getPoint).
func GetPoint(d *Node) Point {
	if d == nil || d.Type != TypeVector {
		return Point{}
	}
	return d.pt
}

// GetPointPath resolves path then reads the vector (Java getPoint(String, MapleData)).
func GetPointPath(path string, d *Node) Point { return GetPoint(d.ChildByPath(path)) }

// GetPointPathDef resolves path then reads the vector with a fallback.
func GetPointPathDef(path string, d *Node, def Point) Point {
	child := d.ChildByPath(path)
	if child == nil || child.Type != TypeVector {
		return def
	}
	return child.pt
}

// FullDataPath is Java MapleDataTool.getFullDataPath: the "/"-joined chain of
// names from the document root down to d.
func FullDataPath(d *Node) string {
	names := make([]string, 0, 8)
	for cur := d; cur != nil; cur = cur.parent {
		names = append(names, cur.Name)
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, "/")
}
