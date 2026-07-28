package session

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/schedule"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
)

// registerSchedulingCommands adds the commands that express routines and
// scenarios rather than one instruction at a time.
//
// Those that take a whole line -- a scenario, a file, another command -- are
// marked RawLine, so writing one does not run it.
func (s *Session) registerSchedulingCommands() {
	s.Commands.Add(
		command.Command{
			Name: "every", Group: GroupScheduling, RawLine: true,
			Description: "run a command repeatedly in simulated time: 'every <duration> <command...>' (add 'x5' to limit the runs)",
			Run:         s.cmdEvery,
		},
		command.Command{
			Name: "after", Group: GroupScheduling, RawLine: true,
			Description: "run a command once, after a stretch of simulated time: 'after <duration> <command...>'",
			Run:         s.cmdAfter,
		},
		command.Command{
			Name: "at", Group: GroupScheduling, RawLine: true,
			Description: "run a command once, at a simulated time since the run began: 'at <sim time> <command...>'",
			Run:         s.cmdAt,
		},
		command.Command{
			Name: "jobs", Group: GroupScheduling,
			Description: "list scheduled commands",
			Run:         s.cmdJobs,
		},
		command.Command{
			Name: "cancel", Group: GroupScheduling,
			Description: "cancel a scheduled command by id, or 'cancel all'",
			Run:         s.cmdCancel,
		},
		command.Command{
			Name: "script", Group: GroupScheduling, RawLine: true,
			Description: "name a scenario: 'script <name> <command>; wait <duration>; <command>; ...'",
			Run:         s.cmdScript,
		},
		command.Command{
			Name: "run", Group: GroupScheduling, RawLine: true,
			Description: "run a named scenario: 'run <name>'",
			Run:         s.cmdRun,
		},
		command.Command{
			Name: "scripts", Group: GroupScheduling,
			Description: "list named scenarios",
			Run:         s.cmdScripts,
		},
		command.Command{
			Name: "scenarios", Group: GroupScheduling,
			Description: "list scenarios currently running",
			Run:         s.cmdScenarios,
		},
		command.Command{
			Name: "stop", Group: GroupScheduling,
			Description: "stop a running scenario by id, or 'stop all'",
			Run:         s.cmdStop,
		},
		command.Command{
			Name: "load", Group: GroupScheduling, RawLine: true,
			Description: "read commands from a file, one per line",
			Run:         s.cmdLoad,
		},
	)
}

// splitDurationAndCommand peels a leading duration off a command line.
func splitDurationAndCommand(args []string, usage string) (duration string, line command.Line, err error) {
	if len(args) < 2 {
		return "", "", fmt.Errorf("usage: %s", usage)
	}
	return args[0], command.Line(strings.Join(args[1:], " ")), nil
}

func (s *Session) cmdEvery(out io.Writer, args []string) error {
	// An optional trailing "x5" limits the number of runs.
	times := schedule.Forever
	if len(args) > 0 {
		if last := args[len(args)-1]; strings.HasPrefix(last, "x") {
			parsed, err := strconv.Atoi(strings.TrimPrefix(last, "x"))
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid repeat count %q (use e.g. 'x5')", last)
			}
			times, args = parsed, args[:len(args)-1]
		}
	}

	interval, line, err := splitDurationAndCommand(args, "every <duration> <command...> [x<count>]")
	if err != nil {
		return err
	}

	d, err := simtime.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", interval, err)
	}

	job, err := s.Timeline.Every(s.Now(), d, line, times)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "scheduled", job.Describe(s.Now()))
	return nil
}

func (s *Session) cmdAfter(out io.Writer, args []string) error {
	delay, line, err := splitDurationAndCommand(args, "after <duration> <command...>")
	if err != nil {
		return err
	}

	d, err := simtime.ParseDuration(delay)
	if err != nil {
		return fmt.Errorf("invalid delay %q: %w", delay, err)
	}

	job := s.Timeline.After(s.Now()+d, line)
	fmt.Fprintln(out, "scheduled", job.Describe(s.Now()))
	return nil
}

func (s *Session) cmdAt(out io.Writer, args []string) error {
	when, line, err := splitDurationAndCommand(args, "at <sim time> <command...>")
	if err != nil {
		return err
	}

	d, err := simtime.ParseDuration(when)
	if err != nil {
		return fmt.Errorf("invalid time %q: %w", when, err)
	}
	if d < s.Now() {
		return fmt.Errorf("simulated time %s has already passed (now %s)",
			simtime.FormatDuration(d), simtime.FormatDuration(s.Now()))
	}

	job := s.Timeline.After(d, line)
	fmt.Fprintln(out, "scheduled", job.Describe(s.Now()))
	return nil
}

func (s *Session) cmdJobs(out io.Writer, _ []string) error {
	jobs := s.Timeline.Jobs()
	if len(jobs) == 0 {
		fmt.Fprintln(out, "nothing scheduled")
		return nil
	}

	now := s.Now()
	fmt.Fprintf(out, "scheduled commands (simulated time now %s):\n", simtime.FormatDuration(now))
	for _, job := range jobs {
		fmt.Fprintln(out, "  "+job.Describe(now))
	}
	return nil
}

func (s *Session) cmdCancel(out io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: cancel <id>   or 'cancel all'")
	}

	if args[0] == "all" {
		fmt.Fprintf(out, "cancelled %d scheduled command(s)\n", s.Timeline.CancelAllJobs())
		return nil
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid job id %q", args[0])
	}
	if !s.Timeline.Cancel(id) {
		return fmt.Errorf("no scheduled command with id %d", id)
	}
	fmt.Fprintf(out, "cancelled job %d\n", id)
	return nil
}

func (s *Session) cmdScript(out io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: script <name> <step>; <step>; ...")
	}

	name := args[0]
	steps, err := schedule.ParseSteps(strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("could not read scenario: %w", err)
	}

	s.Timeline.Define(name, steps)
	fmt.Fprintf(out, "defined %q: %s\n", name, schedule.FormatSteps(steps))
	return nil
}

func (s *Session) cmdRun(out io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: run <name>")
	}

	scenario, err := s.Timeline.StartNamed(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "running", scenario.Describe(s.Now()))
	return nil
}

func (s *Session) cmdScripts(out io.Writer, _ []string) error {
	names := s.Timeline.Defined()
	if len(names) == 0 {
		fmt.Fprintln(out, "no scenarios defined")
		return nil
	}

	fmt.Fprintln(out, "defined scenarios:")
	for _, name := range names {
		steps, _ := s.Timeline.Steps(name)
		fmt.Fprintf(out, "  %s: %s\n", name, schedule.FormatSteps(steps))
	}
	return nil
}

func (s *Session) cmdScenarios(out io.Writer, _ []string) error {
	running := s.Timeline.Scenarios()
	if len(running) == 0 {
		fmt.Fprintln(out, "no scenarios running")
		return nil
	}

	now := s.Now()
	fmt.Fprintf(out, "running scenarios (simulated time now %s):\n", simtime.FormatDuration(now))
	for _, scenario := range running {
		fmt.Fprintln(out, "  "+scenario.Describe(now))
	}
	return nil
}

func (s *Session) cmdStop(out io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: stop <id>   or 'stop all'")
	}

	if args[0] == "all" {
		fmt.Fprintf(out, "stopped %d scenario(s)\n", s.Timeline.StopAllScenarios())
		return nil
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid scenario id %q", args[0])
	}
	if !s.Timeline.Cancel(id) {
		return fmt.Errorf("no scenario running with id %d", id)
	}
	fmt.Fprintf(out, "stopped scenario %d\n", id)
	return nil
}

// cmdLoad reads commands from a file, one per line, so a scenario can live in
// version control rather than in shell history.
//
// The commands run inline, on the session goroutine, exactly as if they had
// been typed -- including one that ends the session, which stops reading the
// file and returns ErrExit rather than being swallowed.
func (s *Session) cmdLoad(out io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: load <file>")
	}

	file, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	var loaded int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := s.Dispatch(command.Line(line)); err != nil {
			// Report what a line complained about and carry on, unless it ended
			// the session.
			if errors.Is(err, ErrExit) {
				return err
			}
			fmt.Fprintf(out, "%s: %v\n", line, err)
		}
		loaded++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	fmt.Fprintf(out, "ran %d command(s) from %s\n", loaded, args[0])
	return nil
}
