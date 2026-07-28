package session

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/sink"
)

// stepDuration is what one advance of the fake engine is worth.
const stepDuration = 10 * time.Millisecond

// fakeEngine is a simulation with nothing in it: every advance moves the clock
// on and counts itself. Driving the session with one keeps these tests about
// the session -- the loop, the commands, the deadlines -- and makes them
// instant and deterministic.
//
// It is advanced only from the session goroutine, but tests read its counters
// from theirs while the loop runs, so the counters are mutex-guarded.
type fakeEngine struct {
	mu    sync.Mutex
	steps int
	now   time.Duration

	idle bool  // report every advance as changing nothing
	done bool  // report the simulation as finished
	fail error // fail every advance
}

func (e *fakeEngine) Advance() (simulation.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.fail != nil {
		return simulation.Result{}, e.fail
	}
	e.steps++
	e.now += stepDuration
	return simulation.Result{Idle: e.idle, Done: e.done}, nil
}

func (e *fakeEngine) Now() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *fakeEngine) Steps() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.steps
}

// newTestSession returns a session writing into a buffer, plus the engine and a
// func returning everything written so far.
func newTestSession(t *testing.T, opts ...Option) (*Session, *fakeEngine, func() string) {
	t.Helper()

	engine := &fakeEngine{}
	out := &syncBuffer{}
	s := New(engine, append([]Option{WithOut(out)}, opts...)...)
	return s, engine, out.String
}

// syncBuffer is a strings.Builder that tests can read while the session writes.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *syncBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *syncBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// lineSource feeds prepared lines and then behaves as told: either returning
// (input exhausted) or waiting to be cancelled.
type lineSource struct {
	lines  []command.Line
	linger bool
	done   chan struct{} // closed when Run returns
}

func newLineSource(linger bool, lines ...command.Line) *lineSource {
	return &lineSource{lines: lines, linger: linger, done: make(chan struct{})}
}

func (s *lineSource) Run(ctx context.Context, out chan<- command.Line) {
	defer close(s.done)

	for _, line := range s.lines {
		select {
		case out <- line:
		case <-ctx.Done():
			return
		}
	}
	if s.linger {
		<-ctx.Done()
	}
}

func TestSession_RunForStopsAtTheDeadline(t *testing.T) {
	s, engine, _ := newTestSession(t)

	if err := s.RunFor(10 * stepDuration); err != nil {
		t.Fatal(err)
	}

	// Not a step early, and not a run that never ends.
	if got := engine.Steps(); got != 10 {
		t.Fatalf("advanced %d steps, want 10", got)
	}
	if got := s.Now(); got != 10*stepDuration {
		t.Fatalf("simulated time is %v, want %v", got, 10*stepDuration)
	}
}

func TestSession_RunForIsResumable(t *testing.T) {
	s, engine, _ := newTestSession(t)

	for range 3 {
		if err := s.RunFor(5 * stepDuration); err != nil {
			t.Fatal(err)
		}
	}

	// Each run picks up where the last left off, so a test can advance, assert,
	// and advance again.
	if got := engine.Steps(); got != 15 {
		t.Fatalf("advanced %d steps over three runs, want 15", got)
	}
}

func TestSession_RunForSurvivesCancelAll(t *testing.T) {
	s, engine, _ := newTestSession(t)

	// A batch deadline is not a job on the timeline: a simulation that cancels
	// its own scheduled commands must not thereby cancel what stops it.
	s.Do("every 1s nothing")
	s.Do("cancel all")

	if err := s.RunFor(5 * stepDuration); err != nil {
		t.Fatal(err)
	}

	if got := engine.Steps(); got != 5 {
		t.Fatalf("advanced %d steps, want 5", got)
	}
	if jobs := s.Timeline.Jobs(); len(jobs) != 0 {
		t.Fatalf("timeline still holds %v", jobs)
	}
}

func TestSession_RunForStopsWhenThereIsNothingToResumeIt(t *testing.T) {
	s, engine, output := newTestSession(t)

	// A failing advance pauses the loop, and a paused loop never reaches its
	// deadline. With no front end to type "resume", the run has to end.
	engine.fail = errors.New("mesh exploded")

	if err := s.RunFor(time.Hour); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output(), "mesh exploded") {
		t.Fatalf("the failure was not reported:\n%s", output())
	}
	if !strings.Contains(output(), "nothing to resume it") {
		t.Fatalf("the run did not explain why it stopped:\n%s", output())
	}
}

func TestSession_StopsWhenTheEngineIsDone(t *testing.T) {
	s, _, output := newTestSession(t)
	s.Engine.(*fakeEngine).done = true

	if err := s.RunFor(time.Hour); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output(), "nothing left to do") {
		t.Fatalf("a finished engine should end the run:\n%s", output())
	}
}

func TestSession_AutoPauseStopsAnIdleSimulation(t *testing.T) {
	s, engine, output := newTestSession(t, WithAutoPause(true))
	engine.idle = true

	if err := s.RunFor(time.Hour); err != nil {
		t.Fatal(err)
	}

	// One advance was enough to learn nothing is happening.
	if got := engine.Steps(); got != 1 {
		t.Fatalf("advanced %d steps on an idle simulation, want 1", got)
	}
	if !strings.Contains(output(), "Nothing is happening") {
		t.Fatalf("auto-pause was not reported:\n%s", output())
	}
}

func TestSession_StepAdvancesExactlyNAndStaysPaused(t *testing.T) {
	s, engine, _ := newTestSession(t)

	if err := s.Dispatch("step 3"); err != nil {
		t.Fatal(err)
	}
	if !s.Paused() {
		t.Fatal("stepping should pause the simulation")
	}

	// The deadline is an hour away, so what stops this run is the stepping
	// running out: exactly three advances, and then a paused simulation with
	// nothing to resume it.
	if err := s.RunFor(time.Hour); err != nil {
		t.Fatal(err)
	}

	if got := engine.Steps(); got != 3 {
		t.Fatalf("advanced %d steps, want exactly the 3 asked for", got)
	}
	if !s.Paused() {
		t.Fatal("the session should still be paused after stepping")
	}
}

func TestSession_StepPausesARunningSimulation(t *testing.T) {
	source := newLineSource(true, "step 2")
	s, _, _ := newTestSession(t, WithSource(source))

	done := make(chan error, 1)
	go func() { done <- s.Run() }()

	// The simulation is running flat out when the command arrives; it must
	// stop, whatever it was in the middle of.
	waitFor(t, s.Paused, "the simulation to be paused by stepping")

	s.Do("exit")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSession_ResumeCancelsPendingSteps(t *testing.T) {
	s, engine, _ := newTestSession(t)

	// Steps owed to a paused simulation are meaningless once it is running
	// again; they must not fire at the next pause.
	if err := s.Dispatch("step 100"); err != nil {
		t.Fatal(err)
	}
	if err := s.Dispatch("resume"); err != nil {
		t.Fatal(err)
	}
	if err := s.RunFor(2 * stepDuration); err != nil {
		t.Fatal(err)
	}

	if got := engine.Steps(); got != 2 {
		t.Fatalf("advanced %d steps, want the 2 the deadline allowed", got)
	}
}

func TestSession_ExitTakesArgumentsAndStillExits(t *testing.T) {
	// "exit" used to be matched as a whole line, so any stray argument turned it
	// into a silent no-op.
	for _, line := range []command.Line{"exit", "exit now", "exit  "} {
		s, _, _ := newTestSession(t)
		source := newLineSource(true, line)
		s.source = source

		done := make(chan error, 1)
		go func() { done <- s.Run() }()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%q: %v", line, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q did not end the session", line)
		}
	}
}

func TestSession_EndsWhenTheSourceRunsOut(t *testing.T) {
	// A piped script ends the session when it is exhausted -- but everything it
	// queued must run first, or piping a setup script would drop half of it.
	source := newLineSource(false, "mark a", "mark b", "mark c")
	s, _, _ := newTestSession(t, WithSource(source))

	var marked []string
	s.Commands.Add(command.Command{
		Name: "mark",
		Run: func(_ io.Writer, args []string) error {
			marked = append(marked, args[0])
			return nil
		},
	})

	done := make(chan error, 1)
	go func() { done <- s.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the session outlived its input")
	}

	if got := strings.Join(marked, ""); got != "abc" {
		t.Fatalf("ran %q, want every queued command (abc)", got)
	}
}

func TestSession_ShutsDownItsSource(t *testing.T) {
	// The session ending must reach the front end, or it goes on accepting
	// commands for a simulation that is no longer there -- and, if it owns the
	// terminal, never puts it back.
	source := newLineSource(true, "exit")
	s, _, _ := newTestSession(t, WithSource(source))

	if err := s.Run(); err != nil {
		t.Fatal(err)
	}

	// Run waits for it, so by the time it returns the front end has had its
	// chance to wind down.
	select {
	case <-source.done:
	default:
		t.Fatal("Run returned before its source had finished")
	}
}

func TestSession_DoesNotWaitForeverOnAnUninterruptibleSource(t *testing.T) {
	// A source blocked reading a terminal cannot be interrupted. The session
	// must give up on it rather than keep the process alive.
	source := newBlockedSource()
	s, _, _ := newTestSession(t, WithSource(source))

	s.Do("exit")

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Run()
	}()

	select {
	case <-done:
	case <-time.After(shutdownGrace + 2*time.Second):
		t.Fatal("Run hung waiting for a source that will never return")
	}
	close(source.release)
}

// blockedSource never returns until released, standing in for a source blocked
// on a read that cannot be cancelled.
type blockedSource struct{ release chan struct{} }

func newBlockedSource() *blockedSource {
	return &blockedSource{release: make(chan struct{})}
}

func (s *blockedSource) Run(context.Context, chan<- command.Line) { <-s.release }

func TestSession_ClosesItsSink(t *testing.T) {
	target := sink.NewChannel(4)
	source := newLineSource(false)
	s, _, _ := newTestSession(t, WithSource(source), WithSink(target),
		WithTelemetry(func() []string { return []string{"x 1"} }))

	if err := s.Run(); err != nil {
		t.Fatal(err)
	}

	// A closed channel sink ends its consumer's range; leaving it open would
	// hang whatever was draining it. Drain whatever was published first.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range target.Lines() {
		}
	}()

	select {
	case <-drained:
	case <-time.After(2 * time.Second):
		t.Fatal("the sink was left open after the session ended")
	}
}

func TestSession_PublishesTelemetry(t *testing.T) {
	target := sink.NewChannel(64)
	s, _, _ := newTestSession(t, WithSink(target),
		WithTelemetry(func() []string { return []string{"level 1", "level 2"} }))

	if err := s.RunFor(3 * stepDuration); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}

	var lines []string
	for line := range target.Lines() {
		lines = append(lines, line)
	}
	// Two lines per advance, unthrottled.
	if len(lines) != 6 {
		t.Fatalf("published %d lines, want 6: %v", len(lines), lines)
	}
}

func TestSession_ThrottleBoundsPublishing(t *testing.T) {
	target := sink.NewChannel(1024)
	s, _, _ := newTestSession(t, WithSink(target),
		WithTelemetry(func() []string { return []string{"x 1"} }))

	// Far longer than the run will take in wall-clock time, so only the first
	// snapshot gets through.
	s.Throttle.SetInterval(time.Hour)

	if err := s.RunFor(100 * stepDuration); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}

	var published int
	for range target.Lines() {
		published++
	}
	if published != 1 {
		t.Fatalf("published %d snapshots through an hour-long throttle, want 1", published)
	}
}

func TestSession_UnknownCommandIsReportedAndSurvived(t *testing.T) {
	s, engine, output := newTestSession(t)

	s.Do("nonsense 1 2 3")
	if err := s.RunFor(2 * stepDuration); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output(), "unknown command: nonsense 1 2 3") {
		t.Fatalf("the mistake was not reported:\n%s", output())
	}
	if got := engine.Steps(); got != 2 {
		t.Fatalf("a typo cost %d of 2 steps", got)
	}
}

func TestSession_ScheduledCommandsRunBetweenAdvances(t *testing.T) {
	s, _, output := newTestSession(t)

	var fired []time.Duration
	s.Commands.Add(command.Command{
		Name: "mark",
		Run: func(io.Writer, []string) error {
			fired = append(fired, s.Now())
			return nil
		},
	})

	s.Do("after 30ms mark")
	s.Do("every 20ms mark x2")

	if err := s.RunFor(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// The one-shot at 30ms, the repeat at 20ms and 40ms.
	want := []time.Duration{20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond}
	if len(fired) != len(want) {
		t.Fatalf("fired at %v, want %v (output:\n%s)", fired, want, output())
	}
	for i, w := range want {
		if fired[i] != w {
			t.Fatalf("fired at %v, want %v", fired, want)
		}
	}
}

func TestSession_RawLineCommandsAreNotMistakenForScenarios(t *testing.T) {
	s, _, output := newTestSession(t)

	var ran []string
	s.Commands.Add(command.Command{
		Name: "mark",
		Run: func(_ io.Writer, args []string) error {
			ran = append(ran, strings.Join(args, " "))
			return nil
		},
	})

	// The separator belongs to the scheduled command, not to the "after" line:
	// scheduling a two-step scenario must not run both steps immediately.
	if err := s.Dispatch("after 1h mark one; mark two"); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatalf("scheduling ran the command immediately: %v (output:\n%s)", ran, output())
	}
	if jobs := s.Timeline.Jobs(); len(jobs) != 1 {
		t.Fatalf("expected one scheduled job, got %v", jobs)
	}

	// A plain line of steps is still a scenario.
	if err := s.Dispatch("mark one; mark two"); err != nil {
		t.Fatal(err)
	}
	if err := s.RunFor(stepDuration); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 {
		t.Fatalf("the scenario ran %v, want both steps", ran)
	}
}

func TestSession_LoadRunsAFileAndItsExitEndsTheRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.sim")
	script := "# a comment\n\nmark before\nexit\nmark after\n"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	source := newLineSource(true, command.Line("load "+path))
	s, _, output := newTestSession(t, WithSource(source))

	var marked []string
	s.Commands.Add(command.Command{
		Name: "mark",
		Run: func(_ io.Writer, args []string) error {
			marked = append(marked, args[0])
			return nil
		},
	})

	done := make(chan error, 1)
	go func() { done <- s.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("an 'exit' inside a loaded file did not end the run:\n%s", output())
	}

	// The exit ended the session rather than just the file, so nothing after it
	// ran. This used to be swallowed: handlers had no way to report a stop.
	if got := strings.Join(marked, ","); got != "before" {
		t.Fatalf("ran %q; commands after 'exit' should not have run", got)
	}
}

func TestSession_HelpListsBuiltinsInGroups(t *testing.T) {
	s, _, output := newTestSession(t)

	if err := s.Dispatch("help"); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		GroupSession, GroupSimulation, GroupScheduling,
		"step", "rate:sim", "rate:publish", "every", "load", "scenarios",
	} {
		if !strings.Contains(output(), want) {
			t.Errorf("help does not mention %q:\n%s", want, output())
		}
	}
}

func TestSession_DoIsSafeAfterTheSessionEnds(t *testing.T) {
	source := newLineSource(false)
	s, _, _ := newTestSession(t, WithSource(source))

	if err := s.Run(); err != nil {
		t.Fatal(err)
	}

	// A front end that outlives the simulation (a UI still repainting, a
	// goroutine winding down) must not block or panic on a late command.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range lineBuffer * 2 {
			s.Do("step 1")
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Do blocked after the session ended")
	}
}

// waitFor polls cond until it holds, failing the test if it never does.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
