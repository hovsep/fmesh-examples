package damage

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

type Damage struct {
}

const (
	damageLevel         common.State = "damage_level"
	criticalDamageLevel              = 1.0 * unit.DNCS
	defaultDamageLevel               = 0.01 * unit.DNCS
	damageRampRate                   = 3.5e-12 * unit.DNCS //  ~90 years
)

func (d Damage) GetName() string {
	return "Damage"
}

// @TODO:: ability to receive damage from outside
// @TODO:: check if we need to check if time is ticking in order to apply damage
// @TODO:: add this plugin to all organs and other components where makes sense (boundaries, distributed anatomy etc)
func (d Damage) Init(c *component.Component) error {
	c.Logger().Println("Damage plugin initializing")

	// State
	c.State().Set(damageLevel, defaultDamageLevel)

	// IO
	if err := c.AddOutputs("failure"); err != nil {
		return err
	}

	// Activation
	c.SetupHooks(func(hooks *component.Hooks) {
		hooks.OnActivation(damageActivationFunction)
	})

	return nil
}

func New() *Damage {
	return &Damage{}
}

func damageActivationFunction(this *component.Component) error {
	var currentDamage float64

	this.State().Update(damageLevel, func(oldDamage any) any {
		currentDamage = oldDamage.(float64)
		return currentDamage + damageRampRate
	})

	if currentDamage >= criticalDamageLevel {
		return this.OutputByName("failure").PutSignals(signal.New("brain_failure").WithLabel("type", "acute"))
	}

	return nil
}
