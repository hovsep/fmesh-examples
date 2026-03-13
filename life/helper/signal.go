package helper

import (
	"github.com/hovsep/fmesh/signal"
)

func AsF64(s *signal.Signal) float64 {
	if s == nil {
		panic("signal is nil")
	}

	payload, err := s.Payload()
	if err != nil {
		panic(err)
	}

	return payload.(float64)
}
