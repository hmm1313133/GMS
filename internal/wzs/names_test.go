package wzs

import (
	"os"
	"testing"
)

func TestLoadNamesFixtures(t *testing.T) {
	n, err := LoadNames(openTestRoot(t))
	if err != nil {
		t.Fatalf("LoadNames: %v", err)
	}
	if len(n.Errors) != 0 {
		t.Errorf("unexpected load errors: %v", n.Errors)
	}

	want := NameCounts{Items: 8, Maps: 2, Mobs: 2, NPCs: 1, Skills: 2, Forbidden: 3}
	if n.Counts != want {
		t.Errorf("counts = %+v, want %+v", n.Counts, want)
	}

	// items: flat images (Cash) and nested ones (Eqp.img/Eqp/<category>/<id>)
	if got, ok := n.ItemName(5010000); !ok || got.Name != "测试现金道具" || got.Desc != "这是一段描述" {
		t.Errorf("item 5010000 = %+v (ok=%v)", got, ok)
	}
	if got, ok := n.ItemName(5010001); !ok || got.Name != "第二个道具" || got.Desc != "" {
		t.Errorf("item 5010001 = %+v (ok=%v)", got, ok)
	}
	if got, ok := n.ItemName(1302000); !ok || got.Name != "木剑" || got.Desc != "新手用武器" {
		t.Errorf("item 1302000 = %+v (ok=%v)", got, ok)
	}
	if got, ok := n.ItemName(1010000); !ok || got.Name != "红头巾" {
		t.Errorf("item 1010000 = %+v (ok=%v)", got, ok)
	}
	if _, ok := n.ItemName(1); ok {
		t.Errorf("unknown item should be absent")
	}

	// map names, keyed by id instead of Java's computed region path
	if got, ok := n.MapName(700000000); !ok || got.Name != "红鸾宫入口" || got.Street != "红鸾宫" {
		t.Errorf("map 700000000 = %+v (ok=%v)", got, ok)
	}
	if got, ok := n.MapName(100000000); !ok || got.Name != "彩虹村" || got.Street != "彩虹岛" {
		t.Errorf("map 100000000 = %+v (ok=%v)", got, ok)
	}

	if got := n.MobName(100100); got != "蜗牛" {
		t.Errorf("MobName(100100) = %q", got)
	}
	if got := n.MobName(999); got != NoItemName {
		t.Errorf("MobName(999) = %q, want %q", got, NoItemName)
	}
	if got := n.NPCName(1002000); got != "测试NPC" {
		t.Errorf("NPCName = %q", got)
	}
	if got := n.NPCName(1); got != NoNPCName {
		t.Errorf("NPCName(1) = %q, want %q", got, NoNPCName)
	}
	// 8-digit skill keys are outside Java's 7-digit padding
	if got := n.SkillName(8); got != "群宠" {
		t.Errorf("SkillName(8) = %q", got)
	}
	if got := n.SkillName(10000018); got != "扩展技能" {
		t.Errorf("SkillName(10000018) = %q", got)
	}
	if got := n.SkillName(42); got != "" {
		t.Errorf("SkillName(42) = %q, want empty", got)
	}
	// the "000" bookName entry carries no name
	if got := n.SkillName(0); got != "" {
		t.Errorf("SkillName(0) = %q, want empty", got)
	}

	if !n.IsForbiddenName("abcFuckdef") {
		t.Errorf("IsForbiddenName should match substrings (Java uses contains)")
	}
	if n.IsForbiddenName("正常名字") {
		t.Errorf("IsForbiddenName false positive")
	}
}

func TestLoadNamesRealExport(t *testing.T) {
	const realRoot = "../../wz"
	if _, err := os.Stat(realRoot); err != nil {
		t.Skipf("full wz export not present at %s", realRoot)
	}
	r, err := OpenRoot(realRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	n, err := LoadNames(r)
	if err != nil {
		t.Fatalf("LoadNames: %v", err)
	}
	if len(n.Errors) != 0 {
		t.Errorf("load errors: %v", n.Errors)
	}
	if n.Counts.Items < 1000 || n.Counts.Maps < 100 || n.Counts.Mobs < 100 || n.Counts.NPCs < 100 || n.Counts.Skills < 100 {
		t.Errorf("suspiciously small tables: %+v", n.Counts)
	}
	if got := n.MobName(100100); got != "蜗牛" {
		t.Errorf("MobName(100100) = %q, want 蜗牛", got)
	}
	if _, ok := n.MapName(700000000); !ok {
		t.Errorf("map 700000000 missing (String.wz/Map.img/chinese)")
	}
	if got := n.NPCName(1002000); got == NoNPCName {
		t.Errorf("NPCName(1002000) missing")
	}
	t.Logf("counts: %+v", n.Counts)
}
