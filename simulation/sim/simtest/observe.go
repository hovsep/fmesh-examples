package simtest

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

// Observable names one number a simulation publishes, and knows how to read it.
//
// Everything a test can assert on is one of these, whether it comes off a port
// or out of a component's state. Nothing here knows what is being simulated:
// give it a component name and a port name and it will watch anything.
type Observable struct {
	// Name is what a failure message calls this. It also keys the expectations
	// in a Scenario, so it has to be stable.
	Name string

	// Read returns the current value, and whether there was one to read.
	//
	// The second return is not a formality. A port that published nothing this
	// cycle is a different thing from a port that published zero, and conflating
	// them reads a heart rate of nought off a beating heart -- which is exactly
	// what a first cut of this did.
	Read func(*fmesh.FMesh) (float64, bool)
}

// Port watches the payload of the first signal on a component's output port.
//
// It reads through Signal.AsNumber rather than a typed accessor on purpose. A
// mesh does not publish one numeric type -- a heart rate is a whole number of
// beats and a stomach fill is not -- and the typed accessors infer their type
// from the default they are given, so asking for a float64 with a default of 0
// silently returns that 0 for every int-valued port in the model.
func Port(componentName, portName string) Observable {
	return Observable{
		Name: componentName + "." + portName,
		Read: func(fm *fmesh.FMesh) (float64, bool) {
			sig := firstSignal(fm, componentName, portName)
			if sig == nil {
				return 0, false
			}
			return sig.AsNumber()
		},
	}
}

// Scalar watches one named scalar carried by a composite signal.
//
// Composite signals -- a breath of air, a sample of blood -- carry a string tag
// as their payload and every real measurement as a scalar, so this is the only
// way to see most of what a model publishes.
func Scalar(componentName, portName, scalarName string) Observable {
	return Observable{
		Name: componentName + "." + portName + ":" + scalarName,
		Read: func(fm *fmesh.FMesh) (float64, bool) {
			sig := firstSignal(fm, componentName, portName)
			if sig == nil || !sig.Scalars().Has(scalarName) {
				return 0, false
			}
			return sig.Scalars().ValueOrDefault(scalarName, 0), true
		},
	}
}

// StateValue watches a number a component keeps in its own state.
//
// State is where a model keeps what it is rather than what it is saying, so this
// reaches things no port exposes. It is also less stable than a port -- state
// keys are private by convention -- so prefer a Port where one exists.
//
// State values are `any`, and a simulation keeps more than numbers in there
// (durations, counters, whole objects). Anything this cannot read as a number
// reads as absent.
func StateValue(componentName, key string) Observable {
	return Observable{
		Name: componentName + "#" + key,
		Read: func(fm *fmesh.FMesh) (float64, bool) {
			c := fm.ComponentByName(componentName)
			if c == nil || !c.State().Has(key) {
				return 0, false
			}
			return asNumber(c.State().Get(key))
		},
	}
}

// In redirects an Observable at a component inside a nested mesh.
//
// A simulation may run a mesh inside a component -- an organism inside a world --
// and the inner components are invisible to the outer mesh's lookup. Given a way
// to find the inner mesh, this rebinds the same reader to it.
func (o Observable) In(inner func(*fmesh.FMesh) *fmesh.FMesh) Observable {
	read := o.Read
	return Observable{
		Name: o.Name,
		Read: func(fm *fmesh.FMesh) (float64, bool) {
			nested := inner(fm)
			if nested == nil {
				return 0, false
			}
			return read(nested)
		},
	}
}

// Named replaces the display name, for when the generated one is unwieldy.
func (o Observable) Named(name string) Observable {
	o.Name = name
	return o
}

func firstSignal(fm *fmesh.FMesh, componentName, portName string) *signal.Signal {
	c := fm.ComponentByName(componentName)
	if c == nil {
		return nil
	}
	p := c.OutputByName(portName)
	if p == nil || !p.HasSignals() {
		return nil
	}
	return p.Signals().First()
}

// asNumber reads the numeric types a simulation actually keeps in state.
func asNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case time.Duration:
		return n.Seconds(), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// ComponentPorts returns an Observable for every output port of a component.
//
// It is the blunt way to watch everything, for a model with no catalogue of its
// own observables. A model that has one should generate the list from that
// instead, so that adding a published value adds it to the tests.
func ComponentPorts(fm *fmesh.FMesh, componentName string) []Observable {
	c := fm.ComponentByName(componentName)
	if c == nil {
		return nil
	}

	var observables []Observable
	for _, p := range c.Outputs().AllOrdered() {
		observables = append(observables, Port(componentName, p.Name()))
	}
	return observables
}
