package habitat

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/annotation"
	"github.com/hovsep/fmesh-examples/life/habitat/factor"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "habitat_mesh"
)

// GetMesh builds the habitat mesh
func GetMesh() *fmesh.FMesh {
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second, // One mesh run (or 1 simulation tick) must not exceed this limit
	})

	addFactors(mesh, getFactors())

	return mesh
}

// getFactors returns a collection of habitat factors
func getFactors() *component.Collection {
	factors := component.NewCollection().Add(
		factor.GetTimeComponent(),
		factor.GetSunComponent(),
		factor.GetTemperatureComponent(),
		factor.GetAirComponent(),
	)

	return factors
}

// addFactors adds all exposure factors to the habitat mesh
func addFactors(habitatMesh *fmesh.FMesh, factors *component.Collection) {
	// Add all factors to the mesh
	factors.ForEach(func(c *component.Component) error {
		return habitatMesh.AddComponents(c).ChainableErr()
	}).ForEach(func(c *component.Component) error {
		// Handle auto piping
		annotation.AutopipeComponent(habitatMesh, c)
		return c.ChainableErr()
	})
}

func AddOrganisms(habitatMesh *fmesh.FMesh, organisms ...*component.Component) {
	for _, organism := range organisms {
		annotation.AutopipeComponent(habitatMesh, organism)
		habitatMesh.AddComponents(organism)
	}
}
