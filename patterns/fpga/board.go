package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The board: a fixed number of identical, generic logic cells, a clock, and
// pins. Flashing a design configures cells (each gets a truth table, and a
// flip-flop if it is a register) and routes the wires between them. The
// cells a design does not use stay on the board, blank and unconnected.

// BoardCells is how many logic cells the board has.
const BoardCells = 16

// Port names.
const (
	portEdge  = "edge"  // clock: one rising edge
	portTick  = "tick"  // clock → pins and registers
	portClk   = "clk"   // a pin's or a register's clock input
	portSet   = "set"   // an input pin's value, set from outside
	portIn    = "in"    // an output pin's input
	portOut   = "out"   // the value a pin or cell drives
	stateQ    = "q"     // a register's current value
	stateNext = "next"  // what it will take on the next clock edge
	stateVal  = "value" // a pin's value
)

func cellName(i int) string     { return fmt.Sprintf("cell-%02d", i) }
func inPinName(s string) string { return "in:" + s }
func outPin(s string) string    { return "out:" + s }
func lutInput(j int) string     { return fmt.Sprintf("i%d", j) }

// Flash programs a fresh board with the design and returns it as a mesh.
// Nothing about the circuit is in the code: change the .v file and the same
// board becomes a different circuit.
func Flash(d *Design) (*fmesh.FMesh, error) {
	if len(d.Cells) > BoardCells {
		return nil, fmt.Errorf("%s needs %d cells, the board has %d", d.Name, len(d.Cells), BoardCells)
	}

	fm, err := fmesh.New("fpga "+d.Name,
		fmesh.WithDescription(fmt.Sprintf("a %d-cell board flashed with %s", BoardCells, d.Name)),
	)
	if err != nil {
		return nil, err
	}

	clock, err := newClock()
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(clock); err != nil {
		return nil, err
	}

	// driver: which component drives each signal.
	driver := map[string]*component.Component{}
	for _, in := range d.Inputs {
		pin, err := newInputPin(in)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(pin); err != nil {
			return nil, err
		}
		if err := clock.OutputByName(portTick).PipeTo(pin.InputByName(portClk)); err != nil {
			return nil, err
		}
		driver[in] = pin
	}

	// Place: design cell i goes to board cell i. Blank cells fill the rest.
	for i := range BoardCells {
		var cell *component.Component
		if i < len(d.Cells) {
			cell, err = newCell(cellName(i), d.Cells[i])
			driver[d.Cells[i].Signal] = cell
		} else {
			cell, err = component.New(cellName(i),
				component.WithDescription("blank"),
				component.WithActivationFunc(func(context.Context, *component.Component) error { return nil }),
			)
		}
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(cell); err != nil {
			return nil, err
		}
	}

	// Route: a wire from the driver of every signal a cell reads.
	for _, c := range d.Cells {
		cell := driver[c.Signal]
		for j, s := range c.Inputs {
			if err := driver[s].OutputByName(portOut).PipeTo(cell.InputByName(lutInput(j))); err != nil {
				return nil, err
			}
		}
		if c.Registered {
			if err := clock.OutputByName(portTick).PipeTo(cell.InputByName(portClk)); err != nil {
				return nil, err
			}
		}
	}

	for _, out := range d.Outputs {
		pin, err := newOutputPin(out)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(pin); err != nil {
			return nil, err
		}
		if err := driver[out].OutputByName(portOut).PipeTo(pin.InputByName(portIn)); err != nil {
			return nil, err
		}
	}
	return fm, nil
}

// Tick is one rising clock edge: registers take their next value, the pins
// present theirs, and the combinational logic settles. When Run returns,
// every output is stable.
func Tick(fm *fmesh.FMesh, inputs map[string]bool) error {
	for name, v := range inputs {
		pin := fm.ComponentByName(inPinName(name))
		if pin == nil {
			return fmt.Errorf("no input pin %s", name)
		}
		if err := pin.InputByName(portSet).PutSignals(signal.New(v)); err != nil {
			return err
		}
	}
	if err := fm.ComponentByName("clock").InputByName(portEdge).PutSignals(signal.New(true)); err != nil {
		return err
	}
	_, err := fm.Run(context.Background())
	return err
}

// Read returns the value on an output pin.
func Read(fm *fmesh.FMesh, name string) bool {
	return fm.ComponentByName(outPin(name)).State().GetOrDefault(stateVal, false).(bool)
}

func newClock() (*component.Component, error) {
	return component.New("clock",
		component.WithDescription("the clock net: one edge per tick, to every pin and register"),
		component.WithInputs(portEdge),
		component.WithOutputs(portTick),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(portTick).PutSignals(signal.New(true))
		}),
	)
}

// newInputPin holds the level set from outside and drives it on the clock,
// so every input changes in step with the registers.
func newInputPin(name string) (*component.Component, error) {
	return component.New(inPinName(name),
		component.WithDescription("input pin"),
		component.WithInputs(portSet, portClk),
		component.WithOutputs(portOut),
		component.WithInitialState(func(s component.State) { s.Set(stateVal, false) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			if this.InputByName(portSet).HasSignals() {
				this.State().Set(stateVal, this.InputByName(portSet).Signals().FirstPayloadOrDefault(false))
			}
			if !this.InputByName(portClk).HasSignals() {
				return nil
			}
			return this.OutputByName(portOut).PutSignals(signal.New(this.State().Get(stateVal).(bool)))
		}),
	)
}

func newOutputPin(name string) (*component.Component, error) {
	return component.New(outPin(name),
		component.WithDescription("output pin"),
		component.WithInputs(portIn),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			this.State().Set(stateVal, this.InputByName(portIn).Signals().FirstPayloadOrDefault(false))
			return nil
		}),
	)
}

// newCell is one configured logic cell: a look-up table, and for a register
// a flip-flop after it.
//
// A combinational cell waits for all its inputs and drives the table's
// answer. A register drives its stored value on the clock edge, then waits
// for its inputs and stores what it will take on the next edge.
func newCell(name string, c Cell) (*component.Component, error) {
	inputs := make([]string, len(c.Inputs))
	for j := range c.Inputs {
		inputs[j] = lutInput(j)
	}
	lookup := func(this *component.Component) (bool, error) {
		bits := 0
		for j, in := range inputs {
			v, err := this.InputByName(in).Signals().FirstAs[bool]()
			if err != nil {
				return false, err
			}
			if v {
				bits |= 1 << j
			}
		}
		return c.Table[bits], nil
	}

	description := fmt.Sprintf("%s = %s · LUT %s", c.Signal, c.Source, c.TableString())
	ports := inputs
	if c.Registered {
		description = fmt.Sprintf("%s <= %s · LUT %s + flip-flop", c.Signal, c.Source, c.TableString())
		ports = append([]string{portClk}, inputs...)
	}

	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(ports...),
		component.WithOutputs(portOut),
		component.WithInitialState(func(s component.State) {
			s.Set(stateQ, false)
			s.Set(stateNext, false)
		}),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if c.Registered && this.InputByName(portClk).HasSignals() {
				q := this.State().Get(stateNext).(bool)
				this.State().Set(stateQ, q)
				return this.OutputByName(portOut).PutSignals(signal.New(q))
			}

			if err := component.RequireInputs(inputs...)(ctx, this); err != nil {
				return err
			}
			v, err := lookup(this)
			if err != nil {
				return err
			}
			if c.Registered {
				this.State().Set(stateNext, v)
				return nil
			}
			return this.OutputByName(portOut).PutSignals(signal.New(v))
		}),
	)
}
