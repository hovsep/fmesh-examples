package env

import (
	"fmt"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

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

func (h *Habitat) AddAggregatedState() *Habitat {
	agg, err := newAggregator("aggregated_state", h.FM, []string{
		"gas::temperature",
		"sun::uvi",
		"human-Leon::is_alive",
		"human-Leon::brain_activity",
		"human-Leon::brain_activity_trend",
		"human-Leon::body_temperature", //@TODO: get human component name dynamically
		"human-Leon::heart_rate",
		"human-Leon::heart_cardiac_activation",
		"human-Leon::pleural_pressure",
		"human-Leon::respiratory_rate",
		"human-Leon::lung_left_volume",
		"human-Leon::lung_left_flow",
		"human-Leon::lung_left_alveolar_pressure",
		"human-Leon::lung_left_gas_composition",

		"human-Leon::lung_right_volume",
		"human-Leon::lung_right_flow",
		"human-Leon::lung_right_alveolar_pressure",
		"human-Leon::lung_right_gas_composition",
	})

	if err != nil {
		// @TODO: handle error
		panic(err)
	}

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
