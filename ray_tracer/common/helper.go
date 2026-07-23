package common

import (
	"fmt"

	"github.com/hovsep/fmesh/signal"
)

// ScalarInt reads a numeric signal metadata value as an int
func ScalarInt(sig *signal.Signal, name string) int {
	return int(sig.Scalars().ValueOrDefault(name, 0))
}

// IndexedPortName returns the name of the index-th port of an indexed group
func IndexedPortName(prefix string, index int) string {
	return fmt.Sprintf("%s%d", prefix, index)
}

// WorkerName returns the name of the index-th component of a parallel chain
func WorkerName(prefix string, index int) string {
	return fmt.Sprintf("%s-%d", prefix, index)
}

// Must panics on mesh construction errors: the mesh structure is static,
// so any error here is a programming mistake, not a runtime condition
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// MustOK panics on mesh wiring errors, see Must
func MustOK(err error) {
	if err != nil {
		panic(err)
	}
}
