// wzdump inspects the exported wz tree (I:\GMS\wz) with internal/wzs.
//
// Examples:
//
//	wzdump -list                          # list the wz directories
//	wzdump -tree String.wz -max 20        # first 20 images of one wz
//	wzdump -dump String.wz/Cash.img/5010000
//	wzdump -verify                        # parse every image of every wz (P3.1 DoD)
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GMS/internal/wzs"
)

func main() {
	root := flag.String("wz", "wz", "wz export directory (one directory per *.wz)")
	list := flag.Bool("list", false, "list available wz directories")
	tree := flag.String("tree", "", "list the images of one wz, e.g. String.wz")
	dump := flag.String("dump", "", "dump one image, e.g. String.wz/Cash.img")
	max := flag.Int("max", 30, "cap for -tree / -dump output lines")
	verify := flag.Bool("verify", false, "parse every image of every wz")
	flag.Parse()

	r, err := wzs.OpenRoot(*root)
	if err != nil {
		fail(err)
	}
	switch {
	case *list:
		if err := doList(r); err != nil {
			fail(err)
		}
	case *tree != "":
		if err := doTree(r, *tree, *max); err != nil {
			fail(err)
		}
	case *dump != "":
		if err := doDump(r, *dump, *max); err != nil {
			fail(err)
		}
	case *verify:
		if err := doVerify(r); err != nil {
			fail(err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wzdump:", err)
	os.Exit(1)
}

func doList(r *wzs.Root) error {
	names, err := r.Names()
	if err != nil {
		return err
	}
	fmt.Printf("wz root: %s\n", r.Dir())
	for _, n := range names {
		p, err := r.WZ(n)
		if err != nil {
			return err
		}
		fmt.Printf("  %-14s images=%d\n", n, countImages(p.Root()))
	}
	return nil
}

func doTree(r *wzs.Root, name string, max int) error {
	p, err := r.WZ(name)
	if err != nil {
		return err
	}
	shown := 0
	var walk func(e *wzs.DirEntry, depth int) bool
	walk = func(e *wzs.DirEntry, depth int) bool {
		pad := strings.Repeat("  ", depth)
		for _, f := range e.Files() {
			fmt.Printf("%s%s (%d B)\n", pad, f.Name, f.Size)
			shown++
			if max > 0 && shown >= max {
				return false
			}
		}
		for _, sub := range e.Subdirectories() {
			if !walk(sub, depth+1) {
				return false
			}
		}
		return true
	}
	walk(p.Root(), 1)
	return nil
}

func doDump(r *wzs.Root, spec string, max int) error {
	parts := strings.Split(strings.ReplaceAll(spec, "\\", "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("-dump expects wz/xxx.img[/sub/path], e.g. String.wz/Cash.img/1112223 (got %q)", spec)
	}
	// the image ends at the first ".img" segment; the rest is a node path
	imgEnd := -1
	for i, s := range parts[1:] {
		if strings.HasSuffix(s, ".img") {
			imgEnd = i + 1
			break
		}
	}
	if imgEnd < 0 {
		return fmt.Errorf("-dump: %q has no .img segment", spec)
	}
	p, err := r.WZ(parts[0])
	if err != nil {
		return err
	}
	node, err := p.Data(strings.Join(parts[1:imgEnd+1], "/"))
	if err != nil {
		return err
	}
	if rest := strings.Join(parts[imgEnd+1:], "/"); rest != "" {
		node = node.ChildByPath(rest)
		if node == nil {
			return fmt.Errorf("%s: no such node", spec)
		}
	}
	lines := 0
	printTree(node, 0, max, &lines)
	if max > 0 && lines >= max {
		fmt.Printf("... (capped at %d lines, use -max 0 for the full dump)\n", max)
	}
	return nil
}

func printTree(n *wzs.Node, depth, max int, lines *int) {
	if max > 0 && *lines >= max {
		return
	}
	pad := strings.Repeat("  ", depth)
	switch v := n.Data().(type) {
	case nil:
		fmt.Printf("%s%-8s %s\n", pad, n.Type, n.Name)
	case string:
		fmt.Printf("%s%-8s %s = %q\n", pad, n.Type, n.Name, v)
	case wzs.Point:
		fmt.Printf("%s%-8s %s = (%d,%d)\n", pad, n.Type, n.Name, v.X, v.Y)
	case wzs.Canvas:
		fmt.Printf("%s%-8s %s = %dx%d\n", pad, n.Type, n.Name, v.Width, v.Height)
	default:
		fmt.Printf("%s%-8s %s = %v\n", pad, n.Type, n.Name, v)
	}
	*lines++
	for _, c := range n.Children() {
		printTree(c, depth+1, max, lines)
	}
}

func doVerify(r *wzs.Root) error {
	names, err := r.Names()
	if err != nil {
		return err
	}
	start := time.Now()
	var total, failed int
	for _, name := range names {
		// uncached: the point is to touch every file, not to hold 700 MB of trees
		p, err := wzs.OpenUncached(filepath.Join(r.Dir(), name))
		if err != nil {
			return err
		}
		files, err := imagePaths(p.Root())
		if err != nil {
			return err
		}
		var wzNodes, wzFail int
		for i, rel := range files {
			n, err := p.Data(rel)
			if err != nil {
				wzFail++
				if wzFail <= 5 {
					fmt.Fprintf(os.Stderr, "  FAIL %s/%s: %v\n", name, rel, err)
				}
				continue
			}
			count := 0
			n.Walk(func(*wzs.Node) { count++ })
			wzNodes += count
			if i > 0 && i%2000 == 0 {
				fmt.Printf("  %s: %d/%d ...\n", name, i, len(files))
			}
		}
		p.Purge()
		fmt.Printf("%-14s images=%-6d nodes=%-9d failed=%d\n", name, len(files), wzNodes, wzFail)
		total += len(files)
		failed += wzFail
	}
	fmt.Printf("total images=%d failed=%d in %s\n", total, failed, time.Since(start).Round(time.Millisecond))
	if failed > 0 {
		return fmt.Errorf("%d images failed to parse", failed)
	}
	return nil
}

func countImages(e *wzs.DirEntry) int {
	n := len(e.Files())
	for _, sub := range e.Subdirectories() {
		n += countImages(sub)
	}
	return n
}

func imagePaths(root *wzs.DirEntry) ([]string, error) {
	var out []string
	var walk func(e *wzs.DirEntry, prefix string) error
	walk = func(e *wzs.DirEntry, prefix string) error {
		for _, f := range e.Files() {
			out = append(out, pathJoin(prefix, f.Name))
		}
		for _, sub := range e.Subdirectories() {
			if err := walk(sub, pathJoin(prefix, sub.Name)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no images found")
	}
	return out, nil
}

func pathJoin(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}
