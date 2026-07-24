package helper

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

type PortPair [2]*port.Port

type PipeLineStageFunction func(signals *signal.Group) (*signal.Group, error)

// FindHumanComponent finds the first component that represents a human organism
func FindHumanComponent(fm *fmesh.FMesh) *component.Component {
	return fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "organism") &&
			c.Labels().ValueIs("genus", "homo") &&
			c.Labels().ValueIs("species", "sapiens")
	})
}

// PipeSpec is a single wiring edge from an output port to an input port.
type PipeSpec struct {
	From *port.Port
	To   *port.Port
}

// MultiPipe wires several 1:1 connections, reporting which one failed.
//
// Wiring code reads as a list of edges, and a bare error from PipeTo gives no
// clue which of a dozen lines produced it.
func MultiPipe(specs ...PipeSpec) error {
	for _, spec := range specs {
		if spec.From == nil || spec.To == nil {
			return fmt.Errorf("cannot pipe: %s", describePipe(spec))
		}
		if err := spec.From.PipeTo(spec.To); err != nil {
			return fmt.Errorf("piping %s: %w", describePipe(spec), err)
		}
	}
	return nil
}

func describePipe(spec PipeSpec) string {
	name := func(p *port.Port) string {
		if p == nil {
			return "<missing port>"
		}
		return p.Name()
	}
	return fmt.Sprintf("%s -> %s", name(spec.From), name(spec.To))
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

func PipelineActivationFunc(inputPortNames []string, outputPortName string, stageFuncs ...PipeLineStageFunction) component.ActivationFunc {
	return func(this *component.Component) error {
		signals := this.Inputs().ByNames(inputPortNames...).Signals()
		var stageErr error

		for i, stageFunc := range stageFuncs {
			signals, stageErr = stageFunc(signals)
			if stageErr != nil {
				return fmt.Errorf("pipeline stage %d failed: %w", i, stageErr)
			}
		}

		return this.OutputByName(outputPortName).PutSignalGroups(signals)
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
