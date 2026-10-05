package main

import (
	"maps"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lossOf computes the same network in plain Go, independently of the mesh.
func lossOf(p map[string]float64) float64 {
	loss := 0.0
	for i := range y {
		h1 := math.Tanh(p["w11"]*x1[i] + p["w12"]*x2[i] + p["b1"])
		h2 := math.Tanh(p["w21"]*x1[i] + p["w22"]*x2[i] + p["b2"])
		pred := 1 / (1 + math.Exp(-(p["v1"]*h1 + p["v2"]*h2 + p["c"])))
		loss += (pred - y[i]) * (pred - y[i]) / float64(len(y))
	}
	return loss
}

// TestGradientsMatchFiniteDifferences checks the backward pipes against the
// definition of a derivative: nudge each weight, see how the loss moves.
func TestGradientsMatchFiniteDifferences(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	loss, err := trainStep(fm)
	require.NoError(t, err)
	assert.InDelta(t, lossOf(initial), loss, 1e-12, "forward pass")

	const h = 1e-6
	for name := range initial {
		plus, minus := maps.Clone(initial), maps.Clone(initial)
		plus[name] += h
		minus[name] -= h
		numeric := (lossOf(plus) - lossOf(minus)) / (2 * h)

		got := fm.ComponentByName(name).State().Get("grad").(float64)
		assert.InDelta(t, numeric, got, 1e-8, "d loss / d %s", name)
	}
}

func TestLearnsXOR(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	var loss float64
	for range epochs {
		loss, err = trainStep(fm)
		require.NoError(t, err)
	}
	assert.Less(t, loss, 0.001)

	pred := fm.ComponentByName("loss").State().Get("pred").([]float64)
	for i := range y {
		assert.InDelta(t, y[i], pred[i], 0.1, "case %d", i)
	}
}
