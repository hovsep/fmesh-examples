package helper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

//@TODO: this sounds like a part of generic simulation package, check

// CommandLabel marks a signal as a control command. The habitat factors already
// use this convention (see env/factor/gas.go), and the body follows it so there
// is a single way to steer the simulation from outside.
const CommandLabel = "cmd"

// NamespaceSeparator splits a command name into the subsystem that owns it and
// the verb it performs, e.g. "intake:water" is the intake controller's "water".
const NamespaceSeparator = ":"

// PackCommand builds a control signal named e.g. "intake:water", carrying its
// arguments as scalars ("ml": 500). Arguments are scalars rather than a payload
// so a command can carry several named quantities and be read with the same
// Scalars API as every other structured signal in the mesh.
func PackCommand(name string, args map[string]float64) *signal.Signal {
	sig := signal.New(name).WithLabel(CommandLabel, name)
	for key, value := range args {
		sig = sig.WithScalar(key, value)
	}
	return sig
}

// UnpackCommand returns a command signal's name and arguments.
func UnpackCommand(sig *signal.Signal) (name string, args *meta.Scalars, err error) {
	if sig == nil {
		return "", nil, fmt.Errorf("command signal is nil")
	}
	if !sig.Labels().Has(CommandLabel) {
		return "", nil, fmt.Errorf("signal is not a command (no %q label)", CommandLabel)
	}
	return sig.Labels().ValueOrDefault(CommandLabel, ""), sig.Scalars(), nil
}

// IsCommand reports whether a signal carries a command.
func IsCommand(sig *signal.Signal) bool {
	return sig != nil && sig.Labels().Has(CommandLabel)
}

// CommandNamespace returns the owning subsystem of a command name, so
// "intake:water" routes on "intake". A name without a separator is its own
// namespace.
func CommandNamespace(name string) string {
	namespace, _, found := strings.Cut(name, NamespaceSeparator)
	if !found {
		return name
	}
	return namespace
}

// CommandVerb returns the action part of a command name ("water" for
// "intake:water"), or the whole name when it has no namespace.
func CommandVerb(name string) string {
	_, verb, found := strings.Cut(name, NamespaceSeparator)
	if !found {
		return name
	}
	return verb
}

// ForEachCommand invokes fn for every command signal waiting on the given input
// port of a component. Signals that are not commands are skipped rather than
// failing the activation: an error would stop the entire mesh run.
func ForEachCommand(c *component.Component, portName string, fn func(name string, args *meta.Scalars) error) error {
	in := c.InputByName(portName)
	if in == nil || !in.HasSignals() {
		return nil
	}

	return in.Signals().ForEach(func(sig *signal.Signal) error {
		name, args, err := UnpackCommand(sig)
		if err != nil {
			c.Logger().Println("ignoring non-command signal:", err)
			return nil
		}
		return fn(name, args)
	})
}

// Quantity is a number with the unit the user typed it in, e.g. 500ml or 200kcal.
// Commands accept whatever unit reads naturally and convert at the point of use,
// so "intake:water 500ml" and "intake:water 0.5l" mean the same thing.
//
// Durations are not among them: simulated time belongs to the simulation, so
// "30m" and "1d" go through simtime.ParseDuration.
type Quantity struct {
	Value float64
	Unit  string
}

// ParseQuantity reads a number with an optional trailing unit: "500ml", "200kcal",
// "1.5l", "30m", or a bare "0.8".
func ParseQuantity(s string) (Quantity, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Quantity{}, fmt.Errorf("empty quantity")
	}

	// The unit is the trailing run of letters; everything before it is the number.
	split := len(trimmed)
	for split > 0 {
		c := trimmed[split-1]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			break
		}
		split--
	}

	value, err := strconv.ParseFloat(trimmed[:split], 64)
	if err != nil {
		return Quantity{}, fmt.Errorf("invalid quantity %q: %w", s, err)
	}

	return Quantity{Value: value, Unit: strings.ToLower(trimmed[split:])}, nil
}

// Milliliters converts a volume quantity. A bare number is taken as millilitres.
func (q Quantity) Milliliters() (float64, error) {
	switch q.Unit {
	case "ml", "":
		return q.Value, nil
	case "cl":
		return q.Value * 10, nil
	case "dl":
		return q.Value * 100, nil
	case "l":
		return q.Value * 1000, nil
	default:
		return 0, fmt.Errorf("%q is not a volume (use ml, cl, dl or l)", q.Unit)
	}
}

// Kilocalories converts an energy quantity. A bare number is taken as kilocalories.
func (q Quantity) Kilocalories() (float64, error) {
	switch q.Unit {
	case "kcal", "cal", "":
		// "cal" reads as the everyday food calorie, which is a kilocalorie.
		return q.Value, nil
	case "kj":
		return q.Value / 4.184, nil
	default:
		return 0, fmt.Errorf("%q is not an energy (use kcal or kj)", q.Unit)
	}
}

// Grams converts a mass quantity. A bare number is taken as grams.
func (q Quantity) Grams() (float64, error) {
	switch q.Unit {
	case "g", "":
		return q.Value, nil
	case "mg":
		return q.Value / 1000, nil
	case "kg":
		return q.Value * 1000, nil
	default:
		return 0, fmt.Errorf("%q is not a mass (use mg, g or kg)", q.Unit)
	}
}
