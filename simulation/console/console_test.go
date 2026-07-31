package console

import (
	"testing"

	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPane_ClearIsTheConsolesOwn(t *testing.T) {
	lines := make(chan command.Line, 4)
	p := NewPane(func() []string { return []string{"air:preset"} }, "", lines)

	p.Append("something the simulation said")
	p.Append("and another thing")

	p.input.SetValue("clear")
	p.submit()

	assert.Empty(t, p.transcript, "the transcript should be empty")
	assert.Empty(t, lines, "and clear must not be sent to the simulation, which has never heard of a transcript")

	t.Run("still forwards everything else", func(t *testing.T) {
		p.input.SetValue("air:preset hyperbaric")
		p.submit()

		require.Len(t, lines, 1)
		assert.Equal(t, command.Line("air:preset hyperbaric"), <-lines)
	})
}
