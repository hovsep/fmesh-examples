// This file holds the "waiting-list" component: the queue at the door. It
// remembers who is waiting, shows the list to the host every minute and lets
// impatient parties leave.
package main

import (
	"context"
	"slices"

	"github.com/hovsep/fmesh/component"
)

// Key of the waiting list's state: the parties waiting, in arrival order.
const stateWaiting = "waiting"

// newWaitingList creates the queue. A party that has waited longer than
// patienceMinutes gives up and is reported as lost.
func newWaitingList(patienceMinutes int) (*component.Component, error) {
	return component.New(waitingListName,
		component.WithDescription("the queue at the door: parties wait for a table or give up"),
		component.WithInputs(portParty, portSeated, portTick),
		component.WithOutputs(portWaiting, portLost),
		component.WithInitialState(func(state component.State) {
			state.Set(stateWaiting, []Party{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			waiting := this.State().Get(stateWaiting).([]Party)
			defer func() { this.State().Set(stateWaiting, waiting) }()

			// New arrivals join the end of the queue.
			arrived, err := payloads[Party](this.InputByName(portParty))
			if err != nil {
				return err
			}
			waiting = append(waiting, arrived...)

			// Parties the host has seated leave the queue.
			seated, err := payloads[Party](this.InputByName(portSeated))
			if err != nil {
				return err
			}
			waiting = removeParties(waiting, seated)

			// On a tick: impatient parties leave, then the host gets the current list.
			if !this.InputByName(portTick).HasSignals() {
				return nil
			}
			minute, err := firstPayload[int](this.InputByName(portTick))
			if err != nil {
				return err
			}
			var lost []Party
			waiting, lost = splitImpatient(waiting, minute, patienceMinutes)
			for _, party := range lost {
				if err := this.OutputByName(portLost).PutPayloads(party); err != nil {
					return err
				}
			}
			for _, party := range waiting {
				if err := this.OutputByName(portWaiting).PutPayloads(party); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// removeParties returns the waiting list without the given parties (matched by ID).
func removeParties(waiting []Party, gone []Party) []Party {
	return slices.DeleteFunc(slices.Clone(waiting), func(candidate Party) bool {
		return slices.ContainsFunc(gone, func(g Party) bool { return g.ID == candidate.ID })
	})
}

// splitImpatient separates the parties still willing to wait from the ones
// that have waited longer than their patience and leave.
func splitImpatient(waiting []Party, minute int, patienceMinutes int) (staying []Party, lost []Party) {
	for _, party := range waiting {
		if minute-party.ArrivedAt > patienceMinutes {
			lost = append(lost, party)
		} else {
			staying = append(staying, party)
		}
	}
	return staying, lost
}
