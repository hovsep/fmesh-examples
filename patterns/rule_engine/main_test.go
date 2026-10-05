package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSampleInbox(t *testing.T) {
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	fm, err := getMesh(cfg)
	require.NoError(t, err)
	emails, err := LoadInbox("")
	require.NoError(t, err)

	actions, err := apply(fm, emails)
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{
		// escalate and storage both ask for notify-phone: it is taken once
		"1": {"flag", "notify-phone"},
		"2": {"forward-to-accountant", "label-vendor", "move-to-finance"},
		"3": {"move-to-later"},
		// #4 is from the CEO but not urgent: "all" needs both
		"5": {"label-vendor"},
		"6": {"notify-phone"},
		"7": {"label-vendor"},
	}, actions)
}

// TestConditionsAreShared pins the Rete idea: a condition used by two rules is
// one component, piped to both.
func TestConditionsAreShared(t *testing.T) {
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	fm, err := getMesh(cfg)
	require.NoError(t, err)

	pipes := fm.ComponentByName("if:invoice").OutputByName(portOut).Pipes()
	assert.Equal(t, 2, pipes.Len())
	assert.Equal(t, 1+7+5+6+1, fm.Components().Len())
}

func TestInvalidConfig(t *testing.T) {
	cfg := &Config{
		Conditions: []Condition{
			{Name: "a", Field: "from", Op: "equals", Value: "x"},
			{Name: "a", Field: "from", Op: "sounds_like", Value: "x"},
		},
		Rules: []Rule{
			{Name: "both", All: []string{"a"}, Any: []string{"a"}, Actions: []string{"flag"}},
			{Name: "typo", All: []string{"b"}},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	for _, want := range []string{
		`condition "a" is defined twice`,
		`unknown op "sounds_like"`,
		`rule "both": set exactly one`,
		`rule "typo" has no actions`,
		`unknown condition "b"`,
	} {
		assert.ErrorContains(t, err, want)
	}
}
