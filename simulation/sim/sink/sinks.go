package sink

import "fmt"

// Noop discards everything. It is the default: a simulation nobody is watching
// should not pay for telemetry.
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (s *Noop) Publish(string) error { return nil }

func (s *Noop) Close() error { return nil }

// Stdout prints every line. Useful for headless runs where the terminal is the
// consumer, and for piping telemetry into another program.
type Stdout struct{}

func NewStdout() *Stdout { return &Stdout{} }

func (s *Stdout) Publish(line string) error {
	_, err := fmt.Println(line)
	return err
}

func (s *Stdout) Close() error { return nil }

// Channel hands lines to a consumer in the same process.
//
// It is the sink to reach for when the front end runs alongside the simulation
// (the usual case): there is nothing to serialize or connect, Publish drops a
// line onto the channel and the consumer drains Lines on its own goroutine.
type Channel struct {
	lines chan string
}

// NewChannel returns a sink buffering up to size lines. Size it generously: a
// snapshot is many lines, and a consumer that falls behind should lose old
// snapshots rather than the newest one.
func NewChannel(size int) *Channel {
	return &Channel{lines: make(chan string, size)}
}

// Lines is the stream to drain. Close closes it, so a range over it ends when
// the simulation does.
func (s *Channel) Lines() <-chan string { return s.lines }

// Publish enqueues a line, dropping it if the buffer is full. Back-pressure
// here would stall the simulation, and a dropped line only means the consumer
// sees the next snapshot instead of this one.
func (s *Channel) Publish(line string) error {
	select {
	case s.lines <- line:
	default:
	}
	return nil
}

// Close stops the consumer by closing the channel. Call it once the simulation
// has stopped publishing, which is what session.Run guarantees.
func (s *Channel) Close() error {
	close(s.lines)
	return nil
}
