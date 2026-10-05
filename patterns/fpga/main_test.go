package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func flashSource(t *testing.T, src string) *fmesh.FMesh {
	t.Helper()
	d, err := Synthesize(src)
	require.NoError(t, err)
	fm, err := Flash(d)
	require.NoError(t, err)
	return fm
}

func TestAdderAddsEverything(t *testing.T) {
	fm := flashSource(t, adderSource)
	for a := range 16 {
		for b := range 16 {
			got, err := add(fm, a, b)
			require.NoError(t, err)
			assert.Equal(t, a+b, got, "%d + %d", a, b)
		}
	}
}

func TestCounterCountsAndHolds(t *testing.T) {
	fm := flashSource(t, counterSource)
	for tick := range 40 {
		require.NoError(t, Tick(fm, map[string]bool{"en": true}))
		assert.Equal(t, tick%16, word(fm, "q", 4), "tick %d", tick)
	}
	// Enable off: one more edge takes the value computed while it was on, then it holds.
	require.NoError(t, Tick(fm, map[string]bool{"en": false}))
	held := word(fm, "q", 4)
	for range 5 {
		require.NoError(t, Tick(fm, map[string]bool{"en": false}))
		assert.Equal(t, held, word(fm, "q", 4))
	}
}

func TestTruthTables(t *testing.T) {
	d, err := Synthesize("module m(input a, b, output x, y, z); assign x = a ^ b; assign y = ~(a | b); assign z = a & ~b | 0; endmodule")
	require.NoError(t, err)
	assert.Equal(t, "0110", d.Cells[0].TableString())
	assert.Equal(t, "0001", d.Cells[1].TableString())
	assert.Equal(t, "0010", d.Cells[2].TableString())
}

func TestSynthesisErrors(t *testing.T) {
	for src, want := range map[string]string{
		"module m(input a, b, c, d, e, output x); assign x = a & b & c & d & e; endmodule": "split it with an assign",
		"module m(input a, output x); assign x = a & ghost; endmodule":                       "nothing drives",
		"module m(input a, output x); assign x = a; assign x = ~a; endmodule":                "driven twice",
		"module m(input a, output x); assign y = a; endmodule":                               "output x is not driven",
		"module m(input a, output x); assign x = a + 1; endmodule":                           "unexpected",
		"module m(input a, output x); assign x = a;":                                         "endmodule",
	} {
		_, err := Synthesize(src)
		assert.ErrorContains(t, err, want, src)
	}
}

func TestDesignTooBigForTheBoard(t *testing.T) {
	var b strings.Builder
	b.WriteString("module big(input a, output x0);")
	for i := range BoardCells + 1 {
		fmt.Fprintf(&b, " assign x%d = ~a;", i)
	}
	b.WriteString(" endmodule")
	d, err := Synthesize(b.String())
	require.NoError(t, err)
	_, err = Flash(d)
	assert.ErrorContains(t, err, "the board has 16")
}

// A combinational loop has no clock to break it, so its cells wait for each
// other forever. fmesh's livelock detection names them.
func TestCombinationalLoopIsCaught(t *testing.T) {
	fm := flashSource(t, "module ring(input a, output x); assign x = a & y; assign y = ~x; endmodule")
	err := Tick(fm, map[string]bool{"a": true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, fmesh.ErrLivelockDetected), err.Error())
}
