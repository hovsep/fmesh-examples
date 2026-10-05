package main

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Dispatcher port names.
const (
	portCall  = "call"     // hall button → dispatcher
	portLEDUp = "led-up"   // dispatcher → hall up LEDs, led-up1…led-up5
	portLEDDn = "led-down" // dispatcher → hall down LEDs, led-down2…led-down6
	portToCab = "assign-"  // dispatcher → controller, one output per cab
)

// HallCall is someone on a landing who wants to go up or down.
type HallCall struct {
	Floor int `json:"floor"`
	Dir   int `json:"dir"`
}

// Assignment is a hall call and the cab sent to answer it.
type Assignment struct {
	HallCall
	Cab string `json:"cab"`
}

// DispatcherState is the calls waiting for a cab.
type DispatcherState struct {
	Calls []Assignment `json:"calls"`
}

// newDispatcher is the group controller: it gives every hall call to the cab
// that can get there first, lights the button's LED, and puts it out once
// that cab opens its doors on the floor.
func newDispatcher() (*component.Component, error) {
	outputs := []string{portTelemetry}
	for _, c := range cabs {
		outputs = append(outputs, portToCab+c)
	}
	return component.New("dispatcher",
		component.WithDescription("group controller: assigns each hall call to the cab that can get there first"),
		component.WithInputs(portCall, portStatus),
		component.WithOutputs(outputs...),
		component.WithIndexedOutputs(portLEDUp, 1, floors-1),
		component.WithIndexedOutputs(portLEDDn, 2, floors),
		component.WithInitialState(func(s component.State) {
			s.Set("cabs", map[string]CabStatus{})
			s.Set("pending", map[HallCall]string{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			known := this.State().Get("cabs").(map[string]CabStatus)
			pending := this.State().Get("pending").(map[HallCall]string)

			for _, sig := range this.InputByName(portStatus).Signals().All() {
				st, err := sig.As[CabStatus]()
				if err != nil {
					return err
				}
				known[st.Cab] = st
				for _, dir := range []int{+1, -1} {
					call := HallCall{Floor: st.Served, Dir: dir}
					if st.Served > 0 && pending[call] == st.Cab {
						delete(pending, call)
						if err := this.OutputByName(hallLEDPort(call)).PutSignals(signal.New(false)); err != nil {
							return err
						}
					}
				}
			}

			for _, sig := range this.InputByName(portCall).Signals().All() {
				call, err := sig.As[HallCall]()
				if err != nil {
					return err
				}
				if _, ok := pending[call]; ok {
					continue
				}
				best := slices.MinFunc(cabs, func(a, b string) int { return cmp.Compare(cost(known[a], call), cost(known[b], call)) })
				pending[call] = best
				if err := this.OutputByName(hallLEDPort(call)).PutSignals(signal.New(true)); err != nil {
					return err
				}
				if err := this.OutputByName(portToCab + best).PutSignals(signal.New(call.Floor)); err != nil {
					return err
				}
			}

			calls := []Assignment{}
			for call, cab := range pending {
				calls = append(calls, Assignment{HallCall: call, Cab: cab})
			}
			slices.SortFunc(calls, func(a, b Assignment) int { return cmp.Or(cmp.Compare(a.Floor, b.Floor), cmp.Compare(a.Dir, b.Dir)) })
			return report(this, DispatcherState{Calls: calls})
		}),
	)
}

// cost estimates how long a cab would take to answer a call, in floors.
func cost(st CabStatus, call HallCall) float64 {
	away := floorPos(call.Floor) - st.Pos
	c := math.Abs(away)/floorHeight + 2*float64(len(st.Stops))
	if st.Dir != 0 && away*float64(st.Dir) < 0 {
		c += 2 * floors // it is heading away and must turn around first
	}
	return c
}

func hallLEDPort(call HallCall) string {
	if call.Dir > 0 {
		return fmt.Sprint(portLEDUp, call.Floor)
	}
	return fmt.Sprint(portLEDDn, call.Floor)
}
