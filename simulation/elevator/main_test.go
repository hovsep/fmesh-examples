package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoEveryoneArrives(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	var view View
	visited := map[string]map[int]bool{"A": {}, "B": {}}
	for s := 1; s <= 40; s++ {
		view, err = step(fm, script[s])
		require.NoError(t, err)
		for _, c := range cabs {
			if view.Cabs[c].Door == doorOpen {
				visited[c][view.Cabs[c].Floor] = true
			}
		}
		if s > 18 && idle(view) {
			break
		}
	}

	require.True(t, idle(view), "the building goes quiet once every call is served")
	// The dispatcher splits the work: A takes the calls on 5 and 2, B those on 3 and 4.
	assert.Equal(t, map[int]bool{5: true, 2: true, 1: true, 3: true}, visited["A"])
	assert.Equal(t, map[int]bool{3: true, 6: true, 4: true, 1: true}, visited["B"])
}

func TestCallWhereTheDoorsAreOpen(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	view, err := step(fm, []string{"1"})
	require.NoError(t, err)
	view, err = step(fm, nil)
	require.NoError(t, err)
	require.Equal(t, doorOpen, view.Cabs["A"].Door)

	// Pressing again while a cab stands there with open doors: no new trip, lamp stays off.
	_, err = step(fm, []string{"1"})
	require.NoError(t, err)
	view, err = step(fm, nil)
	require.NoError(t, err)
	assert.False(t, view.Lamps[1])
	assert.Empty(t, view.Cabs["A"].Stops)
}

func TestBadPress(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)
	for _, p := range []string{"9", "c3", "a0"} {
		_, err := parsePress(fm, p)
		assert.Error(t, err, p)
	}
}
