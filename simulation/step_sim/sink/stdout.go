package sink

import "fmt"

type StdOutSink struct {
}

func NewStdOutSink() *StdOutSink {
	return &StdOutSink{}
}

func (s *StdOutSink) Publish(line string) error {
	_, err := fmt.Println(line)
	return err
}
