package step_sim

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// StepSeparator divides a program into steps, so a scenario can be written on
// one line: "intake:food 200kcal; wait 1h; activity:start 8 30m".
const StepSeparator = ";"

// WaitKeyword suspends a program for a stretch of simulated time. It is a step
// rather than a command because it pauses only the program that contains it --
// the simulation, and any other running program, carries on.
const WaitKeyword = "wait"

// Step is one instruction in a program: either a command to run or a pause.
type Step struct {
	Command Command
	Wait    time.Duration
}

// IsWait reports whether the step is a pause rather than a command.
func (s Step) IsWait() bool { return s.Command == "" }

func (s Step) String() string {
	if s.IsWait() {
		return WaitKeyword + " " + FormatSimDuration(s.Wait)
	}
	return string(s.Command)
}

// ParseSteps reads a program from a single line of semicolon-separated steps.
func ParseSteps(line string) ([]Step, error) {
	var steps []Step

	for _, part := range strings.Split(line, StepSeparator) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Fields(part)
		if fields[0] != WaitKeyword {
			steps = append(steps, Step{Command: Command(part)})
			continue
		}

		if len(fields) != 2 {
			return nil, fmt.Errorf("%q needs exactly one duration, e.g. %q", part, "wait 1h")
		}
		duration, err := ParseSimDuration(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid wait %q: %w", fields[1], err)
		}
		if duration < 0 {
			return nil, fmt.Errorf("cannot wait a negative duration (%v)", duration)
		}
		steps = append(steps, Step{Wait: duration})
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps in %q", line)
	}
	return steps, nil
}

// FormatSteps renders a program back to the form it was written in.
func FormatSteps(steps []Step) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, s.String())
	}
	return strings.Join(parts, StepSeparator+" ")
}

// Program is a scenario partway through running.
type Program struct {
	ID    int
	Name  string
	Steps []Step

	// pc is the step to run next.
	pc int

	// resumeAt is the simulated time a wait ends. Zero when not waiting.
	resumeAt time.Duration
	waiting  bool
}

// Done reports whether every step has run.
func (p *Program) Done() bool { return p.pc >= len(p.Steps) }

// Programs runs scenarios: ordered lists of commands with waits between them.
//
// Several run at once, each with its own position, so a background routine and a
// hands-on experiment can overlap. Like Scheduler, it is only ever touched from
// the simulation goroutine.
type Programs struct {
	nextID  int
	running []*Program
	defined map[string][]Step
}

func NewPrograms() *Programs {
	return &Programs{defined: make(map[string][]Step)}
}

// Define stores a program under a name so it can be run repeatedly.
func (p *Programs) Define(name string, steps []Step) {
	p.defined[name] = steps
}

// Defined returns the names of stored programs, sorted.
func (p *Programs) Defined() []string {
	return slices.Sorted(maps.Keys(p.defined))
}

// Steps returns a stored program's steps.
func (p *Programs) Steps(name string) ([]Step, bool) {
	steps, ok := p.defined[name]
	return steps, ok
}

// Start begins running a list of steps, returning the new program.
func (p *Programs) Start(name string, steps []Step) *Program {
	p.nextID++
	program := &Program{ID: p.nextID, Name: name, Steps: steps}
	p.running = append(p.running, program)
	return program
}

// StartNamed runs a stored program.
func (p *Programs) StartNamed(name string) (*Program, error) {
	steps, ok := p.defined[name]
	if !ok {
		return nil, fmt.Errorf("no program named %q (known: %v)", name, p.Defined())
	}
	return p.Start(name, steps), nil
}

// Stop cancels a running program, reporting whether it was running.
func (p *Programs) Stop(id int) bool {
	for i, program := range p.running {
		if program.ID == id {
			p.running = slices.Delete(p.running, i, i+1)
			return true
		}
	}
	return false
}

// StopAll cancels every running program and reports how many there were.
func (p *Programs) StopAll() int {
	count := len(p.running)
	p.running = nil
	return count
}

// Running returns a snapshot of the programs in flight.
func (p *Programs) Running() []Program {
	programs := make([]Program, 0, len(p.running))
	for _, program := range p.running {
		programs = append(programs, *program)
	}
	return programs
}

// Advance runs every program as far as it can go at the current simulated time,
// returning the commands to execute in order.
//
// A program runs consecutive commands in a single call and stops at a wait, so
// "eat; run" happens in one tick while "eat; wait 1h; run" does not.
func (p *Programs) Advance(now time.Duration) []Command {
	var commands []Command
	stillRunning := p.running[:0]

	for _, program := range p.running {
		commands = append(commands, program.advance(now)...)
		if !program.Done() {
			stillRunning = append(stillRunning, program)
		}
	}

	p.running = stillRunning
	return commands
}

func (p *Program) advance(now time.Duration) []Command {
	if p.waiting {
		if now < p.resumeAt {
			return nil
		}
		p.waiting = false
	}

	var commands []Command
	for p.pc < len(p.Steps) {
		step := p.Steps[p.pc]
		p.pc++

		if !step.IsWait() {
			commands = append(commands, step.Command)
			continue
		}

		// Anchor the wait on the requested duration from now. Simulated time
		// advances in ticks, so `now` may already be a little past the instant
		// the previous step ran; measuring from here keeps a long chain of waits
		// from accumulating that slack into a visible drift.
		p.resumeAt = now + step.Wait
		p.waiting = true
		break
	}
	return commands
}

// Describe renders a program for the listing.
func (p Program) Describe(now time.Duration) string {
	name := p.Name
	if name == "" {
		name = "(anonymous)"
	}

	position := fmt.Sprintf("step %d/%d", min(p.pc+1, len(p.Steps)), len(p.Steps))
	if p.waiting {
		position = fmt.Sprintf("waiting %s, then %s", FormatSimDuration(p.resumeAt-now), position)
	}
	return fmt.Sprintf("[%d] %s: %s -- %s", p.ID, name, position, FormatSteps(p.Steps))
}
