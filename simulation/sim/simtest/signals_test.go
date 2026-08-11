package simtest

import (
	"testing"

	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sigs(payloads ...any) []*signal.Signal {
	out := make([]*signal.Signal, 0, len(payloads))
	for _, p := range payloads {
		out = append(out, signal.New(p))
	}
	return out
}

func traffic(before, after []*signal.Signal) SignalObservation {
	return SignalObservation{Name: "port", Before: before, After: after}
}

func TestPresenceChecks(t *testing.T) {
	spoke := traffic(nil, sigs("tick"))
	silent := traffic(sigs("tick"), nil)

	assert.NoError(t, Emits().Verify(spoke))
	assert.Error(t, Emits().Verify(silent))

	assert.NoError(t, EmitsNothing().Verify(silent))
	assert.Error(t, EmitsNothing().Verify(spoke))

	assert.NoError(t, StartsEmitting().Verify(spoke))
	assert.Error(t, StartsEmitting().Verify(traffic(sigs("a"), sigs("b"))),
		"a port that was already talking did not start")

	assert.NoError(t, StopsEmitting().Verify(silent))
	assert.Error(t, StopsEmitting().Verify(traffic(nil, nil)),
		"a port that was already quiet did not stop")

	assert.NoError(t, EmitsAtLeast(2).Verify(traffic(nil, sigs("a", "b", "c"))))
	assert.Error(t, EmitsAtLeast(4).Verify(traffic(nil, sigs("a", "b", "c"))))
}

func TestPayloadChecks(t *testing.T) {
	// Payloads that are not measurements are exactly what this track is for.
	o := traffic(nil, sigs("alive", "alive", "dead"))

	assert.NoError(t, PayloadIs("dead").Verify(o))
	assert.Error(t, PayloadIs("undead").Verify(o))

	assert.NoError(t, EveryPayloadIs("alive").Verify(traffic(nil, sigs("alive", "alive"))))
	assert.Error(t, EveryPayloadIs("alive").Verify(o), "one of them was 'dead'")
	assert.Error(t, EveryPayloadIs("alive").Verify(traffic(nil, nil)),
		"no signals at all cannot satisfy a claim about every signal")

	// Non-string payloads compare by value, not by formatting.
	assert.NoError(t, PayloadIs(42).Verify(traffic(nil, sigs(42))))
	assert.Error(t, PayloadIs(42).Verify(traffic(nil, sigs("42"))),
		"the string 42 is not the number 42")
}

func TestLabelAndScalarChecks(t *testing.T) {
	labelled := signal.New("load").
		WithLabel("category", "activity").
		WithScalar("intensity", 8)

	o := traffic(nil, []*signal.Signal{labelled})

	assert.NoError(t, Labelled("category", "activity").Verify(o))
	assert.Error(t, Labelled("category", "weather").Verify(o))
	assert.Error(t, Labelled("missing", "activity").Verify(o))

	assert.NoError(t, CarriesScalar("intensity").Verify(o))
	assert.Error(t, CarriesScalar("duration_s").Verify(o))

	assert.NoError(t, ScalarIs("intensity", 8, 0.01).Verify(o))
	assert.Error(t, ScalarIs("intensity", 3, 0.01).Verify(o))
	assert.Error(t, ScalarIs("duration_s", 8, 0.01).Verify(o),
		"a scalar that is not there cannot be the right value")
}

func TestMatchingAndComposition(t *testing.T) {
	o := traffic(nil, sigs("a", "bb", "ccc"))

	long := Matching("carry a payload longer than two characters", func(s *signal.Signal) bool {
		text, ok := s.Payload().(string)
		return ok && len(text) > 2
	})
	assert.NoError(t, long.Verify(o))
	assert.Error(t, long.Verify(traffic(nil, sigs("a", "bb"))))

	both := AllSignals(Emits(), PayloadIs("bb"))
	assert.NoError(t, both.Verify(o))
	assert.Contains(t, both.Describe(), "and")
	assert.Error(t, AllSignals(Emits(), PayloadIs("zz")).Verify(o))
}

func TestEverySignalCheckDescribesItself(t *testing.T) {
	for _, c := range []SignalCheck{
		Emits(), EmitsNothing(), EmitsAtLeast(2), StartsEmitting(), StopsEmitting(),
		PayloadIs("x"), EveryPayloadIs("x"), Labelled("a", "b"),
		CarriesScalar("s"), ScalarIs("s", 1, 0.1),
	} {
		require.NotEmpty(t, c.Describe())
	}
}
