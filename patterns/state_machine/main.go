package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// Every state component has the same two inputs; its outputs are named after
// the events that lead out of it. A guard has one of each.
const (
	portEnter = "enter" // a transition arriving: this state becomes current
	portEvent = "event" // an event fired while this state is current
	portIn    = "in"
	portOut   = "out"
)

const (
	labelEvent   = "event"   // on a signal: the event it carries
	labelCurrent = "current" // on the mesh: the name of the current state
)

var (
	errUnknownEvent    = errors.New("unknown event")
	errEventNotAllowed = errors.New("event not allowed in this state")
	errGuardRejected   = errors.New("transition rejected by guard")
)

const orderTotal = 49.99

func main() {
	fmt.Println("=== Order Lifecycle Demo ===")
	fmt.Println("A state machine that is nothing but an fmesh: states are components,")
	fmt.Println("transitions are pipes, and the mesh graph is the state diagram.")
	fmt.Println()

	fm, err := getMesh("created")
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fmt.Printf("A new order arrives — state %q.\n\n", current(fm))

	fmt.Println("You can't ship what was never paid for:")
	fire(fm, "ship")

	fmt.Println("And refunds are not a thing in this shop:")
	fire(fm, "refund")

	fmt.Println("The customer edits the order — it loops back to where it was:")
	fire(fm, "update")

	fmt.Printf("Paying 5.00 for a %.2f order? The guard says no:\n", orderTotal)
	fire(fm, "pay", signal.New("order #1042").WithScalar("amount", 5.00))

	fmt.Println("Paying in full, by card:")
	fire(fm, "pay", signal.New("order #1042").WithScalar("amount", orderTotal).WithLabel("method", "card"))

	fmt.Println("The shop restarts overnight. The machine's whole state is one mesh label,")
	fmt.Printf("so a fresh mesh built with it picks up where this one left off (%q):\n", current(fm))
	resumed, err := getMesh(current(fm))
	if err != nil {
		fmt.Println("Failed to rebuild mesh:", err)
		os.Exit(1)
	}
	fire(resumed, "ship")
	fire(resumed, "deliver")

	fmt.Println("Delivered is the end of the road — nothing can happen anymore:")
	fire(resumed, "cancel")
	fmt.Printf("   done: %v\n\n", done(resumed))

	fmt.Println("=== Order Lifecycle Demo Complete ===")
}

// getMesh builds the order-lifecycle machine, starting in the given state.
//
// Read it as the state diagram: addState declares a circle and the arrows
// leaving it, and every port.Pipe is one of those arrows drawn to its target.
func getMesh(startAt string) (*fmesh.FMesh, error) {
	fm, err := fmesh.New("order lifecycle",
		fmesh.WithDescription("a state machine: states are components, transitions are pipes leaving event-named ports, a guard sits on its pipe"),
		fmesh.WithLabel(labelCurrent, startAt),
		// A refused event is an activation error, and the run must stop right
		// there: the machine has not moved and the caller gets the reason.
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, err
	}

	// Paying is allowed only for the full amount, and only with a stated
	// method: the guard reads the signal's scalars and labels, not its payload.
	paymentAccepted := signal.And(
		signal.HasLabel("method"),
		func(s *signal.Signal) bool { return s.Scalars().ValueOrDefault("amount", 0) >= orderTotal },
	)

	if err := errors.Join(
		addState(fm, "created", "update", "pay", "cancel"),
		addState(fm, "paid", "ship", "cancel"),
		addState(fm, "shipped", "deliver"),
		addState(fm, "delivered"), // no events out: final
		addState(fm, "cancelled"), // final
		addGuard(fm, "pay?", paymentAccepted),
	); err != nil {
		return nil, err
	}
	if fm.ComponentByName(startAt) == nil {
		return nil, fmt.Errorf("start state %q is not a state of this machine", startAt)
	}

	from := func(state, event string) *port.Port { return fm.ComponentByName(state).OutputByName(event) }
	to := func(state string) *port.Port { return fm.ComponentByName(state).InputByName(portEnter) }
	guard := fm.ComponentByName("pay?")

	if err := port.MultiPipe(
		port.Pipe{From: from("created", "update"), To: to("created")}, // self-loop
		port.Pipe{From: from("created", "pay"), To: guard.InputByName(portIn)},
		port.Pipe{From: guard.OutputByName(portOut), To: to("paid")},
		port.Pipe{From: from("created", "cancel"), To: to("cancelled")},
		port.Pipe{From: from("paid", "ship"), To: to("shipped")},
		port.Pipe{From: from("paid", "cancel"), To: to("cancelled")},
		port.Pipe{From: from("shipped", "deliver"), To: to("delivered")},
	); err != nil {
		return nil, err
	}
	return fm, nil
}

// addState adds one state component. Its output ports are the events that
// lead out of it, so a state with no events is final by construction: after
// entering it the mesh has nothing left to do, and the run simply ends.
func addState(fm *fmesh.FMesh, name string, events ...string) error {
	c, err := component.New(name,
		component.WithDescription("state "+name),
		component.WithInputs(portEnter, portEvent),
		component.WithOutputs(events...),
		component.WithActivationFunc(component.Sequential(
			// A transition arrived: this state is now current. The state itself
			// records that on the mesh — it is the only one who knows for sure.
			component.When(component.HasSignalsOn(portEnter), func(_ context.Context, this *component.Component) error {
				sig := this.InputByName(portEnter).Signals().First()
				fm.Labels().Set(labelCurrent, name)
				fmt.Printf("   → entered %q on %q\n", name, sig.Labels().ValueOrDefault(labelEvent, ""))
				return nil
			}),
			// An event was fired while this state is current: send it down the
			// pipe of the same name, or refuse it if there is no such pipe.
			component.When(component.HasSignalsOn(portEvent), func(_ context.Context, this *component.Component) error {
				sig := this.InputByName(portEvent).Signals().First()
				event := sig.Labels().ValueOrDefault(labelEvent, "")
				out := this.OutputByName(event)
				if out == nil {
					return fmt.Errorf("%w: %q in %q", errEventNotAllowed, event, name)
				}
				return out.PutSignals(sig)
			}),
		)),
		dropInputsOnError,
	)
	if err != nil {
		return err
	}
	return fm.AddComponents(c)
}

// addGuard adds a component that sits on a transition's pipe and lets the
// signal through only when the predicate accepts it. A rejection is an
// activation error: the run stops, and the machine has not moved.
func addGuard(fm *fmesh.FMesh, name string, accept signal.Predicate) error {
	c, err := component.New(name,
		component.WithDescription("guard"),
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			sig := this.InputByName(portIn).Signals().First()
			if !accept(sig) {
				return fmt.Errorf("%w: %q", errGuardRejected, sig.Labels().ValueOrDefault(labelEvent, ""))
			}
			return this.OutputByName(portOut).PutSignals(sig)
		}),
		dropInputsOnError,
	)
	if err != nil {
		return err
	}
	return fm.AddComponents(c)
}

// dropInputsOnError makes a refusal final. A run that stops on an activation
// error returns before the cycle's inputs are drained, so the refused signal
// would still be sitting on the port, ready to replay on the next fire.
var dropInputsOnError = component.WithHooks(func(h *component.Hooks) {
	h.OnError(func(ctx context.Context, ac *component.ActivationContext) error {
		return ac.Component.ClearInputs(ctx)
	})
})

// current is the name of the state the machine is in: a label on the mesh,
// written by whichever state a transition last entered.
func current(fm *fmesh.FMesh) string {
	return fm.Labels().ValueOrDefault(labelCurrent, "")
}

// done reports whether the machine is in a final state: one with no way out.
func done(fm *fmesh.FMesh) bool {
	return fm.ComponentByName(current(fm)).Outputs().IsEmpty()
}

// fire drops an event into the machine and runs the mesh until it settles:
// one transition at most. The event is a labeled signal on the current
// state's event port; the optional signal carries payload and metadata for
// guards to look at. An unknown event is one no state has an exit for.
func fire(fm *fmesh.FMesh, event string, sig ...*signal.Signal) {
	if err := fireEvent(fm, event, sig...); err != nil {
		switch {
		case errors.Is(err, errUnknownEvent), errors.Is(err, errEventNotAllowed), errors.Is(err, errGuardRejected):
			fmt.Println("   ✗", err)
		default:
			fmt.Println("Fire failed:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("   state: %q\n\n", current(fm))
}

func fireEvent(fm *fmesh.FMesh, event string, sig ...*signal.Signal) error {
	known := fm.Components().AnyMatch(func(c *component.Component) bool {
		return c.InputByName(portEvent) != nil && c.OutputByName(event) != nil
	})
	if !known {
		return fmt.Errorf("%w: %q", errUnknownEvent, event)
	}

	s := signal.New(nil)
	if len(sig) > 0 && sig[0] != nil {
		s = sig[0]
	}
	s = s.WithLabel(labelEvent, event) // copy-on-write: the caller's signal is untouched

	if err := fm.ComponentByName(current(fm)).InputByName(portEvent).PutSignals(s); err != nil {
		return err
	}
	_, err := fm.Run(context.Background())
	// A refusal comes back as an activation error wrapped in the run's own
	// report; keep the reason, say which event met it where.
	for _, refusal := range []error{errEventNotAllowed, errGuardRejected} {
		if errors.Is(err, refusal) {
			return fmt.Errorf("%w: %q in %q", refusal, event, current(fm))
		}
	}
	return err
}
