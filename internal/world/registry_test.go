package world

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginRegistryPutTake(t *testing.T) {
	r := NewLoginRegistry()
	r.PutLoginAuth(42, "1.2.3.4:5555", "", 3)

	a, ok := r.TakeLoginAuth(42)
	require.True(t, ok, "ticket must be present after put")
	assert.Equal(t, LoginAuth{IP: "1.2.3.4:5555", TempIP: "", Channel: 3}, a)

	// Java getLoginAuth removes the entry (one-shot).
	_, ok = r.TakeLoginAuth(42)
	assert.False(t, ok, "ticket must be gone after take")

	// absent ids report false without panicking
	_, ok = r.TakeLoginAuth(999)
	assert.False(t, ok)
}

func TestLoginRegistryIPAuth(t *testing.T) {
	r := NewLoginRegistry()
	r.PutLoginAuth(1, "1.2.3.4:5555", "", 1)
	assert.True(t, r.ContainsIPAuth("1.2.3.4:5555"))
	assert.False(t, r.ContainsIPAuth("1.2.3.4:6666"))

	r.RemoveIPAuth("1.2.3.4:5555")
	assert.False(t, r.ContainsIPAuth("1.2.3.4:5555"))

	r.AddIPAuth("5.6.7.8:7777")
	assert.True(t, r.ContainsIPAuth("5.6.7.8:7777"))
}

func TestLoginRegistryConcurrent(t *testing.T) {
	r := NewLoginRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.PutLoginAuth(id, "10.0.0.1:1000", "", id%5+1)
			r.ContainsIPAuth("10.0.0.1:1000")
			r.TakeLoginAuth(id)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 32; i++ {
		_, ok := r.TakeLoginAuth(i)
		assert.False(t, ok, "all tickets must have been taken")
	}
}

func TestFinderRegisterLookup(t *testing.T) {
	f := NewFinder()
	f.Register(7, "Player", 2)

	e, ok := f.Find(7)
	require.True(t, ok)
	assert.Equal(t, FindEntry{ID: 7, Name: "Player", Channel: 2}, e)

	// name lookup is case-insensitive (Java PlayerStorage keys lowercase)
	e, ok = f.FindByName("player")
	require.True(t, ok)
	assert.Equal(t, 7, e.ID)

	f.ForceDeregister(7, "Player")
	_, ok = f.Find(7)
	assert.False(t, ok)
	_, ok = f.FindByName("PLAYER")
	assert.False(t, ok)
}
