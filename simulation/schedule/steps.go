// Package schedule is a timeline of commands due at simulated times.
//
// A command scheduled for later, a command that repeats, and a multi-step
// scenario are all one thing here: an entry that, given the current simulated
// time, yields the commands due now and reports whether it is finished.
package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
)

// StepSeparator divides a scenario into steps on one line:
// "valve:open; wait 1h; valve:close".
const StepSeparator = ";"

// WaitKeyword suspends a scenario for a stretch of simulated time. It pauses
// only the scenario containing it — the simulation and any other scenario carry
// on.
const WaitKeyword = "wait"

// Step is one instruction in a scenario: either a command to run or a pause.
type Step struct {
	Command command.Line
	Wait    time.Duration
}

// IsWait reports whether the step is a pause rather than a command.
func (s Step) IsWait() bool { return s.Command == "" }

func (s Step) String() string {
	if s.IsWait() {
		return WaitKeyword + " " + simtime.FormatDuration(s.Wait)
	}
	return string(s.Command)
}

// ParseSteps reads a scenario from a single line of separated steps.
func ParseSteps(line string) ([]Step, error) {
	var steps []Step

	for part := range strings.SplitSeq(line, StepSeparator) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Fields(part)
		if fields[0] != WaitKeyword {
			steps = append(steps, Step{Command: command.Line(part)})
			continue
		}

		if len(fields) != 2 {
			return nil, fmt.Errorf("%q needs exactly one duration, e.g. %q", part, "wait 1h")
		}
		duration, err := simtime.ParseDuration(fields[1])
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
