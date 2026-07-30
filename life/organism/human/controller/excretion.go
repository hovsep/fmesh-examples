package controller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Excretion command verbs.
const (
	VerbUrinate  = "urinate"
	VerbDefecate = "defecate"
)

// Excretion controller state: whether a voiding is queued for the next tick,
// plus the lifetime counts.
const (
	PendingUrination  string = "pending_urination"
	PendingDefecation string = "pending_defecation"
	TotalUrinations   string = "total_urinations"
	TotalDefecations  string = "total_defecations"
)

//@TODO: check if this component is only cmd executor or it also manages feelings\intentions over time (e.g. human must want to urinate after certain time)
//if this is not the best place to generate intentions\feeling on excretion topic - let's implement it in proper component - some regulation

// GetExcretion returns the excretion controller.
//
// It expresses only the intent to void. How much actually leaves the body
// depends on how full the bladder and bowel are, which the organs downstream
// decide -- so this stays a pure command surface.
func GetExcretion() (*component.Component, error) {
	c, err := component.New("controller:excretion",
		component.WithDescription("Turns voiding commands into excretion intent"),
		component.WithInputs(simulation.TimePort, simulation.ControlPort),
		component.WithOutputs("urine_out", "feces_out"),
		component.WithActivationFunc(component.Sequential(
			acceptExcretionCommands,
			emitExcretionIntent,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(PendingUrination, false)
			state.Set(PendingDefecation, false)
			state.Set(TotalUrinations, 0.0)
			state.Set(TotalDefecations, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:excretion: %w", err)
	}
	return c, nil
}

func acceptExcretionCommands(this *component.Component) error {
	return command.ForEach(this, simulation.ControlPort, func(name string, _ *meta.Scalars) error {
		switch command.Verb(name) {
		case VerbUrinate:
			// A flag rather than a counter: asking twice in one tick still means
			// one voiding, since you cannot empty an empty bladder again.
			this.State().Set(PendingUrination, true)
			this.State().Update(TotalUrinations, func(v any) any { return v.(float64) + 1 })
		case VerbDefecate:
			this.State().Set(PendingDefecation, true)
			this.State().Update(TotalDefecations, func(v any) any { return v.(float64) + 1 })
		default:
			this.Logger().Printf("unknown excretion command %q\n", name)
		}
		return nil
	})
}

func emitExcretionIntent(this *component.Component) error {
	if !this.InputByName(simulation.TimePort).HasSignals() {
		return nil
	}

	if this.State().Get(PendingUrination).(bool) {
		this.State().Set(PendingUrination, false)
		if err := this.OutputByName("urine_out").PutSignals(voidIntent()); err != nil {
			return err
		}
	}

	if this.State().Get(PendingDefecation).(bool) {
		this.State().Set(PendingDefecation, false)
		if err := this.OutputByName("feces_out").PutSignals(voidIntent()); err != nil {
			return err
		}
	}
	return nil
}

func voidIntent() *signal.Signal {
	return signal.New("void").WithLabel("category", "excretion")
}
