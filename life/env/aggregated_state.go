package env

import (
	"fmt"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

func newAggregator(name string, fm *fmesh.FMesh, inputPaths []string) (*component.Component, error) {
	agg, err := component.New(name,
		component.WithDescription("composes data from multiple sources into one (single source of true for UI)"),
		component.WithLabel("role", "aggregator"),
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
	// This could be an autowire plugin -- a rule naming an input "<component>::<port>"
	// would connect all of these with no loop at all. It stays explicit because of
	// what the loop does when it cannot find something: it fails, naming the
	// component. Renaming the gas factor to air broke exactly these paths, and
	// this error is what said so. Autowire declines to wire silently, by design,
	// and the same rename would have produced a UI that was simply empty.
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

// publishSignal renders one signal onto the telemetry stream as whitespace-separated
// "key value" lines, which is all the consumer (tui/protocol.Parse) can parse.
//
// A signal contributes at most one line for its payload, plus one line per scalar
// keyed "<key>:<scalarName>". Composite signals such as air or venous blood carry a
// string type tag as their payload and keep every real measurement in scalars, so
// without the scalar lines they would reach the UI carrying nothing at all.
func publishSignal(stream *port.Port, key string, sig *signal.Signal) error {
	if value, ok := signal.AsNumber(sig); ok {
		if err := stream.PutPayloads(fmt.Sprintf("%s %v \n", key, value)); err != nil {
			return err
		}
	}

	// Keys() is sorted, so the line order of a snapshot is stable.
	for _, name := range sig.Scalars().Keys() {
		line := fmt.Sprintf("%s:%s %v \n", key, name, sig.Scalars().ValueOrDefault(name, 0))
		if err := stream.PutPayloads(line); err != nil {
			return err
		}
	}
	return nil
}

func (h *Habitat) AddAggregatedState() (*Habitat, error) {
	// Habitat-level sources. The tick signal carries tick_count and
	// sim_duration_ms as scalars, so the UI can show real simulated time rather
	// than its own wall clock.
	paths := []string{
		"time::tick",
		"air::environmental_gas",
		"sun::uvi",
	}

	// Everything the body publishes, addressed by the human's actual name rather
	// than a hardcoded one, so renaming the subject does not silently empty the UI.
	body := human.Find(h.FM)
	if body == nil {
		return nil, fmt.Errorf("no human in the habitat to aggregate state from")
	}
	paths = append(paths, telemetry.Paths(body.Name())...)

	agg, err := newAggregator("aggregated_state", h.FM, paths)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}

	if err := h.FM.AddComponents(agg); err != nil {
		return nil, fmt.Errorf("failed to add aggregated_state component: %w", err)
	}
	return h, nil
}

func (h *Habitat) AddAggregatedStatePublisher() (*Habitat, error) {
	agg := h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "aggregator")
	})

	if agg == nil {
		return nil, fmt.Errorf("aggregator not found")
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
				return publishSignal(this.OutputByName("stream"), sig.Labels().ValueOrDefault("from", "unknown"), sig)
			})
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregated_state_publisher: %w", err)
	}

	if err := agg.OutputByName("aggregated_state").PipeTo(publisher.InputByName("aggregated_state")); err != nil {
		return nil, fmt.Errorf("failed to pipe aggregated state to publisher: %w", err)
	}

	if err := h.FM.AddComponents(publisher); err != nil {
		return nil, fmt.Errorf("failed to add publisher component: %w", err)
	}
	return h, nil
}
