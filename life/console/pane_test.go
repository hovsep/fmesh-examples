package console

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hovsep/fmesh-examples/simulation/command"
)

// typeLine feeds a line and the return key to a pane, as a user would.
func typeLine(p *Pane, line string) tea.Cmd {
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
	return p.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestPane_ForwardsSubmittedLines(t *testing.T) {
	lines := make(chan command.Line, 4)
	p := NewPane(func() []string { return nil }, t.TempDir()+"/history", lines)

	typeLine(p, "intake:water 500ml")

	select {
	case got := <-lines:
		if got != "intake:water 500ml" {
			t.Fatalf("forwarded %q", got)
		}
	default:
		t.Fatal("the command never reached the simulation")
	}
}

func TestPane_ForwardsExitRatherThanQuittingItself(t *testing.T) {
	lines := make(chan command.Line, 4)
	p := NewPane(func() []string { return nil }, t.TempDir()+"/history", lines)

	// "exit" belongs to the simulation: it ends the session, and the dashboard
	// closes because the session ended. Quitting here instead would tear down
	// the front end while the simulation was still running.
	if cmd := typeLine(p, "exit"); cmd != nil {
		t.Fatal("the pane quit on 'exit' instead of forwarding it")
	}

	select {
	case got := <-lines:
		if got != "exit" {
			t.Fatalf("forwarded %q, want exit", got)
		}
	default:
		t.Fatal("'exit' never reached the simulation")
	}
}

func TestPane_SaysSoWhenTheSimulationIsNotListening(t *testing.T) {
	// An unbuffered channel nobody is reading stands in for a wedged simulation.
	// The pane must report that rather than freeze the whole dashboard.
	lines := make(chan command.Line)
	p := NewPane(func() []string { return nil }, t.TempDir()+"/history", lines)

	typeLine(p, "intake:water 500ml")

	if last := p.transcript[len(p.transcript)-1]; last == "" || last[0] != '!' {
		t.Fatalf("last transcript line is %q, want a warning", last)
	}
}
