package main

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func search(t *testing.T, fm *fmesh.FMesh, root *Person, skill string) (Answer, int) {
	t.Helper()
	require.NoError(t, fm.ComponentByName(root.Name).InputByName(portQuery).PutSignals(signal.New(Query{Skill: skill})))
	info, err := fm.Run(context.Background())
	require.NoError(t, err)
	answer, err := fm.ComponentByName(root.Name).OutputByName(portAnswers).Signals().FirstAs[Answer]()
	require.NoError(t, err)
	return answer, info.Cycles.Len() - 1 // the last cycle is the empty one that ends the run
}

// bruteForce walks the tree the ordinary way, one person at a time.
func bruteForce(p *Person, skill string) []string {
	found := []string{}
	if slices.Contains(p.Skills, skill) {
		found = append(found, fmt.Sprintf("%s (%s)", p.Name, p.Title))
	}
	for _, r := range p.Reports {
		found = append(found, bruteForce(r, skill)...)
	}
	return found
}

func TestSearchIsAWave(t *testing.T) {
	company := orgChart(4)
	fm, err := getMesh(company)
	require.NoError(t, err)
	assert.Equal(t, 31, fm.Components().Len())

	for _, skill := range []string{"Go", "Rust", "SQL", "Cobol"} {
		answer, cycles := search(t, fm, company, skill)
		assert.ElementsMatch(t, bruteForce(company, skill), answer.Found, skill)
		assert.Equal(t, 31, answer.Asked)
		assert.Equal(t, 2*4+1, cycles, "down four levels, back up four, plus the root")
	}
}

func TestRebuiltTreeWithUnevenDepth(t *testing.T) {
	company := orgChart(4)
	find(company, "Ivy").Reports = []*Person{{Name: "Zed", Title: "Intern", Skills: []string{"Go"}}}
	fm, err := getMesh(company)
	require.NoError(t, err)

	answer, cycles := search(t, fm, company, "Go")
	assert.Contains(t, answer.Found, "Zed (Intern)")
	assert.ElementsMatch(t, bruteForce(company, "Go"), answer.Found)
	assert.Equal(t, 32, answer.Asked)
	assert.Equal(t, 2*5+1, cycles, "the deepest branch sets the pace")
}
