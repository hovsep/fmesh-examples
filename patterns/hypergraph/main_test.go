package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func run(t *testing.T, raw []string) (*Hypergraph, *Result) {
	t.Helper()
	hg, err := LoadHypergraph("")
	require.NoError(t, err)
	fm, err := getMesh(hg, raw)
	require.NoError(t, err)
	res, err := synthesize(fm, hg, raw)
	require.NoError(t, err)
	return hg, res
}

func TestFullShelf(t *testing.T) {
	hg, res := run(t, fullShelf)

	assert.Len(t, res.Steps, 6)
	assert.Equal(t, []string{"Ca3(PO4)2", "Ca(H2PO4)2", "CaSO4"}, res.Missing)
	assert.Len(t, res.MadeBy, len(hg.Substances)-3)
	assert.Equal(t, []string{"superphosphate plant (no Ca3(PO4)2)"}, res.Stuck)

	// Two reactions make hydrogen in the same step: both are remembered,
	// in file order.
	assert.Equal(t, []string{"steam reforming", "chlor-alkali"}, res.MadeBy["H2"])
	// The ammonium nitrate plant is the end of the longest chain.
	assert.Equal(t, []string{"ammonium nitrate plant"}, res.Steps[5])
}

// TestReactionWaitsForAllReactants pins the hyperedge rule: the Solvay tower
// has some of its reactants after step 1 and fires only when the last one
// (ammonia, step 2) is there.
func TestReactionWaitsForAllReactants(t *testing.T) {
	_, res := run(t, fullShelf)

	assert.Contains(t, res.Waiting[1], "Solvay tower (needs NH3, CO2)")
	assert.Contains(t, res.Waiting[2], "Solvay tower (needs NH3)")
	assert.Contains(t, res.Steps[2], "Solvay tower")
}

func TestNoNaturalGas(t *testing.T) {
	hg, res := run(t, noGas)

	assert.Equal(t, []string{"CH4", "CO", "CH3OH", "HCHO", "Ca3(PO4)2", "Ca(H2PO4)2", "CaSO4"}, res.Missing)
	// Ammonia still gets made, with hydrogen from the chlor-alkali cells.
	assert.Equal(t, []string{"chlor-alkali"}, res.MadeBy["H2"])
	assert.Equal(t, []string{
		"NH3 ← Haber process",
		"  N2 ← air separation",
		"    air from the shelf",
		"  H2 ← chlor-alkali",
		"    NaCl from the shelf",
		"    H2O from the shelf",
	}, Trace(hg, res, "NH3"))
}

func TestOneComponentPerNodeAndHyperedge(t *testing.T) {
	hg, err := LoadHypergraph("")
	require.NoError(t, err)
	fm, err := getMesh(hg, fullShelf)
	require.NoError(t, err)

	assert.Equal(t, len(hg.Substances)+len(hg.Reactions), fm.Components().Len())
	// The Solvay tower is one input port fed by four pipes and one output
	// port feeding two.
	assert.Equal(t, 2, fm.ComponentByName("Solvay tower").OutputByName(portProducts).Pipes().Len())
	assert.Equal(t, 2, fm.ComponentByName("NaCl").OutputByName(portPresent).Pipes().Len()) // chlor-alkali, Solvay
}

func TestInvalidReactionFile(t *testing.T) {
	_, err := ParseHypergraph(`
		A = a
		A = again
		make: A + B -> C
		make: A ->
		nonsense
	`)
	require.Error(t, err)
	for _, want := range []string{
		`substance "A" is defined twice`,
		`reaction "make" is defined twice`,
		`reaction "make" needs reactants and products`,
		`want "formula = name"`,
		`unknown substance "B"`,
		`unknown substance "C"`,
	} {
		assert.ErrorContains(t, err, want)
	}
}
