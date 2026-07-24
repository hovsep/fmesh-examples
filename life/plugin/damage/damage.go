// Package damage is a reusable component plugin: attach it to any organ and that
// organ accumulates damage, from slow aging and from external insults, until it
// crosses a critical threshold and fails.
//
// It is the clearest example of the project's "if it works for many organs, make
// it a plugin" principle: one small plugin gives every organ a damage level, a
// way to be hurt from outside (a toxin, a starved reservoir), a failure signal,
// and an observable level -- with no per-organ code beyond attaching it and
// checking Failed.
package damage

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Ports and state the plugin adds to its host component.
const (
	// InputPort receives external damage: each signal's payload is an amount
	// (0..1 scale) added this tick. A starved reservoir or an inhaled toxin
	// emits here.
	InputPort = "damage"

	// LevelOutput publishes the current damage level (0..1) for observation.
	LevelOutput = "damage_level"

	stateLevel  common.State = "damage_level"
	stateFailed common.State = "damage_failed"
)

const (
	// CriticalLevel is the damage at which an organ fails.
	CriticalLevel = 1.0

	// DefaultAgingRatePerSec ramps an organ to failure over roughly 90 years,
	// the background aging every organ shares unless told otherwise.
	DefaultAgingRatePerSec = 1.0 / (90 * 365 * 24 * 3600)
)

// Config parameterises the plugin so each organ can age at its own pace and
// name its own failure.
type Config struct {
	// Organ names the host, so its failure signal and telemetry are identifiable.
	Organ string

	// AgingRatePerSec is the background damage accrued per second. Zero uses the
	// default ~90-year rate.
	AgingRatePerSec float64
}

// Damage is the plugin instance.
type Damage struct {
	organ           string
	agingRatePerSec float64
}

// New builds a damage plugin for an organ.
func New(cfg Config) *Damage {
	rate := cfg.AgingRatePerSec
	if rate <= 0 {
		rate = DefaultAgingRatePerSec
	}
	return &Damage{organ: cfg.Organ, agingRatePerSec: rate}
}

func (d *Damage) GetName() string { return "Damage" }

func (d *Damage) Init(c *component.Component) error {
	c.State().Set(stateLevel, 0.0)
	c.State().Set(stateFailed, false)

	if err := c.AddInputs(InputPort); err != nil {
		return err
	}
	if err := c.AddOutputs(LevelOutput, d.failureOutput()); err != nil {
		return err
	}

	c.SetupHooks(func(hooks *component.Hooks) {
		hooks.OnActivation(d.onActivation)
	})
	return nil
}

func (d *Damage) failureOutput() string { return d.organ + "_failure" }

// onActivation folds in this cycle's insults and the tick's aging, then
// publishes the level and, once critical, latches failure and announces it.
func (d *Damage) onActivation(this *component.Component) error {
	// External insults arrive as discrete amounts on their own mesh cycle. Read
	// and clear them so they are counted exactly once, even when the host keeps
	// its inputs waiting for others (e.g. the lungs).
	var insult float64
	if in := this.InputByName(InputPort); in != nil && in.HasSignals() {
		_ = in.Signals().ForEach(func(sig *signal.Signal) error {
			insult += helper.AsF64OrDefault(sig, 0)
			return nil
		})
		in.Clear()
	}

	// Aging accrues once per tick, scaled by the tick's real duration.
	var aging float64
	if tick := this.InputByName(common.TimePort); tick != nil && tick.HasSignals() {
		if dt, err := helper.TickDurationInSec(tick.Signals().First()); err == nil {
			aging = d.agingRatePerSec * dt
		}
	}

	if insult == 0 && aging == 0 {
		return nil
	}

	level := helper.Clamp(this.State().Get(stateLevel).(float64)+insult+aging, 0, CriticalLevel)
	this.State().Set(stateLevel, level)

	if err := this.OutputByName(LevelOutput).PutPayloads(level); err != nil {
		return err
	}

	if level >= CriticalLevel && !this.State().Get(stateFailed).(bool) {
		this.State().Set(stateFailed, true)
		return this.OutputByName(d.failureOutput()).PutSignals(
			signal.New(d.organ+"_failure").WithLabel("organ", d.organ))
	}
	return nil
}

// Inflict adds damage to a component directly, latching failure if it crosses
// critical. It is the immediate counterpart to the gradual damage that flows in
// over the mesh -- useful for "what if this organ were injured" and for tests.
// Must be called from the simulation goroutine (e.g. a hook), like any other
// mesh state mutation.
func Inflict(c *component.Component, amount float64) {
	if c == nil || amount <= 0 {
		return
	}
	level, _ := c.State().Get(stateLevel).(float64)
	level = helper.Clamp(level+amount, 0, CriticalLevel)
	c.State().Set(stateLevel, level)
	if level >= CriticalLevel {
		c.State().Set(stateFailed, true)
	}
}

// Level returns a component's current damage (0..1), or 0 if it has no plugin.
func Level(c *component.Component) float64 {
	if c == nil {
		return 0
	}
	level, _ := c.State().Get(stateLevel).(float64)
	return level
}

// Failed reports whether a component's damage has reached critical. Organs check
// this at the top of their activation and flatline when it is true, which is how
// an organ "collapses".
func Failed(c *component.Component) bool {
	if c == nil {
		return false
	}
	failed, _ := c.State().Get(stateFailed).(bool)
	return failed
}

// FailureOutput returns the name of the failure output port for an organ, for wiring.
func FailureOutput(organ string) string { return organ + "_failure" }

// FlatlineWhenFailed composes an organ's activation phases so that, once the
// organ has failed, it produces nothing at all -- the mechanism by which a
// collapsed organ stops driving the body. The damage plugin's own hook keeps
// running (aging and telemetry continue), so this only silences the organ's
// output, not its bookkeeping.
func FlatlineWhenFailed(funcs ...component.ActivationFunc) component.ActivationFunc {
	return func(this *component.Component) error {
		if Failed(this) {
			return nil
		}
		for _, f := range funcs {
			if err := f(this); err != nil {
				return err
			}
		}
		return nil
	}
}
