package simtest

import (
	"fmt"
	"math"
	"reflect"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

// SignalSource names a port whose traffic a test wants to inspect.
//
// It is the other half of Observable. An Observable answers "what number is this
// reading now", which is the right question for a temperature or a pulse and the
// wrong one for everything a simulation says that is not a measurement: a
// command acknowledged, an event emitted once, a composite carrying a payload
// that is a string. Those are claims about *signals*, and they need to be
// watched as signals -- including the claim that a port stayed silent, which no
// numeric reading can express.
type SignalSource struct {
	Name string
	Read func(*fmesh.FMesh) []*signal.Signal
}

// FromPort watches everything a component puts on one of its output ports.
func FromPort(componentName, portName string) SignalSource {
	return SignalSource{
		Name: componentName + "." + portName,
		Read: func(fm *fmesh.FMesh) []*signal.Signal {
			c := fm.ComponentByName(componentName)
			if c == nil {
				return nil
			}
			p := c.OutputByName(portName)
			if p == nil || !p.HasSignals() {
				return nil
			}
			return p.Signals().All()
		},
	}
}

// In redirects a SignalSource at a port inside a nested mesh.
func (s SignalSource) In(inner func(*fmesh.FMesh) *fmesh.FMesh) SignalSource {
	read := s.Read
	return SignalSource{
		Name: s.Name,
		Read: func(fm *fmesh.FMesh) []*signal.Signal {
			nested := inner(fm)
			if nested == nil {
				return nil
			}
			return read(nested)
		},
	}
}

// Named replaces the display name.
func (s SignalSource) Named(name string) SignalSource {
	s.Name = name
	return s
}

// SignalObservation is every signal a port carried, split around the command.
type SignalObservation struct {
	Name string

	// Before is what the port carried during the baseline window, and After
	// what it carried during the window at the end of the run. Both are
	// flattened across every sampled cycle in that window, oldest first.
	Before []*signal.Signal
	After  []*signal.Signal
}

// SignalCheck is one assertion about what a port carried.
type SignalCheck interface {
	Describe() string
	Verify(SignalObservation) error
}

type signalCheckFunc struct {
	describe string
	verify   func(SignalObservation) error
}

func (c signalCheckFunc) Describe() string                 { return c.describe }
func (c signalCheckFunc) Verify(o SignalObservation) error { return c.verify(o) }
func signalCheck(describe string, verify func(SignalObservation) error) SignalCheck {
	return signalCheckFunc{describe: describe, verify: verify}
}

// Emits asserts the port carried at least one signal after the command.
func Emits() SignalCheck {
	return signalCheck("emit something", func(o SignalObservation) error {
		if len(o.After) > 0 {
			return nil
		}
		return fmt.Errorf("expected at least one signal, got none")
	})
}

// EmitsNothing asserts the port stayed silent after the command.
//
// Silence is a claim no numeric reading can make. A component that has stopped
// publishing and one that is publishing its last value forever look identical
// through an Observable, and telling them apart is how the difference between a
// body that is failing and a body that is frozen becomes visible.
func EmitsNothing() SignalCheck {
	return signalCheck("stay silent", func(o SignalObservation) error {
		if len(o.After) == 0 {
			return nil
		}
		return fmt.Errorf("expected silence, got %d signal(s), the first carrying %v",
			len(o.After), o.After[0].Payload())
	})
}

// EmitsAtLeast asserts a minimum number of signals after the command, counted
// across every sampled cycle in the window.
func EmitsAtLeast(n int) SignalCheck {
	return signalCheck(fmt.Sprintf("emit at least %d signals", n), func(o SignalObservation) error {
		if len(o.After) >= n {
			return nil
		}
		return fmt.Errorf("expected at least %d signals, got %d", n, len(o.After))
	})
}

// StartsEmitting asserts the port was silent before the command and spoke after
// it, which is the shape of an event a command caused.
func StartsEmitting() SignalCheck {
	return signalCheck("start emitting", func(o SignalObservation) error {
		if len(o.Before) > 0 {
			return fmt.Errorf("expected silence beforehand, but the port was already carrying %d signal(s)", len(o.Before))
		}
		if len(o.After) == 0 {
			return fmt.Errorf("expected it to start emitting, but it stayed silent")
		}
		return nil
	})
}

// StopsEmitting asserts the port was carrying signals before the command and
// went quiet after it.
func StopsEmitting() SignalCheck {
	return signalCheck("stop emitting", func(o SignalObservation) error {
		if len(o.Before) == 0 {
			return fmt.Errorf("expected it to be emitting beforehand, but it was already silent")
		}
		if len(o.After) > 0 {
			return fmt.Errorf("expected it to fall silent, but it carried %d more signal(s)", len(o.After))
		}
		return nil
	})
}

// PayloadIs asserts some signal after the command carried exactly this payload.
//
// Compared with reflect.DeepEqual rather than numerically, because this is the
// check for payloads that are not measurements: a string tag, a state name, a
// bool. Use Reaches for numbers.
func PayloadIs(want any) SignalCheck {
	return signalCheck(fmt.Sprintf("carry the payload %v", want), func(o SignalObservation) error {
		for _, sig := range o.After {
			if reflect.DeepEqual(sig.Payload(), want) {
				return nil
			}
		}
		return fmt.Errorf("no signal carried %v; saw %s", want, summarisePayloads(o.After))
	})
}

// EveryPayloadIs asserts every signal after the command carried this payload,
// and that there was at least one.
func EveryPayloadIs(want any) SignalCheck {
	return signalCheck(fmt.Sprintf("carry only the payload %v", want), func(o SignalObservation) error {
		if len(o.After) == 0 {
			return fmt.Errorf("expected signals carrying %v, got none", want)
		}
		for _, sig := range o.After {
			if !reflect.DeepEqual(sig.Payload(), want) {
				return fmt.Errorf("a signal carried %v rather than %v", sig.Payload(), want)
			}
		}
		return nil
	})
}

// Labelled asserts some signal after the command carried this label.
//
// Labels are how signals in this codebase say what kind of thing they are --
// which command they answer, which category they belong to -- so this is the
// check for routing rather than for values.
func Labelled(name, value string) SignalCheck {
	return signalCheck(fmt.Sprintf("be labelled %s=%s", name, value), func(o SignalObservation) error {
		for _, sig := range o.After {
			if sig.Labels().ValueOrDefault(name, "") == value {
				return nil
			}
		}
		return fmt.Errorf("no signal was labelled %s=%s (saw %d signal(s))", name, value, len(o.After))
	})
}

// CarriesScalar asserts some signal after the command carried a named scalar at
// all, whatever its value.
func CarriesScalar(name string) SignalCheck {
	return signalCheck(fmt.Sprintf("carry the scalar %q", name), func(o SignalObservation) error {
		for _, sig := range o.After {
			if sig.Scalars().Has(name) {
				return nil
			}
		}
		return fmt.Errorf("no signal carried a %q scalar (saw %d signal(s))", name, len(o.After))
	})
}

// ScalarIs asserts some signal after the command carried a named scalar at
// roughly a given value.
func ScalarIs(name string, want, tolerance float64) SignalCheck {
	return signalCheck(fmt.Sprintf("carry %s=%.4g (±%.3g)", name, want, tolerance),
		func(o SignalObservation) error {
			for _, sig := range o.After {
				if sig.Scalars().Has(name) &&
					math.Abs(sig.Scalars().ValueOrDefault(name, 0)-want) <= tolerance {
					return nil
				}
			}
			return fmt.Errorf("no signal carried %s within %.3g of %.4g (saw %d signal(s))",
				name, tolerance, want, len(o.After))
		})
}

// Matching is the escape hatch: an arbitrary predicate over the signals a port
// carried, with a description for the failure message.
func Matching(describe string, pred func(*signal.Signal) bool) SignalCheck {
	return signalCheck(describe, func(o SignalObservation) error {
		for _, sig := range o.After {
			if pred(sig) {
				return nil
			}
		}
		return fmt.Errorf("no signal satisfied %q (saw %d signal(s))", describe, len(o.After))
	})
}

// AllSignals requires every one of several signal checks to pass.
func AllSignals(checks ...SignalCheck) SignalCheck {
	describe := ""
	for i, c := range checks {
		if i > 0 {
			describe += " and "
		}
		describe += c.Describe()
	}
	return signalCheck(describe, func(o SignalObservation) error {
		for _, c := range checks {
			if err := c.Verify(o); err != nil {
				return err
			}
		}
		return nil
	})
}

// summarisePayloads renders a few payloads for a failure message without
// printing a whole window's worth.
func summarisePayloads(sigs []*signal.Signal) string {
	if len(sigs) == 0 {
		return "no signals at all"
	}
	const most = 3
	out := ""
	for i, sig := range sigs {
		if i == most {
			out += fmt.Sprintf(", ... (%d more)", len(sigs)-most)
			break
		}
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%v", sig.Payload())
	}
	return out
}
