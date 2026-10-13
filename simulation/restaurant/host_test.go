// Tests for the two hosts. The scene: a 6-top (table 0) and a 2-top (table 1)
// are free, a couple arrived first and a party of five right after.
package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	couple = party(1, 2, 0, 60)
	five   = party(2, 5, 1, 60)

	sixTop = freeTable(0, 6)
	twoTop = freeTable(1, 2)
)

// seatingsOf returns what the host sent to each table output.
func seatingsOf(t *testing.T, host *component.Component, numberOfTables int) map[int][]*signal.Signal {
	t.Helper()
	result := map[int][]*signal.Signal{}
	for i := range numberOfTables {
		if items := slices.Collect(host.OutputByName(fmt.Sprintf("table%d", i)).Signals().All()); len(items) > 0 {
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
	assert.Equal(t, map[int][]*signal.Signal{0: {couple}}, seatingsOf(t, host, 2))
	assert.Equal(t, []*signal.Signal{couple}, slices.Collect(host.OutputByName("seated").Signals().All()))
}

func TestBestFitHostKeepsTheBigTableForTheBigParty(t *testing.T) {
	host, err := newBestFitHost(2)
	require.NoError(t, err)
	fm := alone(t, host)

	activate(t, fm, host, map[string][]any{
		"free":    {sixTop, twoTop},
		"waiting": {couple, five},
	})

	assert.Equal(t, map[int][]*signal.Signal{0: {five}, 1: {couple}}, seatingsOf(t, host, 2))
	assert.ElementsMatch(t, []*signal.Signal{couple, five}, slices.Collect(host.OutputByName("seated").Signals().All()))
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
	assert.Empty(t, seatingsOf(t, host, 2), "no free table fits five")

	// The 6-top frees up again.
	activate(t, fm, host, map[string][]any{"free": {sixTop}, "waiting": {five}})
	assert.Equal(t, map[int][]*signal.Signal{0: {five}}, seatingsOf(t, host, 2))
}

func TestSeatingPolicies(t *testing.T) {
	cases := []struct {
		name    string
		choose  func(waiting []*signal.Signal, free map[int]int) map[int]*signal.Signal
		waiting []*signal.Signal
		free    map[int]int
		want    map[int]*signal.Signal
	}{
		{
			name:    "fifo takes the first fitting table in index order",
			choose:  chooseFirstFit,
			waiting: []*signal.Signal{couple, five},
			free:    map[int]int{0: 6, 1: 2},
			want:    map[int]*signal.Signal{0: couple},
		},
		{
			name:    "best fit fills small tables first and keeps the 6-top for the five",
			choose:  chooseBestFit,
			waiting: []*signal.Signal{couple, five},
			free:    map[int]int{0: 6, 1: 2},
			want:    map[int]*signal.Signal{1: couple, 0: five},
		},
		{
			name:    "best fit prefers the earlier party among equal sizes",
			choose:  chooseBestFit,
			waiting: []*signal.Signal{party(1, 2, 0, 60), party(2, 2, 0, 60)},
			free:    map[int]int{3: 4},
			want:    map[int]*signal.Signal{3: party(1, 2, 0, 60)},
		},
		{
			name:    "nobody fits",
			choose:  chooseBestFit,
			waiting: []*signal.Signal{five},
			free:    map[int]int{1: 2},
			want:    map[int]*signal.Signal{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.choose(c.waiting, c.free))
		})
	}
}
