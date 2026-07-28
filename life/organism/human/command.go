package human

import (
	"fmt"
	"maps"
	"slices"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// ControlPort is the input every commandable component exposes.
const ControlPort = common.ControlPort

// commandRoutes maps a command namespace to the controller that owns it.
//
// Controllers are the body's command surface: the outside world never addresses
// an organ directly, it states an intent ("intake:water") and the owning
// controller decides what that means physiologically.
var commandRoutes = map[string]string{
	"intake":    "controller:intake",
	"smoke":     "controller:intake", // smoking is oral intake of a toxin
	"activity":  "controller:physical_stress",
	"emotion":   "controller:mental_stress",
	"excretion": "controller:excretion",
	"trauma":    "controller:trauma",
}

// CommandNamespaces returns the command namespaces the body accepts, sorted.
// Registration uses it to reject a typo at startup rather than at runtime.
func CommandNamespaces() []string {
	return slices.Sorted(maps.Keys(commandRoutes))
}

// AcceptsCommand reports whether a command name routes to a known controller.
func AcceptsCommand(name string) bool {
	_, ok := commandRoutes[helper.CommandNamespace(name)]
	return ok
}

// Command builds a control signal for the human's ctl port.
func Command(name string, args map[string]float64) *signal.Signal {
	return helper.PackCommand(name, args)
}

// routeCommands delivers control signals from the human's ctl port to the
// controller that owns each command's namespace.
//
// It runs before act(), so a command lands on its controller's input in the same
// mesh run as the tick that carries it, and the controller sees both together.
func routeCommands(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		ctl := this.InputByName(ControlPort)
		if !ctl.HasSignals() {
			return nil
		}

		return ctl.Signals().ForEach(func(sig *signal.Signal) error {
			name, _, err := helper.UnpackCommand(sig)
			if err != nil {
				// Not addressed to us. Ignore rather than fail: an activation
				// error stops the whole mesh run, and a stray signal should
				// never be able to halt a living body.
				this.Logger().Println("ignoring non-command signal on ctl port:", err)
				return nil
			}

			componentName, ok := commandRoutes[helper.CommandNamespace(name)]
			if !ok {
				this.Logger().Printf("no controller owns command %q (known namespaces: %v)\n", name, CommandNamespaces())
				return nil
			}

			target := mesh.ComponentByName(componentName)
			if target == nil {
				return fmt.Errorf("command %q routes to %q, which is not in the mesh", name, componentName)
			}

			return target.InputByName(ControlPort).PutSignals(sig)
		})
	}
}
