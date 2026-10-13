// Tests for the table component, run for every table size through the same constructor.
package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableAnnouncesServesAndFreesUp(t *testing.T) {
	for _, seats := range []int{2, 4, 6} {
		t.Run(fmt.Sprintf("%d seats", seats), func(t *testing.T) {
			const index = 3
			table, err := newTable(index, seats)
			require.NoError(t, err)
			fm := alone(t, table)
			free := table.OutputByName("free")
			served := table.OutputByName("served")

			assert.Equal(t, fmt.Sprintf("table-%d", index), table.Name())
			assert.Equal(t, fmt.Sprintf("table for %d", seats), table.Description())
			assert.Equal(t, float64(seats), table.Meta().ValueOrDefault("seats", 0.0))

			// The first tick: the table says it is free, once.
			tick(t, fm, table, 0)
			assert.Equal(t, []any{index}, free.Signals().AllPayloads())
			assert.Equal(t, float64(seats), free.Signals().First().Meta().ValueOrDefault("seats", 0.0))
			tick(t, fm, table, 1)
			assert.True(t, free.Signals().IsEmpty(), "no need to repeat it")

			// Minute 5: a party that arrived at minute 2 sits down for a 3-minute meal.
			tick(t, fm, table, 5)
			activate(t, fm, table, map[string][]any{"seat": {party(7, seats, 2, 3)}})

			// Minutes 6 and 7 pass quietly.
			for minute := 6; minute <= 7; minute++ {
				tick(t, fm, table, minute)
				assert.True(t, served.Signals().IsEmpty(), "minute %d", minute)
				assert.True(t, free.Signals().IsEmpty(), "minute %d", minute)
			}

			// Minute 8: the meal is over, the party leaves with its seating minute
			// stamped on, and the table is free again.
			tick(t, fm, table, 8)
			assert.Equal(t, []any{seats}, served.Signals().AllPayloads())
			assert.Equal(t, map[string]any{"id": 7.0, "arrived": 2.0, "meal": 3.0, "seated": 5.0},
				served.Signals().First().Meta().All())
			assert.Equal(t, []any{index}, free.Signals().AllPayloads())
		})
	}
}
