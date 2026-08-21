package main

import (
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMachine(t *testing.T, startAt string) *fmesh.FMesh {
	t.Helper()
	fm, err := getMesh(startAt)
	require.NoError(t, err)
	return fm
}

func payment(amount float64, method string) *signal.Signal {
	s := signal.New("order").WithScalar("amount", amount)
	if method != "" {
		s = s.WithLabel("method", method)
	}
	return s
}

func TestHappyPath(t *testing.T) {
	fm := newMachine(t, "created")
	require.Equal(t, "created", current(fm))

	steps := []struct {
		event string
		sig   *signal.Signal
		want  string
	}{
		{"update", nil, "created"}, // self-loop
		{"pay", payment(orderTotal, "card"), "paid"},
		{"ship", nil, "shipped"},
		{"deliver", nil, "delivered"},
	}
	for _, s := range steps {
		require.NoError(t, fireEvent(fm, s.event, s.sig), "fire %q", s.event)
		assert.Equal(t, s.want, current(fm))
		assert.Equal(t, s.want == "delivered", done(fm))
	}
}

func TestRefusals(t *testing.T) {
	fm := newMachine(t, "created")

	assert.ErrorIs(t, fireEvent(fm, "refund"), errUnknownEvent)
	assert.ErrorIs(t, fireEvent(fm, "ship"), errEventNotAllowed) // exists, but not from "created"
	assert.ErrorIs(t, fireEvent(fm, "pay", payment(5, "card")), errGuardRejected)
	assert.ErrorIs(t, fireEvent(fm, "pay", payment(orderTotal, "")), errGuardRejected) // no method label
	assert.Equal(t, "created", current(fm), "a refused event must not move the machine")

	// A refusal leaves no residue: the next fire does not replay it.
	require.NoError(t, fireEvent(fm, "cancel"))
	assert.Equal(t, "cancelled", current(fm))
	assert.True(t, done(fm))
	assert.ErrorIs(t, fireEvent(fm, "pay", payment(orderTotal, "card")), errEventNotAllowed)
}

func TestCallerSignalIsUntouched(t *testing.T) {
	fm := newMachine(t, "created")
	original := payment(orderTotal, "card")
	require.NoError(t, fireEvent(fm, "pay", original))
	assert.False(t, original.Labels().Has(labelEvent))
}

func TestResume(t *testing.T) {
	fm := newMachine(t, "created")
	require.NoError(t, fireEvent(fm, "pay", payment(orderTotal, "card")))

	// The whole state of the machine is one mesh label; a fresh mesh built
	// with it continues the journey.
	resumed := newMachine(t, current(fm))
	assert.Equal(t, "paid", current(resumed))
	require.NoError(t, fireEvent(resumed, "ship"))
	assert.Equal(t, "shipped", current(resumed))

	_, err := getMesh("limbo")
	assert.ErrorContains(t, err, "limbo")
}

// TestMeshIsTheDiagram pins the promise: the mesh IS the state diagram.
func TestMeshIsTheDiagram(t *testing.T) {
	fm := newMachine(t, "created")

	names := make([]string, 0)
	for name := range fm.Components().All() {
		names = append(names, name)
	}
	assert.ElementsMatch(t, []string{"created", "paid", "shipped", "delivered", "cancelled", "pay?"}, names)

	outputs := func(name string) []string {
		ports := make([]string, 0)
		for portName := range fm.ComponentByName(name).Outputs().All() {
			ports = append(ports, portName)
		}
		return ports
	}
	assert.ElementsMatch(t, []string{"update", "pay", "cancel"}, outputs("created"))
	assert.ElementsMatch(t, []string{"ship", "cancel"}, outputs("paid"))
	assert.ElementsMatch(t, []string{"deliver"}, outputs("shipped"))
	assert.Empty(t, outputs("delivered"))
	assert.Empty(t, outputs("cancelled"))

	// The guarded transition goes through the guard; the others go straight.
	assert.Equal(t, "pay?", fm.ComponentByName("created").OutputByName("pay").Pipes().First().ParentComponent().Name())
	assert.Equal(t, "paid", fm.ComponentByName("pay?").OutputByName(portOut).Pipes().First().ParentComponent().Name())
	assert.Equal(t, "shipped", fm.ComponentByName("paid").OutputByName("ship").Pipes().First().ParentComponent().Name())
}
