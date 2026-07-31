// Package session runs a simulation and lets people steer it while it runs.
//
// A session drives a simulation.Engine in a loop, applying commands between
// advances: typed at a prompt, read from a script, or brought due by simulated
// time. It also paces the run against the wall clock and publishes telemetry.
//
// It knows nothing about what is being simulated, or even that meshes exist —
// everything paradigm-specific is behind simulation.Engine.
//
// Concurrency: the engine is advanced and every command is run on the single
// goroutine that called Run (or RunFor). Sources and front ends live on their
// own goroutines and reach the simulation only by sending command lines, so
// nothing needs locking.
package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/schedule"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh-examples/simulation/sink"
)

// ErrExit ends the session. Any command handler may return it, so a script, a
// scheduled command or a front end can all stop the run the same way.
var ErrExit = errors.New("exit")

// lineBuffer is how many command lines may queue up. A small buffer lets a
// front end enqueue without waiting on a slow advance, without letting a
// runaway script build an unbounded backlog.
const lineBuffer = 16

// shutdownGrace is how long Run waits for its source to wind down once the
// session has ended.
const shutdownGrace = time.Second

// Session is a running simulation and everything around it.
type Session struct {
	// Engine is what actually advances.
	Engine simulation.Engine

	// Commands is what the session accepts. Add to it before running, or from
	// a command handler; front ends read it for completion and help.
	Commands *command.Registry

	// Timeline holds what has been scheduled: jobs and running scenarios.
	Timeline *schedule.Timeline

	// Pacer ties simulated time to wall-clock time (uncapped by default).
	Pacer *simtime.Pacer

	// Throttle bounds how often telemetry is published, independently of how
	// fast the simulation runs (0 = publish on every advance).
	Throttle *Throttle

	// AutoPause stops the loop when an advance changes nothing, instead of
	// spinning on a simulation that has settled. Off by default.
	AutoPause bool

	out       io.Writer
	sink      sink.Sink
	source    command.Source
	telemetry func() []string

	lines  chan command.Line
	ctx    context.Context
	cancel context.CancelFunc

	paused    atomic.Bool
	stepsLeft int            // advances the "step" command still owes
	batch     bool           // no front end: a pause can never be undone
	stopAt    *time.Duration // batch deadline, in simulated time
}

// New returns a session ready to run the given engine.
func New(engine simulation.Engine, opts ...Option) *Session {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Session{
		Engine:   engine,
		Commands: command.NewRegistry(),
		Timeline: schedule.New(),
		Pacer:    simtime.New(),
		Throttle: NewThrottle(0),
		out:      stdout{},
		sink:     sink.NewNoop(),
		lines:    make(chan command.Line, lineBuffer),
		ctx:      ctx,
		cancel:   cancel,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.registerBuiltins()
	return s
}

// Configure applies options after construction. It exists for front ends that
// need the session before they can be built -- a console completing the
// session's own command names, say -- and so cannot be passed to New. Call it
// before running.
func (s *Session) Configure(opts ...Option) {
	for _, opt := range opts {
		opt(s)
	}
}

// Now reports the current simulated time.
func (s *Session) Now() time.Duration { return s.Engine.Now() }

// Out returns where the session and its commands write.
func (s *Session) Out() io.Writer { return s.out }

// Sink returns where telemetry goes.
func (s *Session) Sink() sink.Sink { return s.sink }

// Paused reports whether the simulation is currently stopped.
func (s *Session) Paused() bool { return s.paused.Load() }

// Do queues a command line to be run between advances. It is the only safe way
// to reach a running simulation from another goroutine, and it never blocks
// once the session has ended -- the line is simply dropped.
func (s *Session) Do(line command.Line) {
	select {
	case s.lines <- line:
	case <-s.ctx.Done():
	}
}

// Run starts the simulation and its command source and blocks until the session
// ends: someone exits, the source runs out of input, or the engine is done.
//
// A session that has returned from Run is finished and cannot be run again.
func (s *Session) Run() error {
	defer s.cancel()

	var sourceDone chan struct{}
	if s.source != nil {
		sourceDone = make(chan struct{})
		go func() {
			defer close(sourceDone)
			s.source.Run(s.ctx, s.lines)
		}()
	}

	err := s.loop(sourceDone)

	// Tell the source the session is over, and give it a moment to finish: a
	// full-screen front end needs one to put the terminal back. Only a moment,
	// though -- a source blocked reading stdin cannot be interrupted at all, and
	// must not keep the process alive.
	s.cancel()
	if sourceDone != nil {
		select {
		case <-sourceDone:
		case <-time.After(shutdownGrace):
		}
	}

	if closeErr := s.sink.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("closing sink: %w", closeErr)
	}
	return err
}

// RunFor advances the simulation until d of simulated time has passed, with no
// front end and no pacing -- a simulated hour costs milliseconds of wall clock.
// It is what tests and batch runs want, and it leaves the session usable
// afterwards, so a run can be inspected and continued.
//
// It returns early if the simulation pauses, since a paused simulation never
// reaches its deadline and nothing is there to resume it.
func (s *Session) RunFor(d time.Duration) error {
	s.Pacer.SetFactor(simtime.Uncapped)

	deadline := s.Now() + d
	s.batch, s.stopAt = true, &deadline
	defer func() { s.batch, s.stopAt = false, nil }()

	return s.loop(nil)
}

// loop is the simulation itself: apply what is waiting, honour the pace,
// advance, repeat. sourceDone, when non-nil, is closed once no more input can
// arrive.
func (s *Session) loop(sourceDone <-chan struct{}) error {
	// Only a source that has finished ends the session this way; a session
	// without one (a batch run, a programmatic one) runs until it is told to
	// stop or reaches its deadline.
	inputClosed := false

	for {
		// Apply everything already waiting, without blocking.
		for draining := true; draining; {
			select {
			case <-s.ctx.Done():
				return nil
			case line := <-s.lines:
				if stop, err := s.apply(line); stop {
					return err
				}
			case <-sourceDone:
				// No more input can arrive. Keep going until what is already
				// queued has been applied, then stop. Drop the channel: a
				// closed one is always ready and would spin the loop.
				inputClosed, sourceDone = true, nil
			default:
				draining = false
			}
		}

		if inputClosed && len(s.lines) == 0 {
			return nil
		}

		if s.paused.Load() {
			// A pending "step" runs even while paused -- advancing by hand is
			// the whole point of it -- and ignores the pace, since nobody
			// stepping wants to wait out a step budget.
			if s.stepsLeft > 0 {
				s.stepsLeft--
				if stop, err := s.advance(); stop {
					return err
				}
				continue
			}

			// A batch run has nobody to type "resume", and a paused simulation
			// never reaches its deadline: end, rather than hang.
			if s.batch {
				fmt.Fprintln(s.out, "Simulation paused with nothing to resume it, stopping.")
				return nil
			}

			// Wait for something to happen rather than spinning.
			select {
			case <-s.ctx.Done():
				return nil
			case line := <-s.lines:
				if stop, err := s.apply(line); stop {
					return err
				}
			case <-sourceDone:
				inputClosed, sourceDone = true, nil
			}
			continue
		}

		// Hold off until the next advance is due, re-entering the loop rather
		// than sleeping it out, so commands keep being applied while we wait.
		if !s.Pacer.Ready() {
			s.Pacer.Nap()
			continue
		}

		if stop, err := s.advance(); stop {
			return err
		}
	}
}

// advance runs one unit of simulation: whatever simulated time has brought due,
// then one engine advance, then telemetry.
func (s *Session) advance() (stop bool, err error) {
	if s.stopAt != nil && s.Now() >= *s.stopAt {
		return true, nil
	}

	// Scheduled commands run here, between advances, so they reach the
	// simulation exactly the way typed ones do.
	for _, line := range s.Timeline.Advance(s.Now()) {
		if stop, err := s.apply(line); stop {
			return true, err
		}
	}

	before := s.Now()
	result, err := s.Engine.Advance(s.ctx)
	s.Pacer.Advanced(s.Now() - before)
	if err != nil {
		// A failed advance must not end the session: nothing would then drain
		// the command channel and a front end would wait forever. Pause and
		// report instead, so the run can be inspected, adjusted, or exited.
		fmt.Fprintln(s.out, "Advance failed:", err)
		s.pause()
		fmt.Fprintln(s.out, "Type 'resume' to continue or 'exit' to quit.")
		return false, nil
	}

	s.publish()

	if result.Done {
		fmt.Fprintln(s.out, "The simulation has nothing left to do.")
		return true, nil
	}
	if result.Idle && s.AutoPause {
		fmt.Fprintln(s.out, "Nothing is happening.")
		s.pause()
	}
	return false, nil
}

// publish hands the current state to the sink, if there is anything to publish
// and the throttle allows it now. The decision is made per snapshot, never per
// line, so what reaches the sink is always one consistent moment.
func (s *Session) publish() {
	if s.telemetry == nil || !s.Throttle.Allow() {
		return
	}

	for _, line := range s.telemetry() {
		if err := s.sink.Publish(line); err != nil {
			fmt.Fprintln(s.out, "publishing telemetry:", err)
			return
		}
	}
}

// apply runs one command line and reports whether the session should stop.
// Anything the command complains about is reported and the session carries on.
func (s *Session) apply(line command.Line) (stop bool, err error) {
	if err := s.Dispatch(line); err != nil {
		if errors.Is(err, ErrExit) {
			return true, nil
		}
		fmt.Fprintln(s.out, err)
	}
	return false, nil
}

// Dispatch runs one command line on the calling goroutine and returns what it
// reported. Use it from inside a command (a script being loaded, for instance);
// from any other goroutine use Do.
func (s *Session) Dispatch(line command.Line) error {
	name, args := line.Fields()
	if name == "" {
		return nil
	}

	cmd, ok := s.Commands.Lookup(name)
	if !ok {
		if isScenario(line) {
			return s.startScenario(line)
		}
		return fmt.Errorf("unknown command: %s", line)
	}

	// A line of separated steps is a scenario, not a command -- unless the
	// command takes a whole raw line, in which case defining or scheduling one
	// would run it instead.
	if !cmd.RawLine && isScenario(line) {
		return s.startScenario(line)
	}
	return cmd.Run(s.out, args)
}

// isScenario reports whether a line is several steps rather than one command.
func isScenario(line command.Line) bool {
	if !strings.Contains(string(line), schedule.StepSeparator) {
		return false
	}
	steps, err := schedule.ParseSteps(string(line))
	return err == nil && len(steps) > 1
}

func (s *Session) startScenario(line command.Line) error {
	steps, err := schedule.ParseSteps(string(line))
	if err != nil {
		return fmt.Errorf("could not read scenario: %w", err)
	}

	entry := s.Timeline.Start("", steps)
	fmt.Fprintf(s.out, "running scenario [%d]: %s\n", entry.ID, schedule.FormatSteps(steps))
	return nil
}

func (s *Session) pause() {
	if s.paused.Load() {
		return
	}
	s.paused.Store(true)
	fmt.Fprintln(s.out, "Simulation paused")
}

func (s *Session) resume() {
	s.paused.Store(false)
	// Steps still owed are steps nobody needs any more: the loop is running
	// again, and they would otherwise fire at the next pause. Wall-clock time
	// also passed while paused without simulated time passing, so drop the
	// pacer's schedule rather than racing to make it up.
	s.stepsLeft = 0
	s.Pacer.Reset()
	fmt.Fprintln(s.out, "Simulation resumed")
}
