package main

import (
	"strings"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every command must actually reach the simulation and move the body, not just
// the thing nearest to it.
//
// A command that silently does nothing is the failure this model is most prone
// to, because nothing reports it: the signal is put on a port, the port is
// drained, and the body carries on exactly as before. The older version of this
// file caught some of that, and missed the rest for a structural reason worth
// remembering. It allowed one probe per command, and for most of them the
// nearest probe is the *air* -- so `temp:hot` asserted that the habitat got
// hotter and never that the human noticed. It could therefore sit in the covered
// list while fifty simulated minutes at 38 C left the core at exactly 37.000 and
// the sweat rate at exactly 0.000.
//
// So each command is now driven through the real session and judged on its whole
// downstream chain at once: raising the air temperature has to reach the skin,
// the core, the sweat glands and the water balance, and the claim is written out
// link by link. Everything the claim does not mention is still checked -- the
// backstop asserts it stayed where it was -- which is what makes this catch a
// mechanism nobody thought about.

// observables is everything the tests can asserted on, generated from the
// telemetry catalog rather than written out.
//
// Generated on purpose: adding a metric to the catalog therefore puts it under
// the backstop of every scenario immediately, and a value cannot be published
// and left unwatched.
func observables() []simtest.Observable {
	var watched []simtest.Observable

	for _, sig := range telemetry.Signals(telemetry.DefaultSubject) {
		// A key is "subject::port" or "subject::port:scalar", and a scalar name
		// may itself contain colons ("composition:oxygen"), so the split is on
		// the first colon after the port.
		_, rest, _ := strings.Cut(sig.Key, telemetry.PathSeparator)
		portName, scalarName, isScalar := strings.Cut(rest, telemetry.ScalarSeparator)

		aggregatorPort := telemetry.DefaultSubject + telemetry.PathSeparator + portName
		if isScalar {
			watched = append(watched,
				simtest.Scalar(aggregator, aggregatorPort, scalarName).Named(rest))
			continue
		}
		watched = append(watched, simtest.Port(aggregator, aggregatorPort).Named(rest))
	}

	// The habitat's own readings. The body is judged against the world it is in,
	// so a scenario can say both what the air did and what the body did about it.
	for _, scalar := range []string{
		atmosphere.ScalarPressure, atmosphere.ScalarCOppm,
		"temperature", "humidity", "composition:oxygen",
		"composition:carbon_dioxide", "composition:pollution",
	} {
		watched = append(watched,
			simtest.Scalar(aggregator, "air::environmental_gas", scalar).Named("air:"+scalar))
	}
	watched = append(watched, simtest.Port(aggregator, "sun::uvi").Named("sun:uvi"))

	return watched
}

const aggregator = "aggregated_state"

// The timing profiles scenarios share.
//
// Shared deliberately: the runner executes one do-nothing control run per
// distinct profile and caches it, so scenarios that agree on their timings cost
// one control between them rather than one each.
var (
	// quick is for chains that finish in seconds -- a reflex, a blood gas.
	//
	// The windows are a minute rather than the twenty seconds they started as,
	// because the body is full of waveforms. A window only four breaths long
	// averages a different part of the breathing cycle each run, and the tidal
	// volumes and alveolar gases then wander several percent between two runs
	// that differ in nothing. A dozen breaths averages the phase away.
	quick = simtest.Scenario{
		Warm: 30 * time.Second, Baseline: 60 * time.Second,
		Settle: 120 * time.Second, Window: 60 * time.Second,
	}
	// paced is for chemistry: hormones, gastric emptying, fatigue.
	paced = simtest.Scenario{
		Warm: 60 * time.Second, Baseline: 30 * time.Second,
		Settle: 6 * time.Minute, Window: 90 * time.Second,
	}
	// slow is for the 600-second half-lives: air warming, core temperature.
	//
	// Twenty minutes is two half-lives, which is three quarters of the way to
	// wherever the value is going -- ample for a claim about direction, and half
	// the cost of waiting for the whole move.
	slow = simtest.Scenario{
		Warm: 60 * time.Second, Baseline: 30 * time.Second,
		Settle: 20 * time.Minute, Window: 3 * time.Minute,
	}
)

// at fills a timing profile in with the rest of a scenario.
//
// The backstop it installs is deliberately tight: two percent of the baseline,
// on top of whatever the control run drifted by. Loose enough that the model's
// own jitter (the respiratory rate carries two percent of it) does not trip it,
// tight enough that a real effect cannot hide behind it.
func at(profile simtest.Scenario, name string, cmd command.Line, expect map[string]simtest.Check) simtest.Scenario {
	profile.Name = name
	profile.Command = cmd
	profile.Expect = expect
	// Four percent rather than two. The respiratory rate carries two percent of
	// deliberate jitter, so a two percent backstop sits exactly on the model's
	// own noise floor and flags a breathing rate for breathing.
	profile.Backstop = simtest.StaysWithin(0.04)
	profile.Ignore = alwaysIgnored
	return profile
}

// alwaysIgnored is the short list of observables no scenario backstops.
//
// Both are classifications rather than measurements, and a proportional
// tolerance means nothing applied to them: the brain-activity trend is a
// direction code that sits at zero and flickers between -1 and 1, so any move at
// all is an infinite percentage of its baseline. Judging it needs a check about
// direction codes, which is worth writing the day something depends on it.
// The lung flows and alveolar pressures are here for a different reason: they
// are instantaneous points on a waveform that swings symmetrically about zero,
// so the mean of any window is a number about where the window happened to fall
// rather than about the body. Their amplitude is meaningful and their average is
// not, and asserting on it needs a check about amplitude that does not exist yet.
var alwaysIgnored = []string{
	"brain_activity_trend",
	"lung_left_flow", "lung_right_flow",
	"lung_left_alveolar_pressure", "lung_right_alveolar_pressure",
	"heart_cardiac_activation",
}

// merge combines expectation fragments into one claim.
//
// Later entries win, so a scenario can take a fragment wholesale and then
// override the one line where it differs.
func merge(parts ...map[string]simtest.Check) map[string]simtest.Check {
	all := map[string]simtest.Check{}
	for _, part := range parts {
		for name, check := range part {
			all[name] = check
		}
	}
	return all
}

// ventilating is what changes when the body moves more or less air.
//
// It is a fragment rather than eleven lines in every scenario because it is one
// fact: the lungs are a bellows, and every reading downstream of the bellows
// moves together. Breathing harder means bigger breaths, so more of both gases
// shifted per breath, and less of either left behind in what is exhaled.
func ventilating(dir simtest.Direction) map[string]simtest.Check {
	more, less := up(), down()
	if dir == simtest.Down {
		more, less = down(), up()
	}
	return checks{
		"respiratory_rate":                                  more,
		"lung_left_alveolar_gas:tick_volume":                more,
		"lung_right_alveolar_gas:tick_volume":               more,
		"lung_left_alveolar_gas:O2_vol":                     more,
		"lung_right_alveolar_gas:O2_vol":                    more,
		"lung_left_alveolar_gas:CO2_vol":                    more,
		"lung_right_alveolar_gas:CO2_vol":                   more,
		"lung_left_exhaled_gas:composition:oxygen":          less,
		"lung_right_exhaled_gas:composition:oxygen":         less,
		"lung_left_exhaled_gas:composition:carbon_dioxide":  less,
		"lung_right_exhaled_gas:composition:carbon_dioxide": less,
	}
}

// inspiring is the air the body is breathing following the air it is standing
// in. It is the least surprising chain in the model and the easiest to break,
// since it is the one hand-off between the habitat and the organism.
func inspiring(dir simtest.Direction, scalars ...string) map[string]simtest.Check {
	move := up()
	if dir == simtest.Down {
		move = down()
	}
	claim := checks{}
	for _, scalar := range scalars {
		claim["inspired_gas:"+scalar] = move
	}
	return claim
}

// failing is every organ taking damage at once, which is what a body that is
// dying of anything systemic looks like.
func failing() map[string]simtest.Check {
	claim := checks{}
	for _, organ := range telemetry.DamagedOrgans {
		claim[organ.Port+"_damage"] = up()
	}
	return claim
}

// suffocating is the whole picture of a body running out of oxygen: the blood,
// what it can carry, and what the body has to say about it.
func suffocating() map[string]simtest.Check {
	return checks{
		"blood_pao2":          down(),
		"blood_spo2":          down(),
		"venous_blood:SpO2":   down(),
		"venous_blood:CaO2":   down(),
		"feelings:breathless": up(),
		"feelings:content":    down(),
	}
}

// wholeBody loosens the backstop for a command that legitimately disturbs
// everything at once.
//
// A tight backstop is informative exactly when a command is supposed to be
// narrow: it is what says that raising the air by a degree must not move the
// blood sugar. It says nothing useful about a sprint, where every reading in the
// body is meant to move and the interesting claim is the *direction* of each --
// which is what the explicit expectations are for. So those scenarios keep a
// backstop, but one set where a value swinging by a quarter is still news.
func wholeBody(sc simtest.Scenario) simtest.Scenario {
	sc.Backstop = simtest.StaysWithin(0.25)
	return sc
}

// dying is for the scenarios that end with the subject dead. Once the brain has
// stopped there is no baseline left to compare anything against, so the claim is
// carried entirely by the explicit expectations.
func dying(sc simtest.Scenario) simtest.Scenario {
	sc.Backstop = nil
	return sc
}

// after gives a scenario the commands that must already have run for its own to
// mean anything: stopping requires having started.
func after(sc simtest.Scenario, setup ...command.Line) simtest.Scenario {
	sc.Setup = setup
	return sc
}

// Shorthands, because the table is easier to read down a column than across a
// line of package-qualified constructors.
var (
	up      = simtest.Increases
	down    = simtest.Decreases
	frozen  = simtest.StaysExactlyTheSame
	steady  = simtest.StaysWithin
	byFrac  = simtest.ChangesBy
	backTo  = simtest.ReturnsToward
	reaches = simtest.Reaches
	spikes  = simtest.RisesThenFalls
	dips    = simtest.FallsThenRises
	both    = simtest.All
)

func Test_CommandEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("drives every command through a real simulation")
	}

	suite := &simtest.Suite{
		NewSim:      func(t *testing.T) *session.Session { return newCommandableSim(t) },
		Observables: observables(),
		Tick:        testTick,
	}

	// The controls first and sequentially, then the scenarios all at once.
	//
	// Each scenario builds its own simulation and shares nothing with the others,
	// so they parallelise cleanly, and they need to: this table is most of the
	// package's running time and the suite has ten minutes for everything.
	scenarios := effectScenarios()
	suite.Warm(t, scenarios)

	for _, sc := range scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			t.Parallel()
			suite.Run(t, sc)
		})
	}
}

// effectScenarios is the claim this simulation makes about itself.
//
// Every command appears at least twice, because one dose proves a connection and
// two prove it is a mechanism: an effect that is the same at intensity 3 and
// intensity 8 is a switch someone wired, not a physiological response.
func effectScenarios() []simtest.Scenario {
	return concat(
		airScenarios(),
		weatherScenarios(),
		activityScenarios(),
		intakeScenarios(),
		affectScenarios(),
		traumaScenarios(),
	)
}

func concat(groups ...[]simtest.Scenario) []simtest.Scenario {
	var all []simtest.Scenario
	for _, g := range groups {
		all = append(all, g...)
	}
	return all
}

// bodyValue reads one number the outside world can see, for the few tests that
// want a reading rather than a claim about one.
//
// AsNumber rather than a typed accessor, because the body does not publish one
// numeric type: a heart rate is a whole number of beats and a stomach fill is
// not. Asking for a float64 and defaulting to zero reads a heart rate of zero
// off a perfectly healthy heart -- which is what this did at first, and is the
// same "typed accessor infers from the default" trap the accessor's own
// documentation warns about.
func bodyValue(port string) func(*session.Session) float64 {
	return func(sim *session.Session) float64 {
		p := simMesh(sim).ComponentByName(aggregator).
			OutputByName(telemetry.DefaultSubject + telemetry.PathSeparator + port)
		if p == nil || !p.HasSignals() {
			return 0
		}
		value, _ := signal.AsNumber(p.Signals().First())
		return value
	}
}

// Test_EveryRegisteredCommandIsCovered keeps the table honest.
//
// A command added to the simulation and not to the scenarios would otherwise be
// exactly the thing this file exists to catch, and nothing would say so. Two
// scenarios rather than one, because a single case proves a connection and only
// a second proves it is a response: an effect identical at every dose is a
// switch somebody wired, not physiology.
func Test_EveryRegisteredCommandIsCovered(t *testing.T) {
	sim := newCommandableSim(t)

	// The two that only report. They change nothing by design, so there is
	// nothing for a scenario to observe.
	readOnly := map[string]bool{"habitat:show": true, "time:now": true}

	// Everything the session brings with it -- pause, step, rate, scheduling --
	// belongs to the loop rather than to this simulation.
	ours := map[string]bool{}
	for _, name := range sim.Commands.Names() {
		if readOnly[name] {
			continue
		}
		switch command.Namespace(name) {
		case "air", "temp", "sun", "altitude", "smoke",
			"intake", "activity", "emotion", "excretion", "trauma":
			ours[name] = true
		}
	}

	covered := map[string]int{}
	for _, sc := range effectScenarios() {
		name, _ := sc.Command.Fields()
		covered[name]++
	}

	for name := range ours {
		assert.GreaterOrEqualf(t, covered[name], 2,
			"%s needs at least two scenarios; it has %d", name, covered[name])
	}
	for name := range covered {
		assert.Truef(t, ours[name], "%s is exercised but no longer registered", name)
	}
	require.NotEmpty(t, ours)
}
