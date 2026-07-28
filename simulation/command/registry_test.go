package command

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func noop(io.Writer, []string) error { return nil }

func TestRegistry_LookupAndNames(t *testing.T) {
	r := NewRegistry()
	r.Add(
		Command{Name: "pause", Run: noop},
		Command{Name: "exit", Run: noop},
		Command{Name: "rate:sim", Run: noop},
	)

	if _, ok := r.Lookup("pause"); !ok {
		t.Fatal("registered command not found")
	}
	if _, ok := r.Lookup("nope"); ok {
		t.Fatal("unregistered command found")
	}

	// Sorted, so completion and listings are stable.
	got := strings.Join(r.Names(), " ")
	if want := "exit pause rate:sim"; got != want {
		t.Fatalf("Names() = %q, want %q", got, want)
	}
}

func TestRegistry_AddReplacesByName(t *testing.T) {
	r := NewRegistry()
	r.Add(Command{Name: "step", Description: "first", Run: noop})
	r.Add(Command{Name: "step", Description: "second", Run: noop})

	c, _ := r.Lookup("step")
	if c.Description != "second" {
		t.Fatalf("description = %q, want the later registration", c.Description)
	}
	if len(r.Names()) != 1 {
		t.Fatalf("names = %v, want one entry", r.Names())
	}
}

func TestRegistry_HelpGroupsAndOrders(t *testing.T) {
	r := NewRegistry()
	r.Add(
		Command{Name: "zoom", Description: "zoom in", Group: "View"},
		Command{Name: "exit", Description: "leave", Group: "Session"},
		Command{Name: "dump", Description: "dump state"}, // ungrouped
		Command{Name: "align", Description: "align it", Group: "View"},
	)

	var out strings.Builder
	if err := r.WriteHelp(&out); err != nil {
		t.Fatal(err)
	}

	want := `Available commands:

Session
  exit   leave

View
  align  align it
  zoom   zoom in

Other
  dump   dump state
`
	if out.String() != want {
		t.Fatalf("help output:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestRegistry_HelpReportsWriteErrors(t *testing.T) {
	r := NewRegistry()
	r.Add(Command{Name: "x", Description: "y"})

	if err := r.WriteHelp(failingWriter{}); err == nil {
		t.Fatal("expected the write error to be reported")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no") }

func TestLine_Fields(t *testing.T) {
	tests := []struct {
		line     Line
		wantName string
		wantArgs int
	}{
		{"", "", 0},
		{"   ", "", 0},
		{"jobs", "jobs", 0},
		{"  rate:sim   60  ", "rate:sim", 1},
		{"every 1d tank:drain x5", "every", 3},
	}

	for _, tt := range tests {
		name, args := tt.line.Fields()
		if name != tt.wantName || len(args) != tt.wantArgs {
			t.Errorf("Line(%q).Fields() = %q, %v; want %q with %d args",
				tt.line, name, args, tt.wantName, tt.wantArgs)
		}
	}
}
