package command

import (
	"fmt"
	"strconv"
	"strings"
)

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
