// Tests for the two hosts. The scene: a 6-top (table 0) and a 2-top (table 1)
// are free, a couple arrived first and a party of five right after.
package main

import (
	"testing"

	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	couple = Party{ID: 1, Size: 2, ArrivedAt: 0, MealDuration: 60}
	five   = Party{ID: 2, Size: 5, ArrivedAt: 1, MealDuration: 60}

	sixTop = TableFree{Index: 0, Seats: 6}
	twoTop = TableFree{Index: 1, Seats: 2}
)

// seatingsOf returns what the host sent to each table output.
func seatingsOf(t *testing.T, host *component.Component, numberOfTables int) map[int][]Seating {
	t.Helper()
	result := map[int][]Seating{}
	for i := range numberOfTables {
		if items := outputs[Seating](t, host.OutputByName(tablePort(i))); len(items) > 0 {
			result[i] = items
		}
	}
	return result
}

func TestFIFOHostWastesTheBigTableOnTheCouple(t *testing.T) {
	host, err := newFIFOHost(2)
	require.NoError(t, err)
	fm := alone(t, host)

	activate(t, fm, host, map[string][]any{
		"free":    {sixTop, twoTop},
		"waiting": {couple, five},
	})

	// The couple came first and the 6-top is the first table it fits; the five do not fit the 2-top.
	assert.Equal(t, map[int][]Seating{0: {{Party: couple, TableIndex: 0}}}, seatingsOf(t, host, 2))
	assert.Equal(t, []Party{couple}, outputs[Party](t, host.OutputByName("seated")))
}

func TestBestFitHostKeepsTheBigTableForTheBigParty(t *testing.T) {
	host, err := newBestFitHost(2)
	require.NoError(t, err)
	fm := alone(t, host)

	activate(t, fm, host, map[string][]any{
		"free":    {sixTop, twoTop},
		"waiting": {couple, five},
	})

	assert.Equal(t, map[int][]Seating{
		0: {{Party: five, TableIndex: 0}},
		1: {{Party: couple, TableIndex: 1}},
	}, seatingsOf(t, host, 2))
	assert.ElementsMatch(t, []Party{couple, five}, outputs[Party](t, host.OutputByName("seated")))
}

func TestHostRemembersFreeTablesAcrossMinutes(t *testing.T) {
	host, err := newFIFOHost(2)
	require.NoError(t, err)
	fm := alone(t, host)

	// Minute 1: the tables announce themselves, nobody is waiting yet.
	activate(t, fm, host, map[string][]any{"free": {sixTop, twoTop}, "tick": {1}})
	assert.Empty(t, seatingsOf(t, host, 2))

	// Minute 2: the couple shows up and gets a table; the table is then taken.
	activate(t, fm, host, map[string][]any{"waiting": {couple}})
	assert.Len(t, seatingsOf(t, host, 2), 1)

	// Minute 3: the couple is still on the list (it leaves it a moment later), the five arrive.
	activate(t, fm, host, map[string][]any{"waiting": {five}})
	assert.Equal(t, map[int][]Seating{}, seatingsOf(t, host, 2), "no free table fits five")

	// The 6-top frees up again.
	activate(t, fm, host, map[string][]any{"free": {sixTop}, "waiting": {five}})
	assert.Equal(t, map[int][]Seating{0: {{Party: five, TableIndex: 0}}}, seatingsOf(t, host, 2))
}

func TestSeatingPolicies(t *testing.T) {
	cases := []struct {
		name    string
		choose  seatingPolicy
		waiting []Party
		free    freeTables
		want    []Seating
	}{
		{
			name:    "fifo takes the first fitting table in index order",
			choose:  chooseFirstFit,
			waiting: []Party{couple, five},
			free:    freeTables{0: 6, 1: 2},
			want:    []Seating{{Party: couple, TableIndex: 0}},
		},
		{
			name:    "best fit fills small tables first and keeps the 6-top for the five",
			choose:  chooseBestFit,
			waiting: []Party{couple, five},
			free:    freeTables{0: 6, 1: 2},
			want:    []Seating{{Party: couple, TableIndex: 1}, {Party: five, TableIndex: 0}},
		},
		{
			name:    "best fit prefers the earlier party among equal sizes",
			choose:  chooseBestFit,
			waiting: []Party{{ID: 1, Size: 2}, {ID: 2, Size: 2}},
			free:    freeTables{3: 4},
			want:    []Seating{{Party: Party{ID: 1, Size: 2}, TableIndex: 3}},
		},
		{
			name:    "nobody fits",
			choose:  chooseBestFit,
			waiting: []Party{five},
			free:    freeTables{1: 2},
			want:    nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.choose(c.waiting, c.free))
		})
	}
}
