// This file holds helpers shared by the component tests: run one component on
// its own, as a one-component mesh, and build the signals it is fed.
package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/require"
)

// alone wraps a single component in its own mesh so a test can activate it.
func alone(t *testing.T, c *component.Component) *fmesh.FMesh {
	t.Helper()
	fm, err := fmesh.New("test-" + c.Name())
	require.NoError(t, err)
	require.NoError(t, fm.AddComponents(c))
	return fm
}

// activate puts the given signals (or plain payloads) on the named inputs and
// runs the mesh once. Outputs have no pipes, so the signals stay on the output
// ports to be read.
func activate(t *testing.T, fm *fmesh.FMesh, c *component.Component, inputs map[string][]any) {
	t.Helper()
	for name, items := range inputs {
		for _, item := range items {
			if s, ok := item.(*signal.Signal); ok {
				require.NoError(t, c.InputByName(name).PutSignals(s))
			} else {
				require.NoError(t, c.InputByName(name).PutPayloads(item))
			}
		}
	}
	_, err := fm.Run(context.Background())
	require.NoError(t, err)
}

// tick activates the component with just the minute on its "tick" input.
func tick(t *testing.T, fm *fmesh.FMesh, c *component.Component, minute int) {
	t.Helper()
	activate(t, fm, c, map[string][]any{"tick": {minute}})
}

// party builds a party signal the way the arrivals do.
func party(id int, size int, arrived int, meal int) *signal.Signal {
	return signal.New(size).WithMetaMany(map[string]int{"id": id, "arrived": arrived, "meal": meal})
}

// freeTable builds a "free" announcement the way a table does.
func freeTable(index int, seats int) *signal.Signal {
	return signal.New(index).WithMeta("seats", seats)
}
