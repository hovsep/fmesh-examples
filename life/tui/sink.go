package tui

// channelSink is a step_sim sink.Sink that hands telemetry lines to the
// dashboard over an in-process channel instead of a unix socket.
//
// The simulation and the dashboard now run in one process, so there is nothing
// to serialize or connect: Publish drops a line onto a buffered channel that the
// ingest goroutine drains. Publish is called only from the simulation goroutine
// and Close only after it has stopped (see step_sim.Application.Run), so the two
// never race on the channel.
type channelSink struct {
	lines chan string
}

func newChannelSink(buffer int) *channelSink {
	return &channelSink{lines: make(chan string, buffer)}
}

// Publish enqueues a line, dropping it if the buffer is full. Back-pressure here
// would stall the simulation goroutine, and a dropped telemetry frame just means
// the dashboard repaints one snapshot later -- the same policy the old socket
// reader used.
func (s *channelSink) Publish(line string) error {
	select {
	case s.lines <- line:
	default:
	}
	return nil
}

// Close stops the ingest goroutine by closing the channel. It is safe to call
// once, after the simulation goroutine has stopped publishing.
func (s *channelSink) Close() error {
	close(s.lines)
	return nil
}
