package helper

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

type PortPair [2]*port.Port

// FindHumanComponent finds the first component that represents a human organism
func FindHumanComponent(fm *fmesh.FMesh) *component.Component {
	return fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "organism") &&
			c.Labels().ValueIs("genus", "homo") &&
			c.Labels().ValueIs("species", "sapiens")
	})
}

// MultiForward helps to make multiple 1:1 port forwarding easier
func MultiForward(portPairs ...PortPair) error {
	for _, pair := range portPairs {
		err := port.ForwardSignals(pair[0], pair[1])
		if err != nil {
			return err
		}
	}
	return nil
}

// @TODO: this can be reused, make it part of fmesh (plugin or something)
// SequentialActivationFunc allows composing multiple activation functions into one
func SequentialActivationFunc(funcs ...component.ActivationFunc) component.ActivationFunc {
	return func(this *component.Component) error {
		for _, f := range funcs {
			if err := f(this); err != nil {
				return err
			}
		}
		return nil
	}
}

func PipelineActivationFunc(inputPortNames []string, outputPortName string, stageFuncs ...func(signals *signal.Group) (*signal.Group, error)) component.ActivationFunc {
	return func(this *component.Component) error {
		signals := this.Inputs().ByNames(inputPortNames...).Signals()
		var stageErr error

		for _, stageFunc := range stageFuncs {
			signals, stageErr = stageFunc(signals)
			if stageErr != nil {
				return fmt.Errorf("pipeline stage %s failed: %w", stageFunc, stageErr)
			}
		}

		return this.OutputByName(outputPortName).PutSignalGroups(signals).ChainableErr()
	}
}

func CountInputSignals(c *component.Component) map[string]int {
	res := make(map[string]int)
	c.Inputs().ForEach(func(p *port.Port) error {
		res[p.Name()] = p.Signals().Len()
		return nil
	})
	return res
}
