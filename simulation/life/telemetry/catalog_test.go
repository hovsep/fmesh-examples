package telemetry

import (
	"strings"
	"testing"
)

func TestCatalogIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("catalog is invalid: %v", err)
	}
}

func TestCatalogIsNotEmpty(t *testing.T) {
	// A catalog that silently emptied would take the whole UI with it, and every
	// derived list below would vacuously pass.
	if len(Catalog) == 0 {
		t.Fatal("catalog is empty")
	}
	if len(Signals(DefaultSubject)) == 0 {
		t.Fatal("catalog publishes no drawable signals")
	}
}

func TestPortsMatchCatalog(t *testing.T) {
	ports := Ports()
	if len(ports) != len(Catalog) {
		t.Fatalf("Ports() returned %d entries for %d metrics", len(ports), len(Catalog))
	}

	seen := make(map[string]bool, len(ports))
	for _, p := range ports {
		if seen[p] {
			t.Errorf("duplicate port %q", p)
		}
		seen[p] = true
	}
}

func TestSourcePortsAreDeduplicated(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range SourcePorts() {
		if s == "" {
			t.Error("SourcePorts() contains an empty port name")
		}
		if seen[s] {
			t.Errorf("SourcePorts() repeated %q; observable_state cannot declare the same input twice", s)
		}
		seen[s] = true
	}
}

func TestPassThroughExcludesComputedMetrics(t *testing.T) {
	for _, m := range PassThrough() {
		if m.Computed {
			t.Errorf("metric %q is computed but appears in PassThrough(); it would be forwarded and derived at once", m.Port)
		}
		if m.Source == "" {
			t.Errorf("metric %q has no source but appears in PassThrough(); there is nothing to forward", m.Port)
		}
	}
}

func TestEveryMetricHasAProducerOrSaysItDoesNot(t *testing.T) {
	// A metric must be forwarded, derived, or explicitly flagged as having no
	// producer yet. One that is none of those has an output port nothing writes
	// to, and reads as a working metric stuck at zero.
	forwarded := make(map[string]bool)
	for _, m := range PassThrough() {
		forwarded[m.Port] = true
	}

	for _, m := range Catalog {
		if !forwarded[m.Port] && !m.Computed && !m.AwaitingProducer {
			t.Errorf("metric %q is neither forwarded nor computed; mark it AwaitingProducer if that is intentional", m.Port)
		}
	}
}

func TestAwaitingProducerListsTheKnownGaps(t *testing.T) {
	// Nothing writes body temperature: thermoregulation is not modelled yet.
	// This test exists so the gap has to be closed deliberately rather than
	// discovered by wondering why a dashboard reads 0.0 degrees.
	pending := make(map[string]bool)
	for _, m := range AwaitingProducer() {
		pending[m.Port] = true
	}

	if !pending["body_temperature"] {
		t.Log("body_temperature now has a producer; drop its AwaitingProducer flag")
	}
}

func TestKeysAreNamespacedBySubject(t *testing.T) {
	for _, m := range Catalog {
		key := m.Key("human-Ada")
		if !strings.HasPrefix(key, "human-Ada"+PathSeparator) {
			t.Errorf("metric %q produced key %q, which is not namespaced by its subject", m.Port, key)
		}
	}
}

func TestSignalsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range Signals(DefaultSubject) {
		if seen[s.Key] {
			t.Errorf("duplicate signal key %q; one would overwrite the other in the UI registry", s.Key)
		}
		seen[s.Key] = true

		if s.Label == "" {
			t.Errorf("signal %q has no label", s.Key)
		}
		if s.Min > s.Max {
			t.Errorf("signal %q has an inverted range [%v, %v]", s.Key, s.Min, s.Max)
		}
		if s.HealthMin > s.HealthMax {
			t.Errorf("signal %q has an inverted health band [%v, %v]", s.Key, s.HealthMin, s.HealthMax)
		}
	}
}

func TestCompositeSignalsPublishTheirScalars(t *testing.T) {
	signals := Signals(DefaultSubject)
	published := make(map[string]bool, len(signals))
	for _, s := range signals {
		published[s.Key] = true
	}

	for _, m := range Catalog {
		for _, scalar := range m.Scalars {
			key := m.Key(DefaultSubject) + ScalarSeparator + scalar.Name
			if !published[key] {
				t.Errorf("scalar %q of metric %q is described but never published as %q", scalar.Name, m.Port, key)
			}
		}
	}
}

func TestValidateCatchesStructuralMistakes(t *testing.T) {
	// Validate is the guard the other tests rely on, so check it actually rejects
	// the mistakes it claims to.
	original := Catalog
	t.Cleanup(func() { Catalog = original })

	tests := []struct {
		name    string
		catalog []Metric
	}{
		{
			name:    "metric with no port",
			catalog: []Metric{{Display: &Display{Label: "x"}}},
		},
		{
			name: "duplicate ports",
			catalog: []Metric{
				{Port: "heart_rate", Display: &Display{Label: "a"}},
				{Port: "heart_rate", Display: &Display{Label: "b"}},
			},
		},
		{
			name: "payload and scalars at once",
			catalog: []Metric{{
				Port:    "blood",
				Display: &Display{Label: "a"},
				Scalars: []Scalar{{Name: "O2_level"}},
			}},
		},
		{
			name:    "nothing drawable",
			catalog: []Metric{{Port: "orphan"}},
		},
		{
			name: "scalar with no name",
			catalog: []Metric{{
				Port:    "blood",
				Scalars: []Scalar{{Display: Display{Label: "a"}}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Catalog = tt.catalog
			if err := Validate(); err == nil {
				t.Fatal("Validate() accepted an invalid catalog")
			}
		})
	}
}
