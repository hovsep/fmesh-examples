package main

import (
	"testing"

	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_AppChecks is the wiring check: the mesh builds, the session is set up
// over it, and the body everything else is about is actually in there.
func Test_AppChecks(t *testing.T) {
	t.Parallel() // each test builds its own simulation and shares nothing
	mesh, err := getSimulationMesh()
	require.NoError(t, err)

	sim, err := newSession(mesh)
	require.NoError(t, err)
	require.NotNil(t, sim)

	assert.NotNil(t, human.Find(simMesh(sim)), "no human in the simulation")

	// The world's own commands are registered alongside the session's built-ins,
	// so a front end completing names sees both.
	names := sim.Commands.Names()
	for _, want := range []string{"intake:water", "temp:cold", "step", "every", "help"} {
		assert.Contains(t, names, want)
	}
}
