package annotation

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// AutopipeComponent checks for "@autopipe" annotation (label) and creates pipes to comp inputs
// This is an experimental feature and currently only 1:1 pipes are supported
func AutopipeComponent(mesh *fmesh.FMesh, destComponent *component.Component) {
	destComponent.Inputs().ForEach(func(port *port.Port) error {
		if !port.Labels().HasAll("@autopipe-category", "@autopipe-component", "@autopipe-port") {
			// destPort does not set up
			return nil
		}

		sourceCategory, err := port.Labels().Value("@autopipe-category")
		if err != nil {
			return err
		}

		sourceComponentName, err := port.Labels().Value("@autopipe-component")
		if err != nil {
			return err
		}

		sourcePortName, err := port.Labels().Value("@autopipe-port")
		if err != nil {
			return err
		}

		targetComponent := mesh.ComponentByName(sourceComponentName)

		if targetComponent.HasChainableErr() {
			return fmt.Errorf("failed to find target component %s: %w", sourceComponentName, targetComponent.ChainableErr())
		}

		if !targetComponent.Labels().ValueIs("category", sourceCategory) {
			return fmt.Errorf("target component %s does not have required category label %s", sourceComponentName, sourceCategory)
		}

		targetPort := targetComponent.OutputByName(sourcePortName)
		if targetPort.HasChainableErr() {
			return fmt.Errorf("failed to find target port %s in component %s: %w", sourcePortName, sourceComponentName, targetPort.ChainableErr())
		}
		fmt.Printf("Creating auto pipe from %s.%s to %s.%s\n", sourceComponentName, sourcePortName, destComponent.Name(), port.Name())
		return targetPort.PipeTo(port).ChainableErr()
	})

}
