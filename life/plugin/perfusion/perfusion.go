// Package perfusion is a reusable component plugin: attach it to an organ and
// that organ is connected to the bloodstream. It reads what the blood is
// delivering, draws its own oxygen out of it, and returns carbon dioxide -- with
// no per-organ code beyond declaring how much oxygen the organ needs.
//
// Every tissue in a body is perfused, so this is the second thing (after
// damage) that belongs to organs in general rather than to any one of them.
// Before it, the brain and the heart each hand-rolled the same three steps:
// latch the arterial signal, emit an oxygen demand, emit a carbon dioxide
// return. Everything else -- kidney, gut, skin, muscle, diaphragm -- simply was
// not connected to the blood at all, so nothing but the brain and the heart
// could suffer for a supply that failed.
//
// The resting oxygen demands below are the published figures for each organ,
// which means the body's total oxygen consumption is not a constant anyone typed
// in: it is the sum of what its parts ask for.
package perfusion

import (
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

// Ports the plugin adds to its host. Both are called "blood" -- one is the
// arterial supply arriving, the other is what the organ puts back -- and they
// live in different collections, so the names do not collide.
const (
	SupplyPort = "blood"
	ReturnPort = "blood"
)

// State the plugin keeps on its host.
const (
	stateSaO2        common.State = "perfusion_sao2"
	statePaO2        common.State = "perfusion_pao2"
	statePaCO2       common.State = "perfusion_paco2"
	stateContent     common.State = "perfusion_content"
	stateGlucose     common.State = "perfusion_glucose"
	stateO2PerMinute common.State = "perfusion_o2_per_minute"
	stateOnset       common.State = "perfusion_onset"
	stateFail        common.State = "perfusion_fail"
)

// Default thresholds, in arterial oxygen content (mL of oxygen per dL of blood).
//
// Normal arterial content is about 20 mL/dL. Below roughly 12 a tissue starts to
// struggle; by 5 it cannot sustain itself. Organs that tolerate less say so.
const (
	DefaultContentOnset = 12.0
	DefaultContentFail  = 5.0

	// DefaultRQ is the respiratory quotient: molecules of carbon dioxide
	// produced per molecule of oxygen consumed. 0.8 is the usual mixed diet
	// value (1.0 burning pure carbohydrate, 0.7 pure fat).
	DefaultRQ = 0.8
)

// Config describes an organ's relationship with the bloodstream.
type Config struct {
	// Organ names the host, for telemetry and logs.
	Organ string

	// O2PerMinute is the organ's oxygen demand at rest, in mL/min. The published
	// resting figures are roughly: brain 50, heart 30, kidneys 18, gut 50,
	// resting muscle 50, skin 12, diaphragm 3.
	O2PerMinute float64

	// RQ is the respiratory quotient; zero means DefaultRQ.
	RQ float64

	// ContentOnset and ContentFail bound where the organ's function starts to
	// fail and where it is gone, in mL of oxygen per dL of blood. Zero means the
	// defaults. The brain, which tolerates least, sets these higher.
	ContentOnset, ContentFail float64

	// Demand optionally scales the resting draw by what the organ is currently
	// doing -- a working muscle takes many times its resting share, and a
	// diaphragm fighting stiff lungs takes several times its own. It returns a
	// multiplier; nil means a constant 1.
	Demand func(*component.Component) float64
}

// Perfusion is the plugin instance.
type Perfusion struct {
	organ       string
	o2PerMinute float64
	rq          float64
	onset, fail float64
	demand      func(*component.Component) float64
}

// New builds a perfusion plugin for an organ.
func New(cfg Config) *Perfusion {
	p := &Perfusion{
		organ:       cfg.Organ,
		o2PerMinute: cfg.O2PerMinute,
		rq:          cfg.RQ,
		onset:       cfg.ContentOnset,
		fail:        cfg.ContentFail,
		demand:      cfg.Demand,
	}
	if p.rq <= 0 {
		p.rq = DefaultRQ
	}
	if p.onset <= 0 {
		p.onset = DefaultContentOnset
	}
	if p.fail <= 0 {
		p.fail = DefaultContentFail
	}
	return p
}

func (p *Perfusion) GetName() string { return "Perfusion" }

func (p *Perfusion) Init(c *component.Component) error {
	// Start every organ believing it is well supplied, so it behaves normally on
	// the first tick, before any blood has reached it.
	c.State().Set(stateSaO2, bloodstream.NormalSaO2)
	c.State().Set(statePaO2, bloodstream.NormalPaO2)
	c.State().Set(statePaCO2, bloodstream.NormalPaCO2)
	c.State().Set(stateContent, bloodstream.NormalOxygenContent)
	c.State().Set(stateGlucose, bloodstream.DefaultGlucoseLevel)
	c.State().Set(stateO2PerMinute, p.o2PerMinute)
	c.State().Set(stateOnset, p.onset)
	c.State().Set(stateFail, p.fail)

	if err := c.AddInputs(SupplyPort); err != nil {
		return err
	}
	if err := c.AddOutputs(ReturnPort); err != nil {
		return err
	}

	c.SetupHooks(func(hooks *component.Hooks) {
		hooks.OnActivation(p.onActivation)
	})
	return nil
}

// onActivation latches whatever the blood has delivered and, once per tick,
// puts the organ's demand back onto the shared bus.
func (p *Perfusion) onActivation(this *component.Component) error {
	p.latchSupply(this)

	// The draw is emitted on the tick alone, so an organ that activates several
	// times in one run (the blood bus and its own inputs arrive on different
	// cycles) still consumes exactly one tick's worth.
	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	// Rates per second; the blood integrates them over the tick, which is the
	// same contract every other secretion on the bus follows.
	o2PerSecond := p.o2PerMinute / 60.0 * p.currentDemand(this)
	return this.OutputByName(ReturnPort).PutSignals(
		bloodstream.Secretion(bloodstream.SubstanceO2Draw, o2PerSecond),
		bloodstream.Secretion(bloodstream.SubstanceCO2Load, o2PerSecond*p.rq),
	)
}

// latchSupply remembers the last arterial reading, since the blood signal
// arrives on its own mesh cycle and the organ may need it on another.
func (p *Perfusion) latchSupply(this *component.Component) {
	in := this.InputByName(SupplyPort)
	if in == nil || !in.HasSignals() {
		return
	}

	sig := in.Signals().First()
	if sig == nil {
		return
	}

	s := sig.Scalars()
	this.State().Set(stateSaO2, s.ValueOrDefault("SpO2", bloodstream.NormalSaO2))
	this.State().Set(statePaO2, s.ValueOrDefault("PaO2", bloodstream.NormalPaO2))
	this.State().Set(statePaCO2, s.ValueOrDefault("PaCO2", bloodstream.NormalPaCO2))
	this.State().Set(stateContent, s.ValueOrDefault("CaO2", bloodstream.NormalOxygenContent))
	this.State().Set(stateGlucose, s.ValueOrDefault("glucose_level", bloodstream.DefaultGlucoseLevel))
}

func (p *Perfusion) currentDemand(this *component.Component) float64 {
	if p.demand == nil {
		return 1.0
	}
	return max(p.demand(this), 0)
}

// Supply is the last arterial reading an organ received.
type Supply struct {
	// SaO2 is hemoglobin saturation (%), PaO2 and PaCO2 are partial pressures
	// (mmHg), Content is oxygen carried per dL of blood, Glucose is mg/dL.
	SaO2, PaO2, PaCO2, Content, Glucose float64
}

// Read returns what the blood last delivered to an organ.
func Read(c *component.Component) Supply {
	if c == nil {
		return Supply{}
	}
	get := func(key common.State, fallback float64) float64 {
		v, ok := c.State().Get(key).(float64)
		if !ok {
			return fallback
		}
		return v
	}
	return Supply{
		SaO2:    get(stateSaO2, bloodstream.NormalSaO2),
		PaO2:    get(statePaO2, bloodstream.NormalPaO2),
		PaCO2:   get(statePaCO2, bloodstream.NormalPaCO2),
		Content: get(stateContent, bloodstream.NormalOxygenContent),
		Glucose: get(stateGlucose, bloodstream.DefaultGlucoseLevel),
	}
}

// Sufficiency reports how well the blood is meeting an organ's needs: 1 when it
// is comfortably supplied, 0 when the supply cannot sustain it.
//
// It is expressed in oxygen content rather than in partial pressure or
// saturation, because that is what a tissue actually receives. A patient who has
// lost a third of their blood, or whose hemoglobin is bound up by carbon
// monoxide, has a perfectly normal PaO₂ and SpO₂ and is nonetheless starving --
// which a model built on tension alone cannot express.
func Sufficiency(c *component.Component) float64 {
	if c == nil {
		return 0
	}

	content, ok := c.State().Get(stateContent).(float64)
	if !ok {
		return 1
	}
	onset, _ := c.State().Get(stateOnset).(float64)
	fail, _ := c.State().Get(stateFail).(float64)
	if onset <= fail {
		return 1
	}
	return helper.Clamp((content-fail)/(onset-fail), 0, 1)
}

// O2PerMinute returns an organ's resting oxygen demand, so the body can report
// what it is asking for in total.
func O2PerMinute(c *component.Component) float64 {
	if c == nil {
		return 0
	}
	v, _ := c.State().Get(stateO2PerMinute).(float64)
	return v
}

// IsPerfused reports whether a component carries this plugin, which is how the
// mesh knows to connect it to the bloodstream.
func IsPerfused(c *component.Component) bool {
	if c == nil {
		return false
	}
	_, ok := c.State().Get(stateO2PerMinute).(float64)
	return ok
}
