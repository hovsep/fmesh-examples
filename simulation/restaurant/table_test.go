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

			assert.Equal(t, tableName(index), table.Name())
			assert.Equal(t, fmt.Sprintf("table for %d", seats), table.Description())
			assert.Equal(t, float64(seats), table.Meta().ValueOrDefault(metaSeats, 0.0))

			// The first tick: the table says it is free, once.
			tick(t, fm, table, 0)
			assert.Equal(t, []TableFree{{Index: index, Seats: seats}}, outputs[TableFree](t, table.OutputByName(portFree)))
			tick(t, fm, table, 1)
			assert.Empty(t, outputs[TableFree](t, table.OutputByName(portFree)), "no need to repeat it")

			// Minute 5: a party that arrived at minute 2 sits down for a 3-minute meal.
			party := Party{ID: 7, Size: seats, ArrivedAt: 2, MealDuration: 3}
			tick(t, fm, table, 5)
			activate(t, fm, table, map[string][]any{portSeat: {Seating{Party: party, TableIndex: index}}})

			// Minutes 6 and 7 pass quietly.
			for minute := 6; minute <= 7; minute++ {
				tick(t, fm, table, minute)
				assert.Empty(t, outputs[Served](t, table.OutputByName(portServed)), "minute %d", minute)
				assert.Empty(t, outputs[TableFree](t, table.OutputByName(portFree)), "minute %d", minute)
			}

			// Minute 8: the meal is over, the party leaves and the table is free again.
			tick(t, fm, table, 8)
			assert.Equal(t, []Served{{Party: party, Seats: seats, WaitedMinutes: 3, LeftAt: 8}},
				outputs[Served](t, table.OutputByName(portServed)))
			assert.Equal(t, []TableFree{{Index: index, Seats: seats}}, outputs[TableFree](t, table.OutputByName(portFree)))
		})
	}
}
