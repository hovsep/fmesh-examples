package service

import (
	"errors"
	"math/rand"
)

// UnstableService represents a service that fails randomly to simulate real-world unreliability
type UnstableService struct {
	failureRate float32
}

// NewUnstableService creates a new unstable service with the specified failure rate
func NewUnstableService(failureRate float32) *UnstableService {
	return &UnstableService{
		failureRate: failureRate,
	}
}

// Call attempts to call the service with the given request
// Returns an error based on the configured failure rate
func (s *UnstableService) Call(request string) (string, error) {
	if rand.Float32() < s.failureRate {
		return "", errors.New("service unavailable")
	}
	return "Response for " + request, nil
}
