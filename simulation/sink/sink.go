// Package sink is where a simulation's telemetry goes: a channel a dashboard
// drains, stdout, or nowhere at all.
//
// A sink is written to from the simulation's own goroutine, on every published
// snapshot, so implementations must never block or apply back-pressure: a slow
// consumer has to cost dropped lines, not a slowed-down simulation.
package sink

// Sink receives telemetry lines from a running simulation.
type Sink interface {
	// Publish hands over one line. It must not block.
	Publish(line string) error

	// Close releases whatever the sink holds (channels, connections, files).
	// It must be safe to call more than once.
	Close() error
}
