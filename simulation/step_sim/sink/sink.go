package sink

type Sink interface {
	Publish(line string) error
}
