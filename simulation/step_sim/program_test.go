package step_sim

import (
	"testing"
	"time"
)

func TestParseSteps(t *testing.T) {
	steps, err := ParseSteps("intake:food 200kcal; wait 1h; activity:start 8 30m")
	if err != nil {
		t.Fatal(err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].Command != "intake:food 200kcal" {
		t.Errorf("step 0 = %q", steps[0].Command)
	}
	if !steps[1].IsWait() || steps[1].Wait != time.Hour {
		t.Errorf("step 1 should be a one-hour wait, got %v", steps[1])
	}
	if steps[2].Command != "activity:start 8 30m" {
		t.Errorf("step 2 = %q", steps[2].Command)
	}
}

func TestParseSteps_Rejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty line", ""},
		{"only separators", ";;;"},
		{"wait with no duration", "wait"},
		{"wait with too many arguments", "wait 1h 30m"},
		{"wait with a bad duration", "wait banana"},
		{"negative wait", "wait -1h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseSteps(tt.in); err == nil {
				t.Fatalf("ParseSteps(%q) accepted an invalid program", tt.in)
			}
		})
	}
}

func TestParseStepsRoundTrips(t *testing.T) {
	line := "intake:food 200kcal; wait 1h; activity:start 8 30m"
	steps, err := ParseSteps(line)
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatSteps(steps); got != line {
		t.Errorf("FormatSteps round-trip = %q, want %q", got, line)
	}
}

func TestPrograms_RunsConsecutiveCommandsAtOnce(t *testing.T) {
	p := NewPrograms()
	steps, err := ParseSteps("first; second; third")
	if err != nil {
		t.Fatal(err)
	}
	p.Start("", steps)

	// Nothing separates these steps, so they should all run in one pass rather
	// than dribbling out one per tick.
	commands := p.Advance(0)
	if len(commands) != 3 {
		t.Fatalf("expected all 3 commands at once, got %v", commands)
	}
	if len(p.Running()) != 0 {
		t.Fatal("a finished program is still listed as running")
	}
}

func TestPrograms_WaitSuspendsUntilSimTimePasses(t *testing.T) {
	p := NewPrograms()
	steps, err := ParseSteps("intake:food 200kcal; wait 1h; activity:start 8 30m")
	if err != nil {
		t.Fatal(err)
	}
	p.Start("breakfast", steps)

	commands := p.Advance(0)
	if len(commands) != 1 || commands[0] != "intake:food 200kcal" {
		t.Fatalf("expected only the pre-wait command, got %v", commands)
	}

	if commands := p.Advance(59 * time.Minute); len(commands) != 0 {
		t.Fatalf("program resumed before its wait elapsed: %v", commands)
	}

	commands = p.Advance(time.Hour)
	if len(commands) != 1 || commands[0] != "activity:start 8 30m" {
		t.Fatalf("expected the post-wait command, got %v", commands)
	}
	if len(p.Running()) != 0 {
		t.Fatal("the program should be finished")
	}
}

func TestPrograms_RunConcurrently(t *testing.T) {
	p := NewPrograms()

	slow, err := ParseSteps("slow-start; wait 2h; slow-end")
	if err != nil {
		t.Fatal(err)
	}
	fast, err := ParseSteps("fast-start; wait 30m; fast-end")
	if err != nil {
		t.Fatal(err)
	}
	p.Start("slow", slow)
	p.Start("fast", fast)

	if commands := p.Advance(0); len(commands) != 2 {
		t.Fatalf("both programs should have started, got %v", commands)
	}

	// One program's wait must not hold up the other's.
	commands := p.Advance(30 * time.Minute)
	if len(commands) != 1 || commands[0] != "fast-end" {
		t.Fatalf("expected only the fast program to resume, got %v", commands)
	}
	if len(p.Running()) != 1 {
		t.Fatal("the slow program should still be waiting")
	}

	commands = p.Advance(2 * time.Hour)
	if len(commands) != 1 || commands[0] != "slow-end" {
		t.Fatalf("expected the slow program to resume, got %v", commands)
	}
}

func TestPrograms_NamedDefinitions(t *testing.T) {
	p := NewPrograms()
	steps, err := ParseSteps("intake:water 250ml; wait 15m; intake:water 250ml")
	if err != nil {
		t.Fatal(err)
	}
	p.Define("hydrate", steps)

	if _, err := p.StartNamed("nonexistent"); err == nil {
		t.Fatal("StartNamed accepted an unknown name")
	}

	program, err := p.StartNamed("hydrate")
	if err != nil {
		t.Fatal(err)
	}
	if program.Name != "hydrate" {
		t.Errorf("program name = %q", program.Name)
	}

	// A definition is reusable, so running it must not consume it.
	if _, err := p.StartNamed("hydrate"); err != nil {
		t.Fatalf("a named program should be runnable more than once: %v", err)
	}
	if len(p.Running()) != 2 {
		t.Fatalf("expected 2 running instances, got %d", len(p.Running()))
	}
}

func TestPrograms_Stop(t *testing.T) {
	p := NewPrograms()
	steps, err := ParseSteps("start; wait 1h; end")
	if err != nil {
		t.Fatal(err)
	}
	program := p.Start("", steps)
	p.Advance(0)

	if !p.Stop(program.ID) {
		t.Fatal("Stop reported the program was not running")
	}
	if commands := p.Advance(2 * time.Hour); len(commands) != 0 {
		t.Fatalf("a stopped program still ran: %v", commands)
	}
}

func TestIsProgramLine(t *testing.T) {
	tests := []struct {
		in   Command
		want bool
	}{
		{"intake:water 500ml", false},
		{"intake:food 200kcal; wait 1h; activity:start 8", true},
		// Defining a program contains separators but must not run it.
		{"script breakfast intake:food 400kcal; wait 30m; activity:start 3", false},
	}

	for _, tt := range tests {
		if got := isProgramLine(tt.in); got != tt.want {
			t.Errorf("isProgramLine(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
