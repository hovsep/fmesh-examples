package sim

// Port names two ports mean the same thing in every simulation built this way,
// so that a component can be wired by convention rather than by a list.
//
// They are conventions rather than mechanism: nothing enforces them, and a
// component that declares neither simply receives neither. What they buy is that
// the clock can be broadcast to everything that keeps time, and commands routed
// to everything that takes them, without either side maintaining a registry of
// the other (see the autowire plugin in fmesh).
const (
	// ControlPort is the input port through which a component receives commands
	// from outside the simulation.
	ControlPort = "ctl"

	// TimePort is the input port carrying the tick. Components gate their
	// per-tick work on it, so an out-of-band command cannot make them advance
	// twice in one step.
	TimePort = "time"
)
