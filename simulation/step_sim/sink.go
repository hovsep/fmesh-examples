package step_sim

type Sink interface {
	Publish(line string) error
}
