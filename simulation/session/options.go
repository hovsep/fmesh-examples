package session

import (
	"io"
	"os"

	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/sink"
)

// Option configures a session. Everything a session needs from the outside
// world -- where it writes, where telemetry goes, where commands come from --
// arrives this way, so nothing has to be reached for globally.
type Option func(*Session)

// stdout writes to whatever os.Stdout is at the time of the write, rather than
// to whatever it was when the session was built. Front ends that redirect
// standard output (to capture a simulation's printing into a pane, say)
// install themselves after construction, and their output must not be bypassed.
type stdout struct{}

func (stdout) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

// WithOut sets where the session and its commands write. It defaults to
// standard output; a full-screen front end points it at its own pane, and a
// test at a buffer.
func WithOut(out io.Writer) Option {
	return func(s *Session) { s.out = out }
}

// WithSink sets where telemetry goes. It defaults to discarding it.
func WithSink(target sink.Sink) Option {
	return func(s *Session) { s.sink = target }
}

// WithSource sets where command lines come from. Without one, a session runs
// with no front end at all -- fine for a scripted or scheduled run.
func WithSource(source command.Source) Option {
	return func(s *Session) { s.source = source }
}

// WithTelemetry sets what to publish after an advance. The function is called
// only when the throttle allows a snapshot through, so it can be as expensive
// as rendering the whole state.
func WithTelemetry(snapshot func() []string) Option {
	return func(s *Session) { s.telemetry = snapshot }
}

// WithAutoPause stops the simulation as soon as an advance changes nothing,
// rather than spinning on a simulation that has settled.
func WithAutoPause(on bool) Option {
	return func(s *Session) { s.AutoPause = on }
}
