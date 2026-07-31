package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observedAliveness installs a hook that latches whether is_alive was ever
// observed as 0 over the run, reading the telemetry the TUI reads.
func observedAliveness(t *testing.T, sim *session.Session) func() (everDead bool, final float64) {
	t.Helper()
	agg := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, agg)

	everDead := false
	final := 1.0
	simMesh(sim).SetupHooks(func(h *fmesh.Hooks) {
		h.AfterRun(func(*fmesh.FMesh) error {
			if s := agg.OutputByName("human-Leon::is_alive").Signals().First(); s != nil {
				if v, ok := signal.AsNumber(s); ok {
					final = v
					if v == 0 {
						everDead = true
					}
				}
			}
			return nil
		})
	})
	return func() (bool, float64) { return everDead, final }
}

func organComp(t *testing.T, sim *session.Session, name string) *component.Component {
	t.Helper()
	inner := human.InnerMesh(human.Find(simMesh(sim)))
	require.NotNil(t, inner)
	c := inner.ComponentByName(name)
	require.NotNil(t, c, "no %q in the human mesh", name)
	return c
}

// Test_BrainFailureKillsTheBody drives the death path directly: a fatal brain
// injury flatlines the brain, the body reads no brain activity and dies, and the
// mesh keeps running (a corpse is frozen, not left to stall).
func Test_BrainFailureKillsTheBody(t *testing.T) {
	sim := newCommandableSim(t)
	brain := organComp(t, sim, "organ:brain")
	aliveness := observedAliveness(t, sim)

	// Injure the brain to failure after the body has been alive a moment.
	injured := false
	simMesh(sim).SetupHooks(func(h *fmesh.Hooks) {
		h.AfterRun(func(*fmesh.FMesh) error {
			if !injured {
				damage.Inflict(brain, 2*damage.CriticalLevel)
				injured = true
			}
			return nil
		})
	})

	simtest.RunFor(sim, 2*time.Second, func() {
		assert.True(t, damage.Failed(brain), "the brain should have failed")
		everDead, final := aliveness()
		assert.True(t, everDead, "the body should be observed dead")
		assert.Equal(t, 0.0, final, "is_alive should latch at 0")
	})
}

// Test_DeathIsIrreversible checks the death latch does not flicker back to alive
// once the body has died.
func Test_DeathIsIrreversible(t *testing.T) {
	sim := newCommandableSim(t)
	heart := organComp(t, sim, "organ:heart")
	brain := organComp(t, sim, "organ:brain")

	injured := false
	var aliveAfterDeath int
	seenDead := false
	agg := simMesh(sim).ComponentByName("aggregated_state")
	simMesh(sim).SetupHooks(func(h *fmesh.Hooks) {
		h.AfterRun(func(*fmesh.FMesh) error {
			if !injured {
				damage.Inflict(brain, 2*damage.CriticalLevel)
				injured = true
			}
			if s := agg.OutputByName("human-Leon::is_alive").Signals().First(); s != nil {
				v, _ := signal.AsNumber(s)
				if v == 0 {
					seenDead = true
				} else if seenDead {
					aliveAfterDeath++
				}
			}
			_ = heart
			return nil
		})
	})

	simtest.RunFor(sim, 3*time.Second, func() {
		assert.True(t, seenDead, "the body should have died")
		assert.Zero(t, aliveAfterDeath, "a dead body must not come back to life")
	})
}

// Test_ColdInjuresTheBrainBeforeTheKidney checks the emergent collapse order:
// under cold, the brain (temperature sensitivity 0.6) accrues damage faster than
// the kidney (0.3), so it would fail first. Asserting on the damage levels after
// a short exposure keeps the test fast; the full march to death is the same
// mechanism, only slower.
func Test_ColdInjuresTheBrainBeforeTheKidney(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	sim.Do("temp:cold")

	brain := organComp(t, sim, "organ:brain")
	kidney := organComp(t, sim, "organ:kidney")

	// Eight minutes: long enough for the core to reach the temperature at which
	// injury starts, now that a freezing body cools at a watchable pace rather
	// than losing seven degrees in two minutes.
	simtest.RunFor(sim, 8*time.Minute, func() {
		brainDamage := damage.Level(brain)
		kidneyDamage := damage.Level(kidney)
		assert.Greater(t, brainDamage, 0.0, "cold should injure the brain")
		assert.Greater(t, brainDamage, kidneyDamage,
			"the brain should be injured faster than the kidney under cold (it fails first)")
	})
}

// Test_HealthyBodyNeverDies is the guard that the death machinery does not fire
// spuriously: a well-kept body stays alive throughout.
func Test_HealthyBodyNeverDies(t *testing.T) {
	sim := newCommandableSim(t)
	aliveness := observedAliveness(t, sim)

	simtest.RunFor(sim, 30*time.Second, func() {
		everDead, final := aliveness()
		assert.False(t, everDead, "a healthy body must not be reported dead")
		assert.Equal(t, 1.0, final)
	})
}
