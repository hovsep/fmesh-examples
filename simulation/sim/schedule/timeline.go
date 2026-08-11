package schedule

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/command"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
)

// Forever marks a repeating entry with no limit on how many times it runs.
const Forever = -1

// Entry is one thing on the timeline, partway through running.
//
// A one-shot job is a single command step waiting until its due time. A
// repeating job is the same, re-armed after each run. A scenario is a
// multi-step list that starts running immediately.
type Entry struct {
	ID    int
	Label string // scenario name, or "" for a bare job

	Steps []Step
	pc    int // the step to run next

	waiting  bool
	resumeAt time.Duration // when a wait (or a job's delay) ends

	every     time.Duration // repeat interval; 0 = runs once
	remaining int           // repeats left, or Forever
}

// Done reports whether the entry has nothing left to run.
func (e *Entry) Done() bool { return e.pc >= len(e.Steps) }

// Repeats reports whether the entry re-arms after finishing.
func (e *Entry) Repeats() bool { return e.every > 0 }

// DueAt reports the simulated time this entry next needs attention at, and
// whether it is waiting for one at all (a scenario mid-run is due immediately).
func (e *Entry) DueAt() (time.Duration, bool) {
	if !e.waiting {
		return 0, false
	}
	return e.resumeAt, true
}

// isJob reports whether the entry is a bare (possibly repeating) command rather
// than a multi-step scenario. Jobs and scenarios are listed separately.
func (e *Entry) isJob() bool {
	return e.Label == "" && len(e.Steps) == 1 && !e.Steps[0].IsWait()
}

// Command returns a job's command (its only step).
func (e *Entry) Command() command.Line { return e.Steps[0].Command }

// advance runs the entry as far as it can at time now, returning the commands
// due, and re-arming or finishing when its steps run out.
func (e *Entry) advance(now time.Duration) []command.Line {
	if e.waiting {
		if now < e.resumeAt {
			return nil
		}
		e.waiting = false
	}

	var lines []command.Line
	for e.pc < len(e.Steps) {
		step := e.Steps[e.pc]
		e.pc++

		if !step.IsWait() {
			lines = append(lines, step.Command)
			continue
		}

		// Anchor the wait on now rather than on the previous step's instant:
		// time advances in jumps, so now may already be a little past it, and
		// measuring from here keeps a chain of waits from accumulating drift.
		e.resumeAt = now + step.Wait
		e.waiting = true
		return lines
	}

	// The steps ran out. A repeating entry re-arms one interval out (and does
	// not replay intervals missed while time jumped ahead); a one-shot is left
	// Done and will be dropped.
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
	return lines
}

// Describe renders the entry for the jobs and scenarios listings.
func (e *Entry) Describe(now time.Duration) string {
	if e.isJob() {
		when := simtime.FormatDuration(max(e.resumeAt-now, 0))
		if !e.Repeats() {
			return fmt.Sprintf("[%d] in %s: %s", e.ID, when, e.Command())
		}
		times := "forever"
		if e.remaining > 0 {
			times = fmt.Sprintf("%d more", e.remaining)
		}
		return fmt.Sprintf("[%d] every %s (next in %s, %s): %s",
			e.ID, simtime.FormatDuration(e.every), when, times, e.Command())
	}

	name := e.Label
	if name == "" {
		name = "(anonymous)"
	}
	position := fmt.Sprintf("step %d/%d", min(e.pc+1, len(e.Steps)), len(e.Steps))
	if e.waiting {
		position = fmt.Sprintf("waiting %s, then %s", simtime.FormatDuration(max(e.resumeAt-now, 0)), position)
	}
	return fmt.Sprintf("[%d] %s: %s -- %s", e.ID, name, position, FormatSteps(e.Steps))
}

// Timeline holds everything scheduled or scripted.
//
// It is not goroutine-safe: it belongs to the loop that advances it, which is
// also the only thing that adds to it (commands run there too).
type Timeline struct {
	nextID  int
	entries []*Entry
	defined map[string][]Step
}

func New() *Timeline {
	return &Timeline{defined: make(map[string][]Step)}
}

func (t *Timeline) add(e *Entry) *Entry {
	t.nextID++
	e.ID = t.nextID
	t.entries = append(t.entries, e)
	return e
}

// After queues a command to run once, at the given simulated time.
func (t *Timeline) After(due time.Duration, line command.Line) *Entry {
	return t.add(&Entry{Steps: []Step{{Command: line}}, waiting: true, resumeAt: due})
}

// Every queues a command to run repeatedly, times times or Forever. The first
// run is one interval out, so "every 1d tank:drain" does not fire the moment it
// is typed.
func (t *Timeline) Every(now, interval time.Duration, line command.Line, times int) (*Entry, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("repeat interval must be positive, got %v", interval)
	}
	// Anything but a positive count or Forever would otherwise decrement past
	// zero on the first run and repeat without end -- the opposite of what
	// asking for "zero runs" means.
	if times != Forever && times <= 0 {
		return nil, fmt.Errorf("repeat count must be positive or Forever, got %d", times)
	}

	return t.add(&Entry{
		Steps:     []Step{{Command: line}},
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
	slices.SortFunc(jobs, func(a, b Entry) int { return cmp.Compare(a.resumeAt, b.resumeAt) })
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

// NextDue reports the earliest simulated time anything is due at, and whether
// there is anything at all.
//
// A fixed-step simulation ignores this and just calls Advance on every step; an
// event-driven one uses it to jump its clock straight to the next event.
func (t *Timeline) NextDue() (time.Duration, bool) {
	var earliest time.Duration
	found := false

	for _, e := range t.entries {
		due, waiting := e.DueAt()
		if !waiting {
			// Mid-scenario and not waiting: there is work to do right now.
			return 0, true
		}
		if !found || due < earliest {
			earliest, found = due, true
		}
	}
	return earliest, found
}

// Advance runs every entry as far as it can at time now, returning the commands
// to execute in order, and dropping entries that have finished.
func (t *Timeline) Advance(now time.Duration) []command.Line {
	var lines []command.Line
	kept := t.entries[:0]

	for _, e := range t.entries {
		lines = append(lines, e.advance(now)...)
		if !e.Done() {
			kept = append(kept, e)
		}
	}

	t.entries = kept
	return lines
}
