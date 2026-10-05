package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// Two elevators in a six-floor building, every part of them a component:
// the buttons, LEDs, lanterns and doors on every landing, the buttons,
// display, light, door and light curtain of every car, and the whole hoist
// from the drive to the counterweight, wired through safety chains.
//
// A browser page draws the building from the mesh's state and sends every
// click back in as a signal on a button. Every Run of the mesh is 50 ms of
// the building's life: one pulse goes into the clock, the tick ripples
// through the parts, and the panel gathers what they report into one frame.
//
// Run: go run .   then open http://localhost:8080

// starts is the floor each car waits on when the building opens.
var starts = map[string]int{"A": 1, "B": floors}

func main() {
	addr := flag.String("addr", "localhost:8080", "where to serve the page")
	demo := flag.Bool("demo", false, "people keep pressing buttons on their own")
	flag.Parse()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	if err := serve(fm, *addr, *demo); err != nil {
		fmt.Println("Elevator failed:", err)
		os.Exit(1)
	}
}

// Input is something done to the building from outside: a button pressed,
// someone stepping through a doorway, a technician's switch.
type Input struct {
	Part string `json:"part"`
	Port string `json:"port"`
}

// uiPorts are the only inputs a visitor can reach.
var uiPorts = map[string]bool{portPress: true, portBlock: true, portTest: true, portReset: true}

// target finds the port an input goes to.
func target(fm *fmesh.FMesh, in Input) (*port.Port, error) {
	c := fm.ComponentByName(in.Part)
	if c == nil || !uiPorts[in.Port] {
		return nil, fmt.Errorf("nothing to press at %s/%s", in.Part, in.Port)
	}
	p := c.InputByName(in.Port)
	if p == nil {
		return nil, fmt.Errorf("nothing to press at %s/%s", in.Part, in.Port)
	}
	return p, nil
}

// step runs 50 ms of the building's life: the inputs and one pulse go in,
// the frame of every part's state comes out, with the number of cycles the
// tick took to settle.
func step(fm *fmesh.FMesh, inputs []Input) (Frame, int, error) {
	for _, in := range inputs {
		p, err := target(fm, in)
		if err != nil {
			return nil, 0, err
		}
		if err := p.PutSignals(signal.New(true)); err != nil {
			return nil, 0, err
		}
	}
	if err := fm.ComponentByName("clock").InputByName(portPulse).PutSignals(signal.New(true)); err != nil {
		return nil, 0, err
	}
	info, err := fm.Run(context.Background())
	if err != nil {
		return nil, 0, err
	}
	frame := latest(fm.ComponentByName("panel").OutputByName(portFrame), Frame{})
	// The last cycle is the quiet one in which nothing activated.
	return frame, info.Cycles.Last().Number() - 1, nil
}

// mesh collects components and the pipes between them while the building
// is put together.
type mesh struct {
	parts []*component.Component
	pipes []port.Pipe
}

func (m *mesh) add(c *component.Component, err error) *component.Component {
	if err != nil {
		panic(err) // only a mistake in this file can get here
	}
	m.parts = append(m.parts, c)
	return c
}

// pipe wires one output to any number of inputs.
func (m *mesh) pipe(from *component.Component, out string, to ...*component.Component) {
	m.pipeTo(from, out, "", to...)
}

// pipeTo is pipe for an input whose name differs from the output's.
func (m *mesh) pipeTo(from *component.Component, out, in string, to ...*component.Component) {
	if in == "" {
		in = out
	}
	for _, t := range to {
		m.pipes = append(m.pipes, port.Pipe{From: from.OutputByName(out), To: t.InputByName(in)})
	}
}

// getMesh puts the building together.
func getMesh() (fm *fmesh.FMesh, err error) {
	defer func() {
		if r := recover(); r != nil {
			fm, err = nil, fmt.Errorf("%v", r)
		}
	}()
	m := &mesh{}

	clock := m.add(newClock())
	panel := m.add(newPanel())
	dispatcher := m.add(newDispatcher())

	// The landings: an up and a down button with their LEDs on every floor
	// (no up on the top floor, no down in the lobby).
	for f := 1; f <= floors; f++ {
		for _, h := range []struct {
			dir  int
			name string
			led  string
		}{{+1, "up", portLEDUp}, {-1, "down", portLEDDn}} {
			if (h.dir > 0 && f == floors) || (h.dir < 0 && f == 1) {
				continue
			}
			button := m.add(newButton(fmt.Sprintf("hall-%s-%d", h.name, f),
				fmt.Sprintf("hall call button %s on floor %d", h.name, f), HallCall{Floor: f, Dir: h.dir}))
			led := m.add(newLamp(fmt.Sprintf("hall-%s-led-%d", h.name, f), fmt.Sprintf("LED of the %s button on floor %d", h.name, f)))
			m.pipeTo(button, portPressed, portCall, dispatcher)
			m.pipeTo(dispatcher, fmt.Sprint(h.led, f), portSet, led)
		}
	}

	for _, c := range cabs {
		buildCab(m, c, clock, dispatcher)
	}

	// Every part reports to the panel.
	for _, c := range m.parts {
		if c.OutputByName(portTelemetry) != nil {
			m.pipe(c, portTelemetry, panel)
		}
	}

	fm, err = fmesh.New("elevator",
		fmesh.WithDescription("two elevators in a six-floor building, every part a component, 50 ms per run"),
		fmesh.WithCyclesHistoryLimit(1),
	)
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(m.parts...); err != nil {
		return nil, err
	}
	if err := port.MultiPipe(m.pipes...); err != nil {
		return nil, err
	}
	return fm, nil
}

// buildCab puts one elevator together: its car with everything inside, its
// hoist, its doors on every landing and its safety chain.
func buildCab(m *mesh, c string, clock, dispatcher *component.Component) {
	name := func(part string) string { return fmt.Sprintf("car-%s-%s", c, part) }
	start := starts[c]

	controller := m.add(newController(c, start))
	m.pipe(clock, portTick, controller)
	m.pipeTo(controller, portStatus, portStatus, dispatcher)
	m.pipeTo(dispatcher, portToCab+c, portAssign, controller)

	// Inside the car.
	for f := 1; f <= floors; f++ {
		button := m.add(newButton(name(fmt.Sprint("button-", f)), fmt.Sprintf("car %s button for floor %d", c, f), f))
		led := m.add(newLamp(name(fmt.Sprint("led-", f)), fmt.Sprintf("LED of the car %s button for floor %d", c, f)))
		m.pipeTo(button, portPressed, portCarCall, controller)
		m.pipeTo(controller, ledPort(f), portSet, led)
	}
	for _, cmd := range []string{doorOpen, doorClose} {
		button := m.add(newButton(name(cmd), fmt.Sprintf("door %s button in car %s", cmd, c), cmd))
		m.pipeTo(button, portPressed, portDoorBtn, controller)
	}
	display := m.add(newIndicator(name("display"), "floor display in car "+c, 0))
	light := m.add(newLamp(name("light"), "ceiling light in car "+c))
	m.pipeTo(controller, portDisplay, portShow, display)
	m.pipeTo(controller, portLight, portSet, light)

	// The hoist.
	drive := m.add(newDrive(c))
	motor := m.add(newMotor(c))
	brake := m.add(newBrake(c))
	encoder := m.add(newEncoder(c))
	sheave := m.add(newSheave(c))
	car := m.add(newCar(c, start))
	counterweight := m.add(newCounterweight(c, start))
	governor := m.add(newGovernor(c))
	gear := m.add(newSafetyGear(c))
	sensor := m.add(newPositionSensor(c))
	top := m.add(newLimitSwitch(c, "top", func(pos float64) bool { return pos > travel+0.3 }))
	bottom := m.add(newLimitSwitch(c, "bottom", func(pos float64) bool { return pos < -0.3 }))

	m.pipe(controller, portRef, drive)
	m.pipe(controller, portLift, brake)
	m.pipe(drive, portPower, motor)
	m.pipe(brake, portBrake, motor, controller)
	m.pipe(motor, portShaft, encoder, sheave)
	m.pipeTo(encoder, portSpeed, portFeedback, drive)
	m.pipe(sheave, portRope, car, counterweight)
	m.pipe(gear, portClamp, car)
	m.pipe(car, portPosition, sensor, governor, top, bottom)
	m.pipe(sensor, portReading, controller)
	m.pipe(governor, portContact, gear)

	// The doors.
	operator := m.add(newDoorOperator(c))
	curtain := m.add(newLightCurtain(c))
	m.pipe(controller, portDoorCmd, operator)
	m.pipe(operator, portDoor, controller)
	m.pipe(clock, portTick, curtain)
	m.pipe(curtain, portBeam, controller)

	// The safety chain runs through every contact of the shaft.
	contacts := []string{"car door", "governor", "top limit", "bottom limit"}
	for f := 1; f <= floors; f++ {
		contacts = append(contacts, landingContact(f))
	}
	chain := m.add(newSafetyChain(c, contacts))
	m.pipe(operator, portContact, chain)
	m.pipe(governor, portContact, chain)
	m.pipe(top, portContact, chain)
	m.pipe(bottom, portContact, chain)
	m.pipe(chain, portChain, drive, brake, controller)

	// Every landing of the shaft: a door the car door drags, its lock, and
	// a lantern showing where the car is.
	for f := 1; f <= floors; f++ {
		door := m.add(newLandingDoor(c, f))
		lock := m.add(newDoorLock(c, f))
		lantern := m.add(newIndicator(fmt.Sprintf("lantern-%s-%d", c, f), fmt.Sprintf("hall lantern of shaft %s on floor %d", c, f), f))
		m.pipe(operator, portDoor, door)
		m.pipe(car, portPosition, door)
		m.pipe(door, portPanel, lock)
		m.pipe(lock, portContact, chain)
		m.pipeTo(controller, portIndicator, portShow, lantern)
	}
}
