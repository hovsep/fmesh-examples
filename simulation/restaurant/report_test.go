// Tests for the report component.
package main

import (
	"testing"

	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportAddsUpTheDay(t *testing.T) {
	report, err := newReport(2, 10)
	require.NoError(t, err)
	fm := alone(t, report)

	// Two minutes pass; in the second one two parties leave and one gives up.
	tick(t, fm, report, 0)
	activate(t, fm, report, map[string][]any{
		"tick": {1},
		"served": {
			party(1, 2, 0, 50).WithMeta("seated", 4),
			party(2, 4, 0, 70).WithMeta("seated", 8),
		},
		"lost": {party(3, 5, 0, 60)},
	})

	assert.Equal(t, component.State{
		"minutes": 2, "parties-served": 2, "guests-served": 6, "parties-lost": 1, "guests-lost": 5,
		"wait-minutes": 12, "busy-table-minutes": 120,
		"average-wait": 6.0,
		"utilisation":  3000.0, // 120 busy table-minutes over 2 tables * 2 minutes: a tiny day, so far above 100 %
		"revenue":      60,
		"lost-revenue": 50,
	}, report.State())
}

func TestEmptyReportHasNoAverages(t *testing.T) {
	report, err := newReport(2, 10)
	require.NoError(t, err)
	fm := alone(t, report)

	tick(t, fm, report, 0)
	assert.Equal(t, 0.0, report.State().Get("average-wait"))
	assert.Equal(t, 0.0, report.State().Get("utilisation"))
	assert.Equal(t, 0, report.State().Get("revenue"))
}
