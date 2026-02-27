package env

import (
	"fmt"
	"strings"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	meshName = "habitat_mesh"
)

// Habitat is a useful wrapper around F-Mesh that describes a habitat
type Habitat struct {
	FM *fmesh.FMesh
}

// NewHabitat builds the new habitat
func NewHabitat(factors *component.Collection) *Habitat {
	habitat := &Habitat{}
	habitat.FM = fmesh.NewWithConfig(meshName, &fmesh.Config{
		Debug:       false,
		CyclesLimit: fmesh.UnlimitedCycles,
		TimeLimit:   60 * time.Second, // One mesh run (or 1 simulation tick) must not exceed this limit
	})

	return habitat.addFactors(factors)
}

// addFactors adds all exposure factors to the habitat mesh
func (h *Habitat) addFactors(factors *component.Collection) *Habitat {
	if !factors.AnyMatch(func(factor *component.Component) bool {
		return factor.Name() == "time"
	}) {
		panic("Time factor is required for the habitat mesh")
	}

	// Add all factors to the mesh
	factors.ForEach(func(c *component.Component) error {
		return h.FM.AddComponents(c).ChainableErr()
	}).ForEach(func(c *component.Component) error {
		return c.ChainableErr()
	})

	// Connect inter-factor pipes
	h.FM.Components().ForEach(func(c *component.Component) error {
		h.connectToTimeFactor(c)
		return h.FM.ChainableErr()
	})
	return h
}

func (h *Habitat) AddAggregatedState() *Habitat {
	agg, err := newAggregator("aggregated_state", h.FM, []string{
		"air::temperature",
		"sun::uvi",
		"human-Leon::is_alive",
		"human-Leon::brain_activity",
		"human-Leon::brain_activity_trend",
		"human-Leon::body_temperature", //@TODO: get human component name dynamically
		"human-Leon::heart_rate",
		"human-Leon::heart_cardiac_activation"})

	if err != nil {
		panic(err)
	}

	//DEBUG_START
	agg.SetupHooks(func(hooks *component.Hooks) {
		hooks.AfterActivation(func(ctx *component.ActivationContext) error {
			cmp := ctx.Component
			//cmp.Logger().Println("ports signals counts:")
			cmp.Inputs().ForEach(func(in *port.Port) error {
				//cmp.Logger().Printf("port %s: %d signals", in.Name(), in.Signals().Len())
				return nil
			})
			return nil
		})
	})
	//DEBUG_END

	h.FM.AddComponents(agg)
	return h
}

func (h *Habitat) AddAggregatedStatePublisher() *Habitat {
	agg := h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "aggregator")
	})

	if agg == nil {
		panic("Aggregator not found")
	}

	publisher := component.New("aggregated_state_publisher").
		WithDescription("publishes aggregated state to unit socket").
		AddLabel("role", "publisher").
		AddInputs("aggregated_state").
		AddOutputs("stream").
		WithActivationFunc(func(this *component.Component) error {
			this.InputByName("aggregated_state").Signals().ForEach(func(sig *signal.Signal) error {
				if !sig.Labels().Has("from") {
					return fmt.Errorf("missing 'from' label")
				}

				return this.OutputByName("stream").PutPayloads(fmt.Sprintf("%s %v \n", sig.Labels().ValueOrDefault("from", "unknown"), sig.PayloadOrNil())).ChainableErr()
			})

			return nil
		})

	agg.OutputByName("aggregated_state").PipeTo(publisher.InputByName("aggregated_state"))

	h.FM.AddComponents(publisher)
	return h
}

func newAggregator(name string, fm *fmesh.FMesh, inputPaths []string) (*component.Component, error) {
	agg := component.New(name).
		WithDescription("composes data from multiple sources into one (single source of true for UI)").
		AddLabel("role", "aggregator"). //@TODO: generalise and refactor components taxonomy (same as signals)
		AddOutputs("aggregated_state").
		WithActivationFunc(func(this *component.Component) error {
			return this.Inputs().ForEach(func(in *port.Port) error {
				// Add all signals from the input port to the aggregated state (for later publishing)

				err := port.ForwardWithMap(in, this.OutputByName("aggregated_state"), func(sig *signal.Signal) *signal.Signal {
					return sig.AddLabel("from", in.Name())
				})

				if err != nil {
					return err
				}

				// Just proxy "in -> out" with the same port name
				err = port.ForwardSignals(in, this.OutputByName(in.Name()))
				if err != nil {
					return err
				}
				return nil
			}).ChainableErr()
		})

	// Dynamic piping (extract to plugin or helper)
	for _, inputPath := range inputPaths {
		if inputPath == "" {
			return nil, fmt.Errorf("empty input path")
		}

		if !strings.Contains(inputPath, "::") {
			return nil, fmt.Errorf("delimiter missing in input path: %s", inputPath)
		}

		segments := strings.Split(inputPath, "::")
		if len(segments) != 2 {
			return nil, fmt.Errorf("invalid input path: %s", inputPath)
		}

		componentName, srcPortName := segments[0], segments[1]

		srcComponent := fm.ComponentByName(componentName)
		if srcComponent == nil {
			return nil, fmt.Errorf("unknown component: %s", componentName)
		}

		sourcePort := srcComponent.OutputByName(srcPortName)

		if sourcePort == nil {
			return nil, fmt.Errorf("could not find source port: %s", srcPortName)
		}

		// Add input and output with the same name and connect to the source
		agg.AddInputs(inputPath).AddOutputs(inputPath)
		err := sourcePort.PipeTo(agg.InputByName(inputPath)).ChainableErr()
		if err != nil {
			return nil, err
		}
	}

	return agg, nil
}

// AddOrganisms adds organisms components to the habitat mesh
func (h *Habitat) AddOrganisms(organisms ...*component.Component) *Habitat {
	for _, organism := range organisms {
		h.FM.AddComponents(organism)

		// Connect to habitat factors
		h.FM.Components().ForEach(func(factor *component.Component) error {
			return factor.Outputs().ForEach(func(factorOutput *port.Port) error {
				// Check if the organism has relevant input
				orgInput := organism.Inputs().FindAny(func(p *port.Port) bool {
					return p.Name() == fmt.Sprintf("habitat_%s_%s", factor.Name(), factorOutput.Name())
				})

				if orgInput == nil {
					// No such input, skip
					return nil
				}

				return factorOutput.PipeTo(orgInput).ChainableErr()
			}).ChainableErr()
		})
	}
	return h
}

// getTimeFactor returns the time factor component
func (h *Habitat) getTimeFactor() *component.Component {
	return h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Name() == "time"
	})
}

// connectToTimeFactor connects the given component to the time factor
func (h *Habitat) connectToTimeFactor(c *component.Component) {
	habitatTimeFactor := h.getTimeFactor()
	c.Inputs().ForEach(func(p *port.Port) error {
		if p.Name() == "time" {
			habitatTimeFactor.OutputByName("tick").PipeTo(p)
		}
		return p.ChainableErr()
	})
}
