package strategy

import "github.com/hovsep/fmesh/component"

type StateInitializer func(state component.State)

type Logic = component.ActivationFunc

type RetryStrategy struct {
	Name             string
	StateInitializer StateInitializer
	Logic            Logic
}

func NewRetryStrategy(name string, stateInitializer StateInitializer, logic Logic) *RetryStrategy {
	return &RetryStrategy{
		Name:             name,
		StateInitializer: stateInitializer,
		Logic:            logic,
	}
}
