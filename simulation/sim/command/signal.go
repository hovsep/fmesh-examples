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

// Label is the metadata key that marks a signal as a control command.
const Label = "cmd"

// NamespaceSeparator splits a command name into the subsystem that owns it and
// the verb it performs, e.g. "intake:water" is the intake controller's "water".
const NamespaceSeparator = ":"

// Pack builds a control signal named e.g. "intake:water", carrying its arguments
// as numeric metadata ("ml": 500). Arguments are metadata rather than a payload
// so a command can carry several named quantities and be read with the same Meta
// API as every other structured signal in the mesh. The name is written last,
// so an argument called Label cannot overwrite it.
func Pack(name string, args map[string]float64) *signal.Signal {
	return signal.New(name).WithMetaMany(args).WithMeta(Label, name)
}

// Unpack returns a command signal's name and arguments. The arguments are a
// copy of the signal's metadata without the Label entry.
func Unpack(sig *signal.Signal) (name string, args *meta.Meta, err error) {
	if sig == nil {
		return "", nil, fmt.Errorf("command signal is nil")
	}
	if !IsCommand(sig) {
		return "", nil, fmt.Errorf("signal is not a command (no %q metadata entry)", Label)
	}
	return sig.Meta().ValueOrDefault(Label, ""), sig.Meta().Clone().Remove(Label), nil
}

// IsCommand reports whether a signal carries a command.
func IsCommand(sig *signal.Signal) bool {
	if sig == nil {
		return false
	}
	_, err := sig.Meta().Value[string](Label)
	return err == nil
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
func ForEach(c *component.Component, portName string, fn func(name string, args *meta.Meta) error) error {
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
