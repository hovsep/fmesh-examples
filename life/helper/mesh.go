package helper

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
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
