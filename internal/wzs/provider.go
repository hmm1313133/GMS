package wzs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DirEntry is a navigation node inside a wz directory tree: the Go port of
// provider.WzXML.WZDirectoryEntry / WZFileEntry.
//
// Children are read from disk on first access and cached afterwards; the tree
// is immutable once loaded (safe for concurrent use).
type DirEntry struct {
	Name  string
	IsDir bool
	Size  int64 // files only
	path  string
	parent *DirEntry

	once  sync.Once
	subs  []*DirEntry
	files []*DirEntry
	index map[string]*DirEntry
}

// Path is the on-disk path of the entry.
func (e *DirEntry) Path() string { return e.path }

// Subdirectories lists the child directories (Java getSubdirectories).
func (e *DirEntry) Subdirectories() []*DirEntry {
	e.load()
	return e.subs
}

// Files lists the .img files of this directory (Java getFiles).
func (e *DirEntry) Files() []*DirEntry {
	e.load()
	return e.files
}

// Entry looks up a child by name, directory or file (Java getEntry).
func (e *DirEntry) Entry(name string) *DirEntry {
	e.load()
	return e.index[name]
}

func (e *DirEntry) load() {
	e.once.Do(func() {
		entries, err := os.ReadDir(e.path)
		if err != nil {
			return // keep the tree empty; callers see "no such entry"
		}
		for _, de := range entries {
			name := de.Name()
			info, err := de.Info()
			if err != nil {
				continue
			}
			child := &DirEntry{Name: name, path: filepath.Join(e.path, name), parent: e}
			if de.IsDir() {
				// Java WZDirectoryEntry skips directories named "*.img":
				// those are image uploads whose data lives in the sibling .xml.
				if strings.HasSuffix(name, ".img") {
					continue
				}
				child.IsDir = true
				e.subs = append(e.subs, child)
			} else {
				if !strings.HasSuffix(name, ".xml") {
					continue
				}
				child.Name = strings.TrimSuffix(name, ".xml")
				child.Size = info.Size()
				e.files = append(e.files, child)
			}
			if e.index == nil {
				e.index = make(map[string]*DirEntry)
			}
			if _, dup := e.index[child.Name]; !dup {
				e.index[child.Name] = child
			}
		}
	})
}

// Provider serves the images of one exported wz directory: the Go port of
// provider.WzXML.XMLWZFile (which Java opens per wz file, e.g. "String.wz").
type Provider struct {
	dir  string
	name string
	root *DirEntry

	mu    sync.Mutex
	cache map[string]*Node // nil when opened uncached
}

// Open opens one exported wz directory (a directory named "String.wz" holding
// <img>.xml files). Parsed images are memoised: Java callers cache the parsed
// tree themselves (MapleItemInformationProvider keeps Cash.img/Consume.img/...
// in fields), and re-parsing a multi-MB XML per lookup would be fatal here.
func Open(dir string) (*Provider, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("wzs: open %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("wzs: %s is not a directory", dir)
	}
	return &Provider{
		dir:   dir,
		name:  filepath.Base(dir),
		root:  &DirEntry{Name: filepath.Base(dir), IsDir: true, path: dir},
		cache: map[string]*Node{},
	}, nil
}

// OpenUncached is Open without memoisation - byte-for-byte Java behaviour
// (every Data call parses the XML again). Use it for one-shot dumps.
func OpenUncached(dir string) (*Provider, error) {
	p, err := Open(dir)
	if err != nil {
		return nil, err
	}
	p.cache = nil
	return p, nil
}

// Name returns the wz name, e.g. "String.wz".
func (p *Provider) Name() string { return p.name }

// Dir returns the backing directory.
func (p *Provider) Dir() string { return p.dir }

// Root returns the navigation root (Java getRoot).
func (p *Provider) Root() *DirEntry { return p.root }

// Purge drops every memoised image.
func (p *Provider) Purge() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k := range p.cache {
		delete(p.cache, k)
	}
}

// Data loads one image, e.g. Data("Cash.img") or
// Data("Map/Map0/000000000.img"). The path is wz-relative ("/" separated) and
// carries the ".img" suffix, exactly like Java XMLWZFile.getData.
func (p *Provider) Data(path string) (*Node, error) {
	rel, err := safeRelPath(path)
	if err != nil {
		return nil, err
	}
	if p.cache != nil {
		p.mu.Lock()
		if n, ok := p.cache[rel]; ok {
			p.mu.Unlock()
			return n, nil
		}
		p.mu.Unlock()
	}

	full := filepath.Join(p.dir, filepath.FromSlash(rel)+".xml")
	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("wzs: %s: %w", p.name+"/"+rel, err)
	}
	defer f.Close()
	root, err := parseXML(f, filepath.Dir(full))
	if err != nil {
		return nil, fmt.Errorf("wzs: %s: %w", p.name+"/"+rel, err)
	}

	if p.cache != nil {
		p.mu.Lock()
		p.cache[rel] = root
		p.mu.Unlock()
	}
	return root, nil
}

func safeRelPath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("wzs: invalid image path %q", path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("wzs: invalid image path %q", path)
	}
	return clean, nil
}

// Root is the wz container directory (Java MapleDataProviderFactory's
// "net.sf.odinms.wzpath", default "wz"), holding one directory per wz file.
type Root struct {
	dir string

	mu       sync.Mutex
	providers map[string]*Provider
}

// OpenRoot opens the wz container directory.
func OpenRoot(dir string) (*Root, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("wzs: open wz root %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("wzs: wz root %s is not a directory", dir)
	}
	return &Root{dir: dir, providers: map[string]*Provider{}}, nil
}

// Dir returns the container directory.
func (r *Root) Dir() string { return r.dir }

// WZ returns (and memoises) the provider for a wz file; name may be given
// with or without the ".wz" suffix (Java fileInWZPath("String.wz")).
func (r *Root) WZ(name string) (*Provider, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".wz") {
		name += ".wz"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.providers[name]; ok {
		return p, nil
	}
	p, err := Open(filepath.Join(r.dir, name))
	if err != nil {
		return nil, err
	}
	r.providers[name] = p
	return p, nil
}

// Names lists the wz directories available in the container.
func (r *Root) Names() ([]string, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, fmt.Errorf("wzs: list %s: %w", r.dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".wz") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ErrNoWZ is returned by Root.Names on an empty/missing container.
var ErrNoWZ = errors.New("wzs: no wz directory found")

// IsNotExist reports whether err was caused by a missing file/directory.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
