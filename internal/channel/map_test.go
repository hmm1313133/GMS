package channel

// P4.3b channel tests: Map(id) builds its instance from the wired Map.wz data
// (mapp.NewWithData) and degrades to a bare *mapp.Map when no wz is available.

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"GMS/internal/wzs"
)

func TestChannelMapData(t *testing.T) {
	logs := &strings.Builder{}
	s := New(testConfig(2), slog.New(slog.NewTextHandler(logs, nil)))
	require.NoError(t, s.Start())
	defer s.Stop()
	cs := s.Channel(1)

	// No wz wired: the map is a bare instance, one per id, and nothing panics.
	m := cs.Map(100000000)
	require.NotNil(t, m)
	assert.Same(t, m, cs.Map(100000000), "the channel keeps one instance per map id")
	assert.Nil(t, m.Data(), "no wz -> no map data")
	assert.Equal(t, 1, cs.MapCount())
	assert.Nil(t, cs.lookupMap(12345), "lookupMap never creates")

	// A nil root is the wz-degraded startup: warn once, keep going.
	s.SetWZ(nil)
	assert.Contains(t, logs.String(), "wz root unavailable")
	assert.Nil(t, cs.Map(100000001).Data())

	// Wire the (committed) wz fixture tree: new map instances now carry the
	// loaded data, resolved through the info/link stub.
	root, err := wzs.OpenRoot("../mapp/testdata/wz")
	require.NoError(t, err)
	s.SetWZ(root)
	assert.Contains(t, logs.String(), "map data wired")

	d := cs.Map(900000001).Data()
	require.NotNil(t, d, "Map(id).Data() is non-nil once SetWZ is wired")
	assert.Equal(t, 900000001, d.ID)
	assert.Equal(t, 900000002, d.ImageID, "the stub's info/link is resolved")
	assert.Len(t, d.Portals(), 7)
	assert.Equal(t, 4, d.Footholds().Len())
	require.Len(t, d.Mobs(), 1)

	// The data is cached (the factory hands out one MapData per id) and each
	// channel still owns its own *mapp.Map.
	assert.Same(t, d, cs.Map(900000001).Data())
	other := s.Channel(2).Map(900000001)
	require.NotNil(t, other)
	assert.NotSame(t, cs.Map(900000001), other, "map instances are per channel")
	assert.Same(t, d, other.Data(), "MapData is immutable and shared between channels")

	// A map whose image does not exist degrades to a bare instance, where
	// Java's getMap returns null.
	missing := cs.Map(424242)
	require.NotNil(t, missing)
	assert.Nil(t, missing.Data())
	assert.Same(t, missing, cs.Map(424242), "the degraded map is cached too")
}
