package session

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/command"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
)

// Help groups the built-in commands fall under.
const (
	GroupSession    = "Session"
	GroupSimulation = "Simulation"
	GroupScheduling = "Scheduling & scripts"
)

// registerBuiltins adds the commands every session understands: the ones that
// drive the loop itself, and the ones that express routines and scenarios
// rather than one instruction at a time.
//
// They are ordinary registry entries with no special handling anywhere, so
// "exit" with a stray argument still exits, and a front end sees them exactly
// as it sees the ones a simulation adds.
func (s *Session) registerBuiltins() {
	s.Commands.Add(
		command.Command{
			Name: "help", Group: GroupSession,
			Description: "show this help message",
			Run: func(out io.Writer, _ []string) error {
				return s.Commands.WriteHelp(out)
			},
		},
		command.Command{
			Name: "exit", Group: GroupSession,
			Description: "end the simulation",
			Run: func(out io.Writer, _ []string) error {
				fmt.Fprintln(out, "Exiting simulation...")
				return ErrExit
			},
		},
		command.Command{
			Name: "pause", Group: GroupSession,
			Description: "stop advancing (state is kept)",
			Run: func(io.Writer, []string) error {
				s.pause()
				return nil
			},
		},
		command.Command{
			Name: "resume", Group: GroupSession,
			Description: "start advancing again",
			Run: func(io.Writer, []string) error {
				s.resume()
				return nil
			},
		},
		command.Command{
			Name: "step", Group: GroupSimulation,
			Description: "advance by n steps and stay paused, e.g. 'step' or 'step 10'",
			Run:         s.cmdStep,
		},
		command.Command{
			Name: "rate:sim", Group: GroupSimulation,
			Description: "set speed in simulated seconds per real second: 'rate:sim 1', 'rate:sim 60', 'rate:sim max'",
			Run:         s.cmdRateSim,
		},
		command.Command{
			Name: "rate:publish", Group: GroupSimulation,
			Description: "set how often state is published: 'rate:publish 100ms' ('0' = every step)",
			Run:         s.cmdRatePublish,
		},
		command.Command{
			Name: "time", Group: GroupSimulation,
			Description: "show the current simulated time",
			Run: func(out io.Writer, _ []string) error {
				fmt.Fprintf(out, "simulated time: %s (%s)\n",
					simtime.FormatDuration(s.Now()), s.Pacer.Describe())
				return nil
			},
		},
	)

	s.registerSchedulingCommands()
}

// cmdStep advances the simulation by hand. It pauses first, so "step" always
// means "run exactly this many steps, then wait" rather than nudging a loop
// that is already running.
func (s *Session) cmdStep(out io.Writer, args []string) error {
	steps := 1
	if len(args) > 1 {
		return fmt.Errorf("usage: step [count]")
	}
	if len(args) == 1 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid step count %q (use a positive number, e.g. 'step 10')", args[0])
		}
		steps = parsed
	}

	s.pause()
	s.stepsLeft = steps
	fmt.Fprintf(out, "stepping %d step(s)\n", steps)
	return nil
}

// cmdRateSim sets how fast simulated time advances against wall-clock time.
func (s *Session) cmdRateSim(out io.Writer, args []string) error {
	if len(args) != 1 {
		fmt.Fprintln(out, "usage: rate:sim <factor|max>   e.g. 'rate:sim 1', 'rate:sim 60', 'rate:sim max'")
		fmt.Fprintln(out, "current speed:", s.Pacer.Describe())
		return nil
	}

	if args[0] == "max" {
		s.Pacer.SetFactor(simtime.Uncapped)
		fmt.Fprintln(out, "speed set to", s.Pacer.Describe())
		return nil
	}

	factor, err := strconv.ParseFloat(args[0], 64)
	if err != nil || factor <= 0 {
		return fmt.Errorf("invalid speed %q (use a positive number like '1' or '60', or 'max')", args[0])
	}
	s.Pacer.SetFactor(factor)
	fmt.Fprintln(out, "speed set to", s.Pacer.Describe())
	return nil
}

// cmdRatePublish sets how often state reaches the sink. This is purely a
// telemetry concern: it never changes how fast the simulation runs.
func (s *Session) cmdRatePublish(out io.Writer, args []string) error {
	if len(args) != 1 {
		fmt.Fprintln(out, "usage: rate:publish <duration>   e.g. 'rate:publish 100ms', 'rate:publish 0'")
		fmt.Fprintln(out, "current publish interval:", s.Throttle.Interval())
		return nil
	}

	interval, err := time.ParseDuration(args[0])
	if err != nil || interval < 0 {
		return fmt.Errorf("invalid duration %q (use e.g. '100ms', '1s', or '0')", args[0])
	}
	s.Throttle.SetInterval(interval)
	fmt.Fprintln(out, "publish interval set to", interval)
	return nil
}
