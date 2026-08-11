package main

import (
	"testing"

	"github.com/hovsep/fmesh-examples/simulation/life/organism/human"
	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
	"github.com/stretchr/testify/require"
)

// The catalog claims to describe the real mesh. These tests hold it to that:
// a catalog entry with no matching port would publish nothing, and a port
// missing from the catalog would never reach the UI. Either way the value looks
// present and reads as permanently zero, which is the failure worth catching at
// build time rather than by squinting at a dashboard.

func Test_CatalogMatchesObservableStatePorts(t *testing.T) {
	t.Parallel() // each test builds its own simulation and shares nothing
	fm, err := getSimulationMesh()
	require.NoError(t, err)

	body := human.Find(fm)
	require.NotNil(t, body)

	inner := human.InnerMesh(body)
	require.NotNil(t, inner)

	observableState := inner.ComponentByName("physiology:observable_state")
	require.NotNil(t, observableState)

	for _, port := range telemetry.Ports() {
		require.NotNil(t, observableState.OutputByName(port),
			"catalog declares %q but observable_state has no such output", port)
		require.NotNil(t, body.OutputByName(port),
			"catalog declares %q but the human component has no such output", port)
	}

	for _, source := range telemetry.SourcePorts() {
		require.NotNil(t, observableState.InputByName(source),
			"catalog sources %q but observable_state has no such input", source)
	}
}

func Test_CatalogMatchesAggregatorPaths(t *testing.T) {
	t.Parallel() // each test builds its own simulation and shares nothing
	fm, err := getSimulationMesh()
	require.NoError(t, err)

	body := human.Find(fm)
	require.NotNil(t, body)

	aggregator := fm.ComponentByName("aggregated_state")
	require.NotNil(t, aggregator)

	for _, path := range telemetry.Paths(body.Name()) {
		require.NotNil(t, aggregator.OutputByName(path),
			"catalog publishes %q but the aggregator does not subscribe to it", path)
	}
}

func Test_UISubjectMatchesTheSimulatedHuman(t *testing.T) {
	t.Parallel() // each test builds its own simulation and shares nothing
	fm, err := getSimulationMesh()
	require.NoError(t, err)

	body := human.Find(fm)
	require.NotNil(t, body)

	// The mesh addresses telemetry by the human's real name while the UI assumes
	// DefaultSubject. If they diverge, every panel silently renders zeroes.
	require.Equal(t, telemetry.DefaultSubject, body.Name(),
		"telemetry.DefaultSubject must match the simulated human's component name")
}
