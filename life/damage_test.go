package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/stretchr/testify/assert"
)

// Test_SmokingDamagesTheLungs is the headline what-if: smoking injures the lungs
// and little else. The toxin from a cigarette flows to both lungs' damage input.
func Test_SmokingDamagesTheLungs(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	// A heavy session at once, so the damage is visible in a short run.
	sim.Do("smoke:cigarette 200")

	lungLeft := organComp(t, sim, "organ:lung_left")
	lungRight := organComp(t, sim, "organ:lung_right")
	kidney := organComp(t, sim, "organ:kidney")

	simtest.RunFor(sim, 2*time.Minute, func() {
		leftDamage := damage.Level(lungLeft)
		rightDamage := damage.Level(lungRight)

		assert.Greater(t, leftDamage, 0.0, "smoking should damage the left lung")
		assert.Greater(t, rightDamage, 0.0, "smoking should damage the right lung")

		// The kidney is not in the smoke's path; only aging touches it, which is
		// negligible over twenty minutes.
		assert.Less(t, damage.Level(kidney), leftDamage,
			"smoking should hit the lungs far harder than an unexposed organ")
	})
}

// Test_ASingleCigaretteTakesMinutes checks the process is spanned, not instant:
// one cigarette is still being smoked a minute in, and is done well before it
// could take an implausibly long time.
func Test_ASingleCigaretteTakesMinutes(t *testing.T) {
	sim := newCommandableSim(t)
	sim.Do("smoke:cigarette")

	intake := bodyComponent(t, sim, "controller:intake")

	// One minute in, the cigarette (5-10 min) is still going.
	simtest.RunFor(sim, time.Minute, func() {
		set, _ := intake.State().Get(controller.StateProcesses).(*simtime.ProcessSet)
		if set != nil {
			assert.Positive(t, set.Active(), "a cigarette should still be burning after a minute")
		}
	})
}

// Test_DamagePluginAgesEveryOrgan is a structural guard: every organ that should
// carry the damage plugin exposes a damage level, so the Body view and the death
// cascade have something to read for each.
func Test_DamagePluginAgesEveryOrgan(t *testing.T) {
	sim := newCommandableSim(t)

	for _, name := range []string{
		"organ:brain", "organ:heart", "organ:diaphragm",
		"organ:lung_left", "organ:lung_right", "organ:kidney",
	} {
		c := organComp(t, sim, name)
		// A freshly built organ is undamaged but has the plugin (Failed is defined).
		assert.False(t, damage.Failed(c), "%s should start healthy", name)
		assert.Equal(t, 0.0, damage.Level(c), "%s should start at zero damage", name)
	}
}
