package world

import "sync"

// FindEntry is one online player in the cross-channel lookup (Java
// World.Find / CharacterIdChannelPair).
type FindEntry struct {
	ID      int
	Name    string
	Channel int
}

// Finder ports Java handling.world.World.Find: the global "who is online on
// which channel" registry. PlayerStorage.registerPlayer/forceDeregister keep
// it current (Java calls World.Find.register / forceDeregister there).
type Finder struct {
	mu     sync.RWMutex
	byID   map[int]FindEntry
	byName map[string]int // lowercase name -> id (Java find)
}

// NewFinder returns an empty finder.
func NewFinder() *Finder {
	return &Finder{
		byID:   map[int]FindEntry{},
		byName: map[string]int{},
	}
}

// Register ports World.Find.register(id, name, channel).
func (f *Finder) Register(id int, name string, channel int) {
	f.mu.Lock()
	f.byID[id] = FindEntry{ID: id, Name: name, Channel: channel}
	f.byName[lower(name)] = id
	f.mu.Unlock()
}

// ForceDeregister ports World.Find.forceDeregister(id, name).
func (f *Finder) ForceDeregister(id int, name string) {
	f.mu.Lock()
	delete(f.byID, id)
	// only drop the name slot when it still points at this id
	if f.byName[lower(name)] == id {
		delete(f.byName, lower(name))
	}
	f.mu.Unlock()
}

// Find ports World.Find.find(cid): the entry or false when offline.
func (f *Finder) Find(id int) (FindEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.byID[id]
	return e, ok
}

// FindByName ports World.Find.find(name) (name lookups are case-insensitive
// in the Java storage - PlayerStorage keys lowercase).
func (f *Finder) FindByName(name string) (FindEntry, bool) {
	f.mu.RLock()
	id, ok := f.byName[lower(name)]
	f.mu.RUnlock()
	if !ok {
		return FindEntry{}, false
	}
	return f.Find(id)
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
