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
	agg, err := component.New(name,
		component.WithDescription("composes data from multiple sources into one (single source of true for UI)"),
		component.WithLabel("role", "aggregator"), //@TODO: generalise and refactor components taxonomy (same as signals)
		component.WithOutputs("aggregated_state"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.Inputs().ForEach(func(in *port.Port) error {
				// Add all signals from the input port to the aggregated state (for later publishing)
				err := port.ForwardWithMap(in, this.OutputByName("aggregated_state"), func(sig *signal.Signal) *signal.Signal {
					return sig.MapPayload(func(p any) any { return p }).WithLabel("from", in.Name())
				})

				if err != nil {
					return err
				}

				// Just proxy "in -> out" with the same port name
				return port.ForwardSignals(in, this.OutputByName(in.Name()))
			})
		}),
	)
	if err != nil {
		return nil, err
	}
	// Dynamic piping
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
		if err := agg.AddInputs(inputPath); err != nil {
			return nil, err
		}
		if err := agg.AddOutputs(inputPath); err != nil {
			return nil, err
		}
		if err := sourcePort.PipeTo(agg.InputByName(inputPath)); err != nil {
			return nil, err
		}
	}

	return agg, nil
}

func (h *Habitat) AddAggregatedState() *Habitat {
	agg, err := newAggregator("aggregated_state", h.FM, []string{
		"gas::environmental_gas",
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
		"human-Leon::lung_left_exhaled_gas",

		"human-Leon::lung_right_volume",
		"human-Leon::lung_right_flow",
		"human-Leon::lung_right_alveolar_pressure",
		"human-Leon::lung_right_exhaled_gas",
	})

	if err != nil {
		// @TODO: handle error
		panic(err)
	}

	if err := h.FM.AddComponents(agg); err != nil {
		panic(fmt.Sprintf("failed to add aggregated_state component: %v", err))
	}
	return h
}

func (h *Habitat) AddAggregatedStatePublisher() *Habitat {
	agg := h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "aggregator")
	})

	if agg == nil {
		panic("Aggregator not found")
	}

	publisher, err := component.New("aggregated_state_publisher",
		component.WithDescription("publishes aggregated state to unit socket"),
		component.WithLabel("role", "publisher"),
		component.WithInputs("aggregated_state"),
		component.WithOutputs("stream"),
		component.WithActivationFunc(func(this *component.Component) error {
			err := this.InputByName("aggregated_state").Signals().ForEach(func(sig *signal.Signal) error {
				if !sig.Labels().Has("from") {
					return fmt.Errorf("missing 'from' label")
				}

				return this.OutputByName("stream").PutPayloads(fmt.Sprintf("%s %v \n", sig.Labels().ValueOrDefault("from", "unknown"), sig.PayloadOrNil()))
			})
			return err
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create aggregated_state_publisher: %v", err))
	}

	if err := agg.OutputByName("aggregated_state").PipeTo(publisher.InputByName("aggregated_state")); err != nil {
		panic(fmt.Sprintf("failed to pipe aggregated state to publisher: %v", err))
	}

	if err := h.FM.AddComponents(publisher); err != nil {
		panic(fmt.Sprintf("failed to add publisher component: %v", err))
	}
	return h
}
