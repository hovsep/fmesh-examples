// This file holds helpers shared by the component tests: run one component on
// its own, as a one-component mesh, and read its outputs.
package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
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

// activate puts the given payloads on the named inputs and runs the mesh once.
// Outputs have no pipes, so the signals stay on the output ports to be read.
func activate(t *testing.T, fm *fmesh.FMesh, c *component.Component, inputs map[string][]any) {
	t.Helper()
	for name, items := range inputs {
		require.NoError(t, c.InputByName(name).PutPayloads(items...))
	}
	_, err := fm.Run(context.Background())
	require.NoError(t, err)
}

// tick activates the component with just the minute on its "tick" input.
func tick(t *testing.T, fm *fmesh.FMesh, c *component.Component, minute int) {
	t.Helper()
	activate(t, fm, c, map[string][]any{"tick": {minute}})
}

// outputs reads every payload on an output port as T.
func outputs[T any](t *testing.T, p *port.Port) []T {
	t.Helper()
	items, err := payloads[T](p)
	require.NoError(t, err)
	return items
}
