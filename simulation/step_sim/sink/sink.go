package sink

type Sink interface {
	Publish(line string) error
	// Close releases any resources held by the sink (connections, goroutines, files).
	// It must be safe to call multiple times.
	Close() error
}
