// Tests for the waiting-list component.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPatience = 3

func TestWaitingListShowsPartiesEveryTickUntilTheyGiveUp(t *testing.T) {
	list, err := newWaitingList(testPatience)
	require.NoError(t, err)
	fm := alone(t, list)

	couple := Party{ID: 1, Size: 2, ArrivedAt: 10, MealDuration: 60}
	activate(t, fm, list, map[string][]any{portParty: {couple}})

	// Within the patience the couple is on the list every minute and nobody is lost.
	for minute := 10; minute <= 10+testPatience; minute++ {
		tick(t, fm, list, minute)
		assert.Equal(t, []Party{couple}, outputs[Party](t, list.OutputByName(portWaiting)), "minute %d", minute)
		assert.Empty(t, outputs[Party](t, list.OutputByName(portLost)), "minute %d", minute)
	}

	// One minute past the patience the couple leaves.
	tick(t, fm, list, 10+testPatience+1)
	assert.Empty(t, outputs[Party](t, list.OutputByName(portWaiting)))
	assert.Equal(t, []Party{couple}, outputs[Party](t, list.OutputByName(portLost)))

	// And it is gone for good.
	tick(t, fm, list, 10+testPatience+2)
	assert.Empty(t, outputs[Party](t, list.OutputByName(portWaiting)))
	assert.Empty(t, outputs[Party](t, list.OutputByName(portLost)))
}

func TestWaitingListKeepsArrivalOrderAndDropsSeatedParties(t *testing.T) {
	list, err := newWaitingList(defaultPatienceMinutes)
	require.NoError(t, err)
	fm := alone(t, list)

	first := Party{ID: 1, Size: 2, ArrivedAt: 0}
	second := Party{ID: 2, Size: 4, ArrivedAt: 0}
	third := Party{ID: 3, Size: 1, ArrivedAt: 1}
	activate(t, fm, list, map[string][]any{portParty: {first, second}})
	activate(t, fm, list, map[string][]any{portParty: {third}})

	tick(t, fm, list, 1)
	assert.Equal(t, []Party{first, second, third}, outputs[Party](t, list.OutputByName(portWaiting)))

	// The host seats the second party: it disappears, the others keep their order.
	activate(t, fm, list, map[string][]any{portSeated: {second}})
	tick(t, fm, list, 2)
	assert.Equal(t, []Party{first, third}, outputs[Party](t, list.OutputByName(portWaiting)))
}
