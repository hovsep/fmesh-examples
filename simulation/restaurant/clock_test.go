// Tests for the clock: one nudge, one tick, one minute later each time.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockTicksTheNextMinuteOnEveryNudge(t *testing.T) {
	clock, err := newClock()
	require.NoError(t, err)
	fm := alone(t, clock)

	for minute := range 3 {
		activate(t, fm, clock, map[string][]any{"advance": {true}})
		got, err := clock.OutputByName("tick").Signals().FirstAs[int]()
		require.NoError(t, err)
		assert.Equal(t, minute, got)
	}
}
