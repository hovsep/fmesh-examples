// Package protocol parses the simulation's telemetry lines.
//
// The simulation publishes one metric per line as "<key> <value>", where the
// key may itself carry a scalar suffix ("<key>:<scalar> <value>"); the key is
// taken verbatim, so both forms parse the same way. This used to arrive over a
// unix socket, but the dashboard now runs in the same process as the sim and
// receives the lines directly over a Go channel -- only the parsing remains here.
package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// SignalUpdate is a parsed telemetry line: a metric key and its numeric value.
type SignalUpdate struct {
	Key   string
	Value float64
}

// Parse turns one telemetry line ("<key> <value>") into a SignalUpdate.
func Parse(line string) (SignalUpdate, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return SignalUpdate{}, fmt.Errorf("invalid line format: %s", line)
	}

	key := parts[0]
	value, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return SignalUpdate{}, fmt.Errorf("skipping non-numeric value for %s: %s", key, parts[1])
	}

	return SignalUpdate{Key: key, Value: value}, nil
}
