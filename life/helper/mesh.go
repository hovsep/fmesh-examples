package helper

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
)

// FindHumanComponent finds the first component that represents a human organism
func FindHumanComponent(fm *fmesh.FMesh) *component.Component {
	return fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "organism") &&
			c.Labels().ValueIs("genus", "homo") &&
			c.Labels().ValueIs("species", "sapiens")
	})
}
