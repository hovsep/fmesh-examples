package sink

type NoopSink struct {
}

func NewNoopSink() *NoopSink {
	return &NoopSink{}
}

func (s *NoopSink) Publish(line string) error {
	_ = line
	return nil
}

func (s *NoopSink) Close() error {
	return nil
}
