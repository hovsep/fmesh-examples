package command

import (
	"fmt"
	"strings"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// A command reaches a simulation twice over: first as text a person typed,
// which the Registry looks up and runs, and then as a signal travelling into the
// mesh to whichever component owns it. This file is the second half.
//
// The two halves live together because they are the same idea in two
// transports, and separating them is how a name ends up meaning one thing at the
// prompt and another on the wire.

// Label marks a signal as a control command.
const Label = "cmd"

// NamespaceSeparator splits a command name into the subsystem that owns it and
// the verb it performs, e.g. "intake:water" is the intake controller's "water".
const NamespaceSeparator = ":"

// Pack builds a control signal named e.g. "intake:water", carrying its arguments
// as scalars ("ml": 500). Arguments are scalars rather than a payload so a
// command can carry several named quantities and be read with the same Scalars
// API as every other structured signal in the mesh.
func Pack(name string, args map[string]float64) *signal.Signal {
	sig := signal.New(name).WithLabel(Label, name)
	for key, value := range args {
		sig = sig.WithScalar(key, value)
	}
	return sig
}

// Unpack returns a command signal's name and arguments.
func Unpack(sig *signal.Signal) (name string, args *meta.Scalars, err error) {
	if sig == nil {
		return "", nil, fmt.Errorf("command signal is nil")
	}
	if !sig.Labels().Has(Label) {
		return "", nil, fmt.Errorf("signal is not a command (no %q label)", Label)
	}
	return sig.Labels().ValueOrDefault(Label, ""), sig.Scalars(), nil
}

// IsCommand reports whether a signal carries a command.
func IsCommand(sig *signal.Signal) bool {
	return sig != nil && sig.Labels().Has(Label)
}

// Namespace returns the owning subsystem of a command name, so "intake:water"
// routes on "intake". A name without a separator is its own namespace.
func Namespace(name string) string {
	namespace, _, found := strings.Cut(name, NamespaceSeparator)
	if !found {
		return name
	}
	return namespace
}

// Verb returns the action part of a command name ("water" for "intake:water"),
// or the whole name when it has no namespace.
func Verb(name string) string {
	_, verb, found := strings.Cut(name, NamespaceSeparator)
	if !found {
		return name
	}
	return verb
}

// ForEach invokes fn for every command signal waiting on the given input port of
// a component. Signals that are not commands are skipped rather than failing the
// activation: an error would stop the entire mesh run.
func ForEach(c *component.Component, portName string, fn func(name string, args *meta.Scalars) error) error {
	in := c.InputByName(portName)
	if in == nil || !in.HasSignals() {
		return nil
	}

	return in.Signals().ForEach(func(sig *signal.Signal) error {
		name, args, err := Unpack(sig)
		if err != nil {
			c.Logger().Println("ignoring non-command signal:", err)
			return nil
		}
		return fn(name, args)
	})
}
