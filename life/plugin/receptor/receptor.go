// Package receptor is a component plugin: attach it to an organ and the organ
// can feel hormones.
//
// It is the counterpart to perfusion. That plugin lets a tissue take what it
// needs out of the blood; this one lets a tissue be told something by it. Both
// speak the same bus, which is the point -- a gland secretes into the
// bloodstream exactly as an organ returns carbon dioxide to it, and a target
// reads it exactly as it reads its oxygen. Adding an endocrine axis therefore
// costs a gland, a substance name, and the receptors that care.
//
// Receptors are declared, not discovered: an organ says which hormones it
// responds to, and reads only those. That is faithful -- a tissue without the
// receptor is deaf to a hormone however much of it is circulating, which is why
// the same adrenaline that races a heart does nothing at all to a bone.
package receptor

import (
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh/component"
)

// statePrefix namespaces the levels this plugin latches, so two plugins on the
// same organ cannot tread on each other's state.
const statePrefix = "receptor_"

// Receptors is the plugin instance: the set of hormones an organ can feel.
type Receptors struct {
	hormones []string
}

// For builds a receptor plugin for the named hormones.
//
//	receptor.For(bloodstream.HormoneAdrenaline)
func For(hormones ...string) *Receptors {
	return &Receptors{hormones: hormones}
}

func (r *Receptors) GetName() string { return "Receptors" }

func (r *Receptors) Init(c *component.Component) error {
	// Every receptor starts reading nothing, so an organ behaves as its
	// unstimulated self until a gland says otherwise.
	for _, hormone := range r.hormones {
		c.State().Set(stateKey(hormone), 0.0)
	}

	// An organ that is perfused already has the port; one that only listens
	// gets it here.
	if err := bloodstream.EnsureSupplyPort(c); err != nil {
		return err
	}

	c.SetupHooks(func(hooks *component.Hooks) {
		hooks.OnActivation(r.latch)
	})
	return nil
}

// latch reads the hormones this organ can feel out of the passing blood.
func (r *Receptors) latch(this *component.Component) error {
	in := this.InputByName(bloodstream.SupplyPort)
	if in == nil || !in.HasSignals() {
		return nil
	}

	sig := in.Signals().First()
	if sig == nil {
		return nil
	}

	for _, hormone := range r.hormones {
		this.State().Set(stateKey(hormone), sig.Scalars().ValueOrDefault(hormone, 0))
	}
	return nil
}

func stateKey(hormone string) string { return string(statePrefix + hormone) }

// Level returns how much of a hormone an organ is currently feeling, 0..1.
// An organ without the receptor reads zero, whatever is circulating.
func Level(c *component.Component, hormone string) float64 {
	if c == nil {
		return 0
	}
	level, _ := c.State().Get(stateKey(hormone)).(float64)
	return level
}
