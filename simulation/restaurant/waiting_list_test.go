// Tests for the waiting-list component.
package main

import (
	"testing"

	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPatience = 3

func TestWaitingListShowsPartiesEveryTickUntilTheyGiveUp(t *testing.T) {
	list, err := newWaitingList(testPatience)
	require.NoError(t, err)
	fm := alone(t, list)
	waiting := list.OutputByName("waiting")
	lost := list.OutputByName("lost")

	couple := party(1, 2, 10, 60)
	activate(t, fm, list, map[string][]any{"party": {couple}})

	// Within the patience the couple is on the list every minute and nobody is lost.
	for minute := 10; minute <= 10+testPatience; minute++ {
		tick(t, fm, list, minute)
		assert.Equal(t, []*signal.Signal{couple}, waiting.Signals().All(), "minute %d", minute)
		assert.True(t, lost.Signals().IsEmpty(), "minute %d", minute)
	}

	// One minute past the patience the couple leaves.
	tick(t, fm, list, 10+testPatience+1)
	assert.True(t, waiting.Signals().IsEmpty())
	assert.Equal(t, []*signal.Signal{couple}, lost.Signals().All())

	// And it is gone for good.
	tick(t, fm, list, 10+testPatience+2)
	assert.True(t, waiting.Signals().IsEmpty())
	assert.True(t, lost.Signals().IsEmpty())
}

func TestWaitingListKeepsArrivalOrderAndDropsSeatedParties(t *testing.T) {
	list, err := newWaitingList(patienceMinutes)
	require.NoError(t, err)
	fm := alone(t, list)
	waiting := list.OutputByName("waiting")

	first := party(1, 2, 0, 60)
	second := party(2, 4, 0, 60)
	third := party(3, 1, 1, 60)
	activate(t, fm, list, map[string][]any{"party": {first, second}})
	activate(t, fm, list, map[string][]any{"party": {third}})

	tick(t, fm, list, 1)
	assert.Equal(t, []*signal.Signal{first, second, third}, waiting.Signals().All())

	// The host seats the second party: it disappears, the others keep their order.
	activate(t, fm, list, map[string][]any{"seated": {second}})
	tick(t, fm, list, 2)
	assert.Equal(t, []*signal.Signal{first, third}, waiting.Signals().All())
}
