package main

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The doors: the car door and its operator, the light curtain across the
// doorway, and on every landing a door with its lock. The car door drags
// the landing door it stands at; every lock and the car door contact sit
// in the safety chain.

// Door port names.
const (
	portDoorCmd = "door-cmd" // controller → door operator: open, close or stop
	portDoor    = "door"     // door operator → controller, landing doors
	portBlock   = "block"    // UI → light curtain: someone steps through the doorway
	portBeam    = "beam"     // light curtain → controller: true while interrupted
	portPanel   = "panel"    // landing door → its lock: how far it is open
)

const (
	doorOpen  = "open"
	doorClose = "close"
	doorStop  = "stop"

	doorOpenTime  = 1.5  // s from shut to fully open
	doorCloseTime = 2.0  // s from fully open to shut
	doorZone      = 0.05 // m: how close to a landing the car door can drag the landing door
	blockTime     = 1.5  // s someone stands in the doorway
)

// DoorState is how far the car door is open.
type DoorState struct {
	Opening float64 `json:"opening"` // 0 shut … 1 fully open
	Open    bool    `json:"open"`
	Closed  bool    `json:"closed"`
}

// newDoorOperator is the motor on top of the car that slides its door, and
// the door contact that tells the safety chain the door is shut.
func newDoorOperator(c string) (*component.Component, error) {
	return component.New("door-operator-"+c,
		component.WithDescription("car door operator: slides the door open or shut, with the car door contact"),
		component.WithInputs(portDoorCmd),
		component.WithOutputs(portDoor, portContact, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("opening", 0.0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			opening := this.State().Get("opening").(float64)
			switch latest(this.InputByName(portDoorCmd), doorStop) {
			case doorOpen:
				opening = min(1, opening+dt/doorOpenTime)
			case doorClose:
				opening = max(0, opening-dt/doorCloseTime)
			}
			this.State().Set("opening", opening)
			st := DoorState{Opening: opening, Open: opening >= 1, Closed: opening <= 0}
			if err := this.OutputByName(portDoor).PutSignals(signal.New(st)); err != nil {
				return err
			}
			if err := this.OutputByName(portContact).PutSignals(signal.New(Contact{Name: "car door", Closed: st.Closed})); err != nil {
				return err
			}
			return report(this, st)
		}),
	)
}

// BeamState is the light curtain.
type BeamState struct {
	Blocked bool `json:"blocked"`
}

// newLightCurtain is the row of infrared beams across the car doorway:
// someone stepping through it keeps the doors from closing on them.
func newLightCurtain(c string) (*component.Component, error) {
	return component.New("light-curtain-"+c,
		component.WithDescription("infrared beams across the doorway: interrupted while someone steps through"),
		component.WithInputs(portTick, portBlock),
		component.WithOutputs(portBeam, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("left", 0.0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			left := this.State().Get("left").(float64)
			if this.InputByName(portBlock).HasSignals() {
				left = blockTime
			} else if this.InputByName(portTick).HasSignals() {
				left = max(0, left-dt)
			}
			this.State().Set("left", left)
			blocked := left > 0
			if err := this.OutputByName(portBeam).PutSignals(signal.New(blocked)); err != nil {
				return err
			}
			return report(this, BeamState{Blocked: blocked})
		}),
	)
}

// OpeningState is a landing door.
type OpeningState struct {
	Opening float64 `json:"opening"`
}

// newLandingDoor is the door on one landing of one shaft. It has no motor:
// the car door drags it along when the car stands at this floor, and it
// shuts by itself otherwise.
func newLandingDoor(c string, f int) (*component.Component, error) {
	return component.New(fmt.Sprintf("landing-door-%s-%d", c, f),
		component.WithDescription(fmt.Sprintf("landing door of shaft %s on floor %d, moved by the car door", c, f)),
		component.WithInputs(portDoor, portPosition),
		component.WithOutputs(portPanel, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("pos", 0.0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			pos := latest(this.InputByName(portPosition), Motion{Pos: this.State().Get("pos").(float64)}).Pos
			this.State().Set("pos", pos)
			if !this.InputByName(portDoor).HasSignals() {
				return nil
			}
			opening := 0.0
			if math.Abs(pos-floorPos(f)) < doorZone {
				opening = latest(this.InputByName(portDoor), DoorState{}).Opening
			}
			if err := this.OutputByName(portPanel).PutSignals(signal.New(opening)); err != nil {
				return err
			}
			return report(this, OpeningState{Opening: opening})
		}),
	)
}

// newDoorLock is the interlock of one landing door: its contact is closed
// only while the door is shut and locked.
func newDoorLock(c string, f int) (*component.Component, error) {
	return component.New(fmt.Sprintf("door-lock-%s-%d", c, f),
		component.WithDescription(fmt.Sprintf("interlock of the shaft %s landing door on floor %d", c, f)),
		component.WithInputs(portPanel),
		component.WithOutputs(portContact, portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			locked := latest(this.InputByName(portPanel), 0.0) <= 0
			if err := this.OutputByName(portContact).PutSignals(signal.New(Contact{Name: landingContact(f), Closed: locked})); err != nil {
				return err
			}
			return report(this, ContactState{Closed: locked})
		}),
	)
}

func landingContact(f int) string { return fmt.Sprintf("landing door %d", f) }
