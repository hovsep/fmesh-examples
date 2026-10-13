// Tests for a whole day: determinism and bookkeeping that adds up.
package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runDay does what main does without the printing: builds the restaurant,
// runs the mesh once per minute for the whole day and returns the
// bookkeeper's state.
func runDay(t *testing.T) component.State {
	t.Helper()
	fm, err := buildMesh()
	require.NoError(t, err)
	for minute := range openingMinutes {
		_, err := fm.Run(context.Background())
		require.NoError(t, err, "minute %d", minute)
	}
	return fm.ComponentByName("report").State()
}

func TestADayIsDeterministic(t *testing.T) {
	assert.Equal(t, runDay(t), runDay(t))
}

func TestADayAddsUp(t *testing.T) {
	report := runDay(t)

	served, lost := report.Get("parties-served").(int), report.Get("parties-lost").(int)
	assert.Equal(t, openingMinutes, report.Get("minutes-open"))
	assert.Greater(t, served, 0)
	assert.Greater(t, lost, 0, "the peaks should be busy enough to lose some parties")
	assert.Less(t, report.Get("average-wait"), float64(patienceMinutes), "nobody who was served waited past their patience")
	assert.Greater(t, report.Get("utilisation"), 0.0)
	assert.Less(t, report.Get("utilisation"), 100.0)

	lostShare := float64(lost) / float64(lost+served)
	assert.InDelta(t, 0.125, lostShare, 0.075, "aim for 5-20 %% of parties lost under FIFO with 8 tables")
}
