package step_sim

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// The timeline is where everything time-based lives: a command scheduled for
// later, a command that repeats, and a multi-step scenario are all one thing —
// an Entry that, given the current simulated time, yields the commands due now
// and reports whether it is finished. Jobs and scenarios used to be two separate
// types with near-identical start/stop/snapshot/advance machinery; unifying them
// removes that duplication.

// Forever marks a repeating entry with no limit on how many times it runs.
const Forever = -1

// StepSeparator divides a scenario into steps on one line:
// "intake:food 200kcal; wait 1h; activity:start 8 30m".
const StepSeparator = ";"

// WaitKeyword suspends a scenario for a stretch of simulated time. It pauses only
// the scenario that contains it — the simulation and any other scenario carry on.
const WaitKeyword = "wait"

// Step is one instruction: either a command to run or a pause.
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

// ParseSteps reads a scenario from a single line of semicolon-separated steps.
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

// FormatSteps renders a scenario back to the form it was written in.
func FormatSteps(steps []Step) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, s.String())
	}
	return strings.Join(parts, StepSeparator+" ")
}

// Entry is one thing on the timeline, partway through running.
//
// A one-shot job is a single command step that starts waiting until its due
// time. A repeating job is the same, re-armed after each run. A scenario is a
// multi-step list that starts running immediately.
type Entry struct {
	ID    int
	Label string // scenario name, or "" for a bare job

	Steps []Step
	pc    int // the step to run next

	waiting  bool
	resumeAt time.Duration // when a wait (or a job's delay) ends

	every     time.Duration // repeat interval; 0 = runs once
	remaining int           // repeats left, or Forever (only with every > 0)
}

// Done reports whether the entry has nothing left to run.
func (e *Entry) Done() bool { return e.pc >= len(e.Steps) }

// Repeats reports whether the entry re-arms after finishing.
func (e *Entry) Repeats() bool { return e.every > 0 }

// isJob reports whether the entry is a bare (possibly repeating) command rather
// than a multi-step scenario. Jobs and scenarios are listed separately.
func (e *Entry) isJob() bool {
	return e.Label == "" && len(e.Steps) == 1 && !e.Steps[0].IsWait()
}

// Command returns a job's command (the first step).
func (e *Entry) Command() Command { return e.Steps[0].Command }

// advance runs the entry as far as it can at time now, returning the commands
// due, and re-arming or finishing when its steps complete.
func (e *Entry) advance(now time.Duration) []Command {
	if e.waiting {
		if now < e.resumeAt {
			return nil
		}
		e.waiting = false
	}

	var commands []Command
	for e.pc < len(e.Steps) {
		step := e.Steps[e.pc]
		e.pc++

		if !step.IsWait() {
			commands = append(commands, step.Command)
			continue
		}

		// Anchor the wait on now rather than the previous step's instant: ticks
		// are discrete, so `now` may already be a little past it, and measuring
		// from here keeps a long chain of waits from accumulating drift.
		e.resumeAt = now + step.Wait
		e.waiting = true
		return commands
	}

	// The step-list finished. A repeating entry re-arms one interval out (and
	// does not replay intervals missed while simulated time jumped ahead); a
	// one-shot is left Done and will be dropped.
	if e.Repeats() {
		if e.remaining != Forever {
			e.remaining--
		}
		if e.remaining != 0 {
			e.pc = 0
			e.waiting = true
			e.resumeAt = now + e.every
		}
	}
	return commands
}

// Describe renders the entry for the jobs / scenarios listings.
func (e *Entry) Describe(now time.Duration) string {
	if e.isJob() {
		when := FormatSimDuration(e.resumeAt - now)
		if !e.Repeats() {
			return fmt.Sprintf("[%d] in %s: %s", e.ID, when, e.Command())
		}
		times := "forever"
		if e.remaining > 0 {
			times = fmt.Sprintf("%d more", e.remaining)
		}
		return fmt.Sprintf("[%d] every %s (next in %s, %s): %s",
			e.ID, FormatSimDuration(e.every), when, times, e.Command())
	}

	name := e.Label
	if name == "" {
		name = "(anonymous)"
	}
	position := fmt.Sprintf("step %d/%d", min(e.pc+1, len(e.Steps)), len(e.Steps))
	if e.waiting {
		position = fmt.Sprintf("waiting %s, then %s", FormatSimDuration(e.resumeAt-now), position)
	}
	return fmt.Sprintf("[%d] %s: %s -- %s", e.ID, name, position, FormatSteps(e.Steps))
}

// Timeline holds everything scheduled or scripted. It is deliberately not
// goroutine-safe: per the Simulation.Run invariant it is only touched from the
// simulation goroutine, both when commands add entries and when the loop advances.
type Timeline struct {
	nextID  int
	entries []*Entry
	defined map[string][]Step
}

func NewTimeline() *Timeline {
	return &Timeline{defined: make(map[string][]Step)}
}

func (t *Timeline) add(e *Entry) *Entry {
	t.nextID++
	e.ID = t.nextID
	t.entries = append(t.entries, e)
	return e
}

// After queues a command to run once, at the given simulated time.
func (t *Timeline) After(due time.Duration, cmd Command) *Entry {
	return t.add(&Entry{Steps: []Step{{Command: cmd}}, waiting: true, resumeAt: due})
}

// Every queues a command to run repeatedly. The first run is one interval out, so
// "every 1d excretion:defecate" does not fire the moment it is typed.
func (t *Timeline) Every(now, interval time.Duration, cmd Command, times int) (*Entry, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("repeat interval must be positive, got %v", interval)
	}
	return t.add(&Entry{
		Steps:     []Step{{Command: cmd}},
		every:     interval,
		remaining: times,
		waiting:   true,
		resumeAt:  now + interval,
	}), nil
}

// Start begins running a scenario immediately.
func (t *Timeline) Start(name string, steps []Step) *Entry {
	return t.add(&Entry{Label: name, Steps: steps})
}

// Define stores a scenario under a name so it can be run repeatedly.
func (t *Timeline) Define(name string, steps []Step) { t.defined[name] = steps }

// Defined returns the names of stored scenarios, sorted.
func (t *Timeline) Defined() []string { return slices.Sorted(maps.Keys(t.defined)) }

// Steps returns a stored scenario's steps.
func (t *Timeline) Steps(name string) ([]Step, bool) {
	steps, ok := t.defined[name]
	return steps, ok
}

// StartNamed runs a stored scenario.
func (t *Timeline) StartNamed(name string) (*Entry, error) {
	steps, ok := t.defined[name]
	if !ok {
		return nil, fmt.Errorf("no scenario named %q (known: %v)", name, t.Defined())
	}
	return t.Start(name, steps), nil
}

// Cancel removes an entry by id, reporting whether it existed.
func (t *Timeline) Cancel(id int) bool {
	for i, e := range t.entries {
		if e.ID == id {
			t.entries = slices.Delete(t.entries, i, i+1)
			return true
		}
	}
	return false
}

// cancelWhere drops every entry matching pred and returns how many.
func (t *Timeline) cancelWhere(pred func(*Entry) bool) int {
	kept := t.entries[:0]
	count := 0
	for _, e := range t.entries {
		if pred(e) {
			count++
		} else {
			kept = append(kept, e)
		}
	}
	t.entries = kept
	return count
}

// CancelAllJobs drops every scheduled job (not scenarios) and returns the count.
func (t *Timeline) CancelAllJobs() int { return t.cancelWhere((*Entry).isJob) }

// StopAllScenarios drops every running scenario (not jobs) and returns the count.
func (t *Timeline) StopAllScenarios() int {
	return t.cancelWhere(func(e *Entry) bool { return !e.isJob() })
}

// Jobs returns a snapshot of the scheduled jobs, soonest first.
func (t *Timeline) Jobs() []Entry {
	jobs := t.snapshot((*Entry).isJob)
	slices.SortFunc(jobs, func(a, b Entry) int { return int(a.resumeAt - b.resumeAt) })
	return jobs
}

// Scenarios returns a snapshot of the running scenarios.
func (t *Timeline) Scenarios() []Entry {
	return t.snapshot(func(e *Entry) bool { return !e.isJob() })
}

func (t *Timeline) snapshot(pred func(*Entry) bool) []Entry {
	var out []Entry
	for _, e := range t.entries {
		if pred(e) {
			out = append(out, *e)
		}
	}
	return out
}

// Advance runs every entry as far as it can at time now, returning the commands
// to execute in order, and dropping entries that have finished.
func (t *Timeline) Advance(now time.Duration) []Command {
	var commands []Command
	kept := t.entries[:0]

	for _, e := range t.entries {
		commands = append(commands, e.advance(now)...)
		if !e.Done() {
			kept = append(kept, e)
		}
	}

	t.entries = kept
	return commands
}
