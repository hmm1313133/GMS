package wzs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testWZRoot = "testdata/wz"

func openTestRoot(t *testing.T) *Root {
	t.Helper()
	r, err := OpenRoot(testWZRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	return r
}

func mustData(t *testing.T, p *Provider, path string) *Node {
	t.Helper()
	n, err := p.Data(path)
	if err != nil {
		t.Fatalf("Data(%q): %v", path, err)
	}
	return n
}

func TestDataTypeOf(t *testing.T) {
	cases := map[string]DataType{
		"imgdir":  TypeProperty,
		"canvas":  TypeCanvas,
		"convex":  TypeConvex,
		"sound":   TypeSound,
		"uol":     TypeUOL,
		"double":  TypeDouble,
		"float":   TypeFloat,
		"int":     TypeInt,
		"long":    TypeLong,
		"short":   TypeShort,
		"string":  TypeString,
		"vector":  TypeVector,
		"null":    TypeImg0x00,
		"weird":   TypeUnknownType,
		"extendo": TypeUnknownType,
	}
	for tag, want := range cases {
		if got := dataTypeOf(tag); got != want {
			t.Errorf("dataTypeOf(%q) = %v, want %v", tag, got, want)
		}
	}
	if TypeProperty.String() != "PROPERTY" || TypeImg0x00.String() != "IMG_0x00" {
		t.Errorf("DataType.String mismatch: %v / %v", TypeProperty, TypeImg0x00)
	}
}

func TestParseScalarTypes(t *testing.T) {
	root := mustData(t, mustWZ(t, openTestRoot(t), "Types.wz"), "types.img")

	if root.Name != "types.img" || root.Type != TypeProperty {
		t.Fatalf("root = %q/%v", root.Name, root.Type)
	}
	checks := []struct {
		name string
		typ  DataType
		data any
	}{
		{"version", TypeInt, 10},
		{"slotMax", TypeShort, int16(200)},
		{"exp", TypeLong, int64(123456789012)},
		{"mobRate", TypeFloat, float32(1.5)},
		{"rate", TypeDouble, 0.25},
		{"name", TypeString, "测试物品"},
		{"link", TypeUOL, "../0100/info"},
		{"origin", TypeVector, Point{24, -17}},
		{"blank", TypeInt, 0},                                 // empty value tolerated
		{"parent", TypeImg0x00, nil},                          // null node has no payload
		{"bgm", TypeSound, nil},                               // resolved through info/
	}
	for _, c := range checks {
		var n *Node
		if c.name == "bgm" {
			n = root.ChildByPath("info/" + c.name)
		} else {
			n = root.Child(c.name)
		}
		if n == nil {
			t.Fatalf("child %q missing", c.name)
		}
		if n.Type != c.typ {
			t.Errorf("%s type = %v, want %v", c.name, n.Type, c.typ)
		}
		if got := n.Data(); got != c.data {
			t.Errorf("%s data = %#v, want %#v", c.name, got, c.data)
		}
	}

	// unknown tag -> UNKNOWN_TYPE, kept as a node (Java would have NPE'd later)
	if u := root.Child("unknown"); u == nil || u.Type != TypeUnknownType {
		t.Errorf("unknown node = %#v", u)
	}
	// containers expose no scalar payload (canvas does: it has a size)
	for _, name := range []string{"hull", "info"} {
		if v := root.Child(name).Data(); v != nil {
			t.Errorf("%s data = %#v, want nil", name, v)
		}
	}
}

func TestCanvasAndConvex(t *testing.T) {
	root := mustData(t, mustWZ(t, openTestRoot(t), "Types.wz"), "types.img")

	icon := root.Child("icon")
	cv, ok := icon.Data().(Canvas)
	if !ok {
		t.Fatalf("canvas data = %#v", icon.Data())
	}
	if cv.Width != 48 || cv.Height != 34 {
		t.Errorf("canvas size = %dx%d, want 48x34", cv.Width, cv.Height)
	}
	wantPNG := filepath.Join("testdata/wz/Types.wz", "types.img", "icon.png")
	if cv.PNGPath != filepath.FromSlash(wantPNG) {
		t.Errorf("PNGPath = %q, want %q", cv.PNGPath, wantPNG)
	}
	if got := GetPointPath("origin", icon); got != (Point{24, 17}) {
		t.Errorf("canvas origin = %v", got)
	}

	// convex keeps ordered children like a property
	hull := root.Child("hull")
	if hull.Type != TypeConvex || len(hull.Children()) != 2 {
		t.Fatalf("convex = %v with %d children", hull.Type, len(hull.Children()))
	}
	if got := GetPoint(hull.Child("1")); got != (Point{3, 4}) {
		t.Errorf("convex[1] = %v", got)
	}
}

func TestChildByPath(t *testing.T) {
	root := mustData(t, mustWZ(t, openTestRoot(t), "Types.wz"), "types.img")

	if got := root.ChildByPath("info/nested/deep"); got == nil || GetInt(got) != 7 {
		t.Errorf("info/nested/deep = %#v", got)
	}
	if got := root.ChildByPath("info/nested").ChildByPath("../../version"); got == nil || GetInt(got) != 10 {
		t.Errorf(".. traversal = %#v", got)
	}
	if got := root.ChildByPath("nope/nested"); got != nil {
		t.Errorf("missing path = %#v, want nil", got)
	}
	if got := root.ChildByPath(".."); got != nil {
		t.Errorf("root/.. = %#v, want nil (root has no parent)", got)
	}
	if got := root.ChildByPath("info/nested").ChildByPath(".."); got == nil || got.Name != "info" {
		t.Errorf("bare .. = %#v", got)
	}
	if got := FullDataPath(root.ChildByPath("info/nested/deep")); got != "types.img/info/nested/deep" {
		t.Errorf("FullDataPath = %q", got)
	}
}

func TestDataToolConversions(t *testing.T) {
	root := mustData(t, mustWZ(t, openTestRoot(t), "Types.wz"), "types.img")

	if got := GetIntPath("version", root); got != 10 {
		t.Errorf("GetIntPath(version) = %d", got)
	}
	// wz stores some numbers as strings; Java getIntConvert parses them
	if got := GetIntConvertPath("numAsString", root); got != 1234 {
		t.Errorf("GetIntConvertPath(numAsString) = %d", got)
	}
	if got := GetIntConvertPathDef("name", root, -1); got != -1 {
		t.Errorf("GetIntConvertPathDef(name) = %d, want -1", got)
	}
	if got := GetIntPathDef("missing", root, 42); got != 42 {
		t.Errorf("GetIntPathDef(missing) = %d", got)
	}
	// SHORT widens, empty string falls back
	if got := GetInt(root.Child("slotMax")); got != 200 {
		t.Errorf("GetInt(short) = %d", got)
	}
	if got := GetStringPathDef("emptyString", root, "fallback"); got != "" {
		t.Errorf("empty string should stay empty, got %q", got)
	}
	if got := GetStringPathDef("missing", root, "fallback"); got != "fallback" {
		t.Errorf("GetStringPathDef(missing) = %q", got)
	}
	if got := GetFloat(root.Child("mobRate")); got != 1.5 {
		t.Errorf("GetFloat = %v", got)
	}
	if got := GetDouble(root.Child("rate")); got != 0.25 {
		t.Errorf("GetDouble = %v", got)
	}
	if got := GetLong(root.Child("exp")); got != 123456789012 {
		t.Errorf("GetLong = %d", got)
	}
	if got := GetLongConvertPath("numAsString", root); got != 1234 {
		t.Errorf("GetLongConvertPath = %d", got)
	}
	if got := GetPoint(root.Child("origin")); got != (Point{24, -17}) {
		t.Errorf("GetPoint = %v", got)
	}
	if got := GetPointPathDef("missing", root, Point{9, 9}); got != (Point{9, 9}) {
		t.Errorf("GetPointPathDef = %v", got)
	}
	// nil safety: Java throws NPE, Go yields zero values
	if got := GetString(nil); got != "" {
		t.Errorf("GetString(nil) = %q", got)
	}
	if got := GetIntPath("x", nil); got != 0 {
		t.Errorf("GetIntPath on nil = %d", got)
	}
}

func TestProviderNavigation(t *testing.T) {
	r := openTestRoot(t)

	names, err := r.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	want := map[string]bool{"Map.wz": false, "Reactor.wz": false, "String.wz": false, "Types.wz": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("wz %s not listed (got %v)", n, names)
		}
	}

	mapWZ := mustWZ(t, r, "Map.wz")
	root := mapWZ.Root()
	mapDir := root.Entry("Map")
	if mapDir == nil || !mapDir.IsDir {
		t.Fatalf("Map.wz/Map missing")
	}
	map0 := mapDir.Entry("Map0")
	if map0 == nil {
		t.Fatalf("Map.wz/Map/Map0 missing")
	}
	files := map0.Files()
	if len(files) != 1 || files[0].Name != "000000000.img" {
		t.Fatalf("Map0 files = %+v", files)
	}
	if err == nil {
		if got := root.Entry("Physics.img"); got == nil || got.IsDir {
			t.Errorf("Physics.img file entry = %+v", got)
		}
	}
	if got := root.Entry("Nope.img"); got != nil {
		t.Errorf("Entry(Nope.img) = %+v, want nil", got)
	}
	if got := root.Subdirectories(); len(got) != 1 || got[0].Name != "Map" {
		t.Errorf("Subdirectories = %+v", got)
	}

	// same provider instance for repeated WZ() calls, and caching returns the
	// identical tree
	if again, err := r.WZ("Map.wz"); err != nil || again != mapWZ {
		t.Errorf("WZ(Map.wz) not memoised")
	}
	a := mustData(t, mapWZ, "Map/Map0/000000000.img")
	b := mustData(t, mapWZ, "Map/Map0/000000000.img")
	if a != b {
		t.Errorf("cached Data returned a different tree")
	}
	uncached, err := OpenUncached(filepath.Join(testWZRoot, "Map.wz"))
	if err != nil {
		t.Fatalf("OpenUncached: %v", err)
	}
	c := mustData(t, uncached, "Map/Map0/000000000.img")
	if c == a {
		t.Errorf("uncached provider should re-parse")
	}
}

func TestProviderErrors(t *testing.T) {
	r := openTestRoot(t)
	p := mustWZ(t, r, "Map.wz")

	if _, err := p.Data("Nope.img"); err == nil || !strings.Contains(err.Error(), "Nope.img") {
		t.Errorf("Data(missing) error = %v", err)
	}
	for _, bad := range []string{"../Map.wz/Physics.img", "/abs/path", ""} {
		if _, err := p.Data(bad); err == nil {
			t.Errorf("Data(%q) should be rejected", bad)
		}
	}
	if _, err := r.WZ("Nope.wz"); err == nil {
		t.Errorf("WZ(Nope.wz) should fail")
	}
}

func TestRealMapImage(t *testing.T) {
	p := mustWZ(t, openTestRoot(t), "Map.wz")
	root := mustData(t, p, "Map/Map0/000000000.img")

	if got := GetIntPathDef("info/returnMap", root, -1); got != 10000 {
		t.Errorf("returnMap = %d", got)
	}
	if got := GetStringPathDef("info/bgm", root, ""); got != "Bgm00/GoPicnic" {
		t.Errorf("bgm = %q", got)
	}
	if got := GetFloat(root.ChildByPath("info/mobRate")); got != 1.0 {
		t.Errorf("mobRate = %v", got)
	}
	if got := GetIntPathDef("info/VRLeft", root, 0); got != -400 {
		t.Errorf("VRLeft = %d", got)
	}
	if got := root.ChildByPath("info/onFirstUserEnter"); got == nil || got.Type != TypeString {
		t.Errorf("onFirstUserEnter = %#v", got)
	}
	for _, section := range []string{"back", "foothold", "life", "portal", "reactor", "info"} {
		if root.Child(section) == nil {
			t.Errorf("section %q missing from 000000000.img", section)
		}
	}
}

func TestRealReactorUOL(t *testing.T) {
	p := mustWZ(t, openTestRoot(t), "Reactor.wz")
	root := mustData(t, p, "0002000.img")

	state1 := root.Child("1")
	if state1 == nil {
		t.Fatalf("state 1 missing")
	}
	if got := GetStringPathDef("0", state1, ""); got != "../0/0" {
		t.Errorf("uol value = %q", got)
	}
	if got := state1.Child("0"); got == nil || got.Type != TypeUOL {
		t.Errorf("uol node = %#v", got)
	}
	hit := root.ChildByPath("0/hit/0")
	if hit == nil || hit.Type != TypeCanvas {
		t.Fatalf("0/hit/0 = %#v", hit)
	}
	cv := hit.Data().(Canvas)
	if cv.Width != 48 || cv.Height != 34 {
		t.Errorf("canvas = %+v", cv)
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse(strings.NewReader(`<imgdir name="x"><int name="bad" value="12abc"/></imgdir>`))
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Errorf("Parse(bad int) error = %v", err)
	}
	if _, err := Parse(strings.NewReader(`<imgdir name="x"><int name="y" value="1"/>`)); err == nil {
		t.Errorf("unclosed document should fail")
	}
	if _, err := Parse(strings.NewReader("")); err == nil {
		t.Errorf("empty document should fail")
	}
}

// TestRealWZTree walks the full workspace wz export (I:\GMS\wz) when it is
// present: 079MAX2 ships 39,986 XML files and this is the "every file parses"
// smoke for P3.1. Skipped when the (gitignored) export has not been copied in.
func TestRealWZTree(t *testing.T) {
	const realRoot = "../../wz"
	if _, err := os.Stat(realRoot); err != nil {
		t.Skipf("full wz export not present at %s", realRoot)
	}
	r, err := OpenRoot(realRoot)
	if err != nil {
		t.Fatalf("OpenRoot(%s): %v", realRoot, err)
	}
	names, err := r.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if len(names) < 16 {
		t.Errorf("expected 16 wz directories, got %d (%v)", len(names), names)
	}

	type sample struct {
		wz    string
		limit int // 0 = all files
	}
	plan := []sample{
		{"String.wz", 0},
		{"Reactor.wz", 0},
		{"Map.wz", 200},
		{"Character.wz", 100},
		{"Sound.wz", 40},
	}
	var parsed int
	for _, s := range plan {
		p, err := r.WZ(s.wz)
		if err != nil {
			t.Fatalf("WZ(%s): %v", s.wz, err)
		}
		files, err := collectImages(t, p.Root(), s.limit)
		if err != nil {
			t.Fatalf("walk %s: %v", s.wz, err)
		}
		if len(files) == 0 {
			t.Errorf("%s: no images found", s.wz)
		}
		for _, rel := range files {
			n, err := p.Data(rel)
			if err != nil {
				t.Fatalf("%s/%s: %v", s.wz, rel, err)
			}
			if n == nil || n.Name == "" {
				t.Fatalf("%s/%s: empty root", s.wz, rel)
			}
			parsed++
		}
		p.Purge()
	}
	t.Logf("parsed %d images across %d wz files", parsed, len(plan))
}

func mustWZ(t *testing.T, r *Root, name string) *Provider {
	t.Helper()
	p, err := r.WZ(name)
	if err != nil {
		t.Fatalf("WZ(%q): %v", name, err)
	}
	return p
}

// collectImages walks a provider's navigation tree and returns wz-relative
// image paths ("Map/Map0/000000000.img"), capped at limit (0 = unlimited).
func collectImages(t *testing.T, e *DirEntry, limit int) ([]string, error) {
	t.Helper()
	var out []string
	var walk func(*DirEntry) bool
	walk = func(dir *DirEntry) bool {
		for _, f := range dir.Files() {
			rel, err := filepath.Rel(e.Path(), f.Path())
			if err != nil {
				t.Fatalf("rel: %v", err)
			}
			// entry names drop the ".xml" suffix (Java WZFileEntry does the same)
			out = append(out, filepath.ToSlash(strings.TrimSuffix(rel, ".xml")))
			if limit > 0 && len(out) >= limit {
				return false
			}
		}
		for _, sub := range dir.Subdirectories() {
			if !walk(sub) {
				return false
			}
		}
		return true
	}
	walk(e)
	return out, nil
}
