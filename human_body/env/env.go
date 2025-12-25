package env

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/annotation"
	"github.com/hovsep/fmesh-examples/human_body/env/factor"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "env"
)

// GetMesh builds the environment mesh
func GetMesh() *fmesh.FMesh {
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second, // One mesh run (or 1 simulation tick) must not exceed this limit
	})

	addFactors(mesh, getFactors())

	return mesh
}

// getFactors returns a collection of environment exposure factors
func getFactors() *component.Collection {
	factors := component.NewCollection().Add(
		factor.GetTimeComponent(),
		factor.GetSunComponent(),
		factor.GetTemperatureComponent(),
		factor.GetAirComponent(),
	)

	return factors
}

// addFactors adds all exposure factors to the mesh
func addFactors(envMesh *fmesh.FMesh, factors *component.Collection) {
	// Add all factors to the mesh
	factors.ForEach(func(c *component.Component) error {
		return envMesh.AddComponents(c).ChainableErr()
	}).ForEach(func(c *component.Component) error {
		// Handle auto piping
		annotation.AutopipeComponent(envMesh, c)
		return c.ChainableErr()
	})
}

func AddOrganisms(envMesh *fmesh.FMesh, organisms ...*component.Component) {
	for _, organism := range organisms {
		annotation.AutopipeComponent(envMesh, organism)
		envMesh.AddComponents(organism)
	}
}
