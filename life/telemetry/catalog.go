// Package telemetry is the single source of truth for everything the body
// publishes about itself.
//
// An observable value used to have to be declared in six places: the inputs and
// the outputs of physiology:observable_state, a pass-through pair in its
// activation function, the human component's outputs, its feedback wiring, the
// habitat aggregator's path list, and the TUI's signal registry. Miss one and
// the value either never leaves the body or arrives with no idea how to draw it.
//
// Everything is derived from the Catalog below instead, so adding a metric means
// adding one entry.
package telemetry

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hovsep/fmesh-examples/life/body"
)

// PathSeparator joins a component name to one of its ports to form a telemetry
// key, e.g. "human-Leon::heart_rate". The habitat aggregator parses paths in
// this form (see env/aggregated_state.go).
const PathSeparator = "::"

// ScalarSeparator joins a signal key to one of its scalars on the wire, e.g.
// "human-Leon::inspired_gas:composition:oxygen".
const ScalarSeparator = ":"

// DefaultSubject is the telemetry key prefix the UI expects. The mesh derives
// the real one from the human component's name; this is what a UI assumes when
// it has nothing else to go on.
const DefaultSubject = "human-Leon"

// View groups metrics into the screens the UI offers.
type View int

const (
	ViewOverview View = iota
	ViewCardiovascular
	ViewRespiratory
	ViewNervous
	ViewMetabolic
	ViewAffect
	ViewBody
)

// Display is how a numeric value should be drawn: its name, its units, the range
// a gauge should span, and the band outside which it is worth worrying about.
type Display struct {
	Label     string
	Unit      string
	Min, Max  float64
	HealthMin float64 // below this is concerning
	HealthMax float64 // above this is concerning
	Decimals  int
	View      View
}

// Scalar is one named number carried inside a composite signal.
//
// Structured signals (air, blood) use their payload as a type tag and keep the
// real measurements in scalars, so these are the values that actually reach a UI.
type Scalar struct {
	Name string
	Display
}

// Metric is one value published on a port of physiology:observable_state.
type Metric struct {
	// Port is the output port, on both observable_state and the human component.
	Port string

	// Source is the input port on observable_state that feeds this metric.
	// Empty means observable_state produces the value from nothing else, as it
	// does for liveness.
	Source string

	// Computed marks a metric that observable_state derives rather than simply
	// forwards. Its handler lives in observable_state.go; the generated
	// pass-through wiring skips it.
	Computed bool

	// AwaitingProducer marks a port that is declared so the UI has somewhere to
	// draw the value, but which no component writes yet.
	//
	// It exists to keep such a gap visible. Otherwise the metric is
	// indistinguishable from a working one that happens to read zero -- which is
	// exactly how body_temperature went unnoticed.
	AwaitingProducer bool

	// Display is set when the payload is a plain number a UI can draw. Composite
	// signals leave it nil and describe their Scalars instead.
	Display *Display

	// Scalars describes the named numbers a composite signal carries.
	Scalars []Scalar
}

// Key returns the telemetry key this metric publishes under for a given subject.
func (m Metric) Key(subject string) string {
	return subject + PathSeparator + m.Port
}

// Signal is a single drawable value on the wire, flattened from the catalog.
type Signal struct {
	Key string
	Display
}

// Catalog lists every value the body publishes.
//
// Ordering is by body system rather than alphabetical, so related values stay
// together when read as documentation.
var Catalog = []Metric{
	// --- Status ---------------------------------------------------------
	{
		Port:     "is_alive",
		Computed: true, // observable_state asserts liveness from brain activity
		Display: &Display{
			Label: "Status", Unit: "", Min: 0, Max: 1,
			HealthMin: 1, HealthMax: 1, Decimals: 0, View: ViewOverview,
		},
	},
	{
		Port: "body_temperature", Source: "body_temperature",
		Display: &Display{
			Label: "Body Temp", Unit: "°C", Min: 35, Max: 42,
			HealthMin: 36.5, HealthMax: 37.5, Decimals: 1, View: ViewOverview,
		},
	},

	// --- Metabolic ------------------------------------------------------
	{
		Port: "hydration", Source: "hydration",
		Display: &Display{
			Label: "Hydration", Unit: "%", Min: 95, Max: 101,
			HealthMin: 98.5, HealthMax: 101, Decimals: 2, View: ViewMetabolic,
		},
	},
	{
		Port: "glycemia", Source: "glycemia",
		Display: &Display{
			Label: "Blood Glucose", Unit: "mg/dL", Min: 40, Max: 200,
			HealthMin: 70, HealthMax: 140, Decimals: 0, View: ViewMetabolic,
		},
	},
	{
		Port: "energy", Source: "energy",
		Display: &Display{
			Label: "Energy Reserve", Unit: "kcal", Min: 0, Max: 4000,
			HealthMin: 800, HealthMax: 4000, Decimals: 0, View: ViewMetabolic,
		},
	},
	{
		Port: "stomach_fill", Source: "stomach_fill",
		Display: &Display{
			Label: "Stomach", Unit: "%", Min: 0, Max: 100,
			HealthMin: 0, HealthMax: 90, Decimals: 0, View: ViewMetabolic,
		},
	},
	{
		Port: "bladder_fill", Source: "bladder_fill",
		Display: &Display{
			Label: "Bladder", Unit: "%", Min: 0, Max: 100,
			HealthMin: 0, HealthMax: 90, Decimals: 0, View: ViewMetabolic,
		},
	},
	{
		Port: "bowel_fill", Source: "bowel_fill",
		Display: &Display{
			Label: "Bowel", Unit: "%", Min: 0, Max: 100,
			HealthMin: 0, HealthMax: 90, Decimals: 0, View: ViewMetabolic,
		},
	},
	{
		Port: "sweat_rate", Source: "sweat_rate",
		Display: &Display{
			Label: "Sweat Rate", Unit: "mL/s", Min: 0, Max: 1,
			HealthMin: 0, HealthMax: 0.5, Decimals: 3, View: ViewMetabolic,
		},
	},
	{
		// Shown next to the sweat rate, because the pair is the lesson: they are
		// the two effectors defending the same set point, and at any moment at
		// most one of them is working.
		Port: "shiver_rate", Source: "shiver_rate",
		Display: &Display{
			Label: "Shivering", Unit: "°C/s", Min: 0, Max: 0.004,
			HealthMin: 0, HealthMax: 0.001, Decimals: 4, View: ViewMetabolic,
		},
	},
	{
		Port: "muscle_fatigue", Source: "fatigue",
		Display: &Display{
			Label: "Muscle Fatigue", Unit: "%", Min: 0, Max: 100,
			HealthMin: 0, HealthMax: 80, Decimals: 0, View: ViewMetabolic,
		},
	},

	// --- Affect ---------------------------------------------------------
	{
		Port: "feelings", Source: "feelings",
		// One scalar per feeling, which is how the whole emotional picture
		// travels as a single consistent snapshot.
		Scalars: feelingScalars(),
	},

	// --- Nervous --------------------------------------------------------
	{
		Port: "brain_activity", Source: "brain_activity", Computed: true, // smoothed with an EMA
		Display: &Display{
			Label: "Brain Activity", Unit: "", Min: 0, Max: 1,
			HealthMin: 0.1, HealthMax: 0.8, Decimals: 2, View: ViewNervous,
		},
	},
	{
		Port: "brain_activity_trend", Computed: true, // derived alongside brain_activity
		Display: &Display{
			Label: "Brain Trend", Unit: "", Min: -1, Max: 1,
			HealthMin: -1, HealthMax: 1, Decimals: 0, View: ViewNervous,
		},
	},

	// --- Cardiovascular -------------------------------------------------
	{
		Port: "heart_rate", Source: "heart_rate",
		Display: &Display{
			Label: "Heart Rate", Unit: "BPM", Min: 40, Max: 200,
			HealthMin: 50, HealthMax: 100, Decimals: 0, View: ViewCardiovascular,
		},
	},
	{
		Port: "heart_cardiac_activation", Source: "heart_cardiac_activation",
		Display: &Display{
			Label: "Cardiac Activation", Unit: "", Min: 0, Max: 1,
			HealthMin: 0, HealthMax: 1, Decimals: 3, View: ViewCardiovascular,
		},
	},
	// The circulation: what the heart is achieving, and against what.
	{
		Port: "mean_arterial_pressure", Source: "mean_arterial_pressure",
		Display: &Display{
			Label: "MAP", Unit: "mmHg", Min: 0, Max: 140,
			HealthMin: 70, HealthMax: 105, Decimals: 0, View: ViewCardiovascular,
		},
	},
	{
		Port: "cardiac_output", Source: "cardiac_output",
		Display: &Display{
			Label: "Cardiac Output", Unit: "L/min", Min: 0, Max: 12,
			HealthMin: 4, HealthMax: 8, Decimals: 1, View: ViewCardiovascular,
		},
	},
	{
		Port: "stroke_volume", Source: "stroke_volume",
		// The scale runs past the healthy band on purpose: a working heart
		// ejects well over 90 mL a beat, and a gauge that pinned at 100 would
		// hide the half of the rise in cardiac output that is not rate.
		Display: &Display{
			Label: "Stroke Volume", Unit: "mL", Min: 0, Max: 130,
			HealthMin: 55, HealthMax: 90, Decimals: 0, View: ViewCardiovascular,
		},
	},
	{
		Port: "vascular_resistance", Source: "vascular_resistance",
		Display: &Display{
			Label: "SVR", Unit: "WU", Min: 0, Max: 40,
			HealthMin: 12, HealthMax: 24, Decimals: 1, View: ViewCardiovascular,
		},
	},

	// Blood gases are reported in the units and reference ranges a clinician
	// reads them in, so every value here can be checked against a textbook:
	// SpO₂ 95-100%, PaO₂ 80-100 mmHg, PaCO₂ 35-45 mmHg, pH 7.35-7.45.
	{
		Port: "blood_spo2", Source: "blood_spo2",
		Display: &Display{
			Label: "SpO₂", Unit: "%", Min: 50, Max: 100,
			HealthMin: 95, HealthMax: 100, Decimals: 1, View: ViewCardiovascular,
		},
	},
	{
		Port: "blood_pao2", Source: "blood_pao2",
		Display: &Display{
			Label: "PaO₂", Unit: "mmHg", Min: 0, Max: 120,
			HealthMin: 80, HealthMax: 100, Decimals: 0, View: ViewCardiovascular,
		},
	},
	{
		Port: "blood_paco2", Source: "blood_paco2",
		Display: &Display{
			Label: "PaCO₂", Unit: "mmHg", Min: 0, Max: 100,
			HealthMin: 35, HealthMax: 45, Decimals: 0, View: ViewCardiovascular,
		},
	},
	{
		Port: "venous_blood", Source: "venous_blood",
		Scalars: []Scalar{
			{Name: "SpO2", Display: Display{Label: "Saturation", Unit: "%", Min: 50, Max: 100, HealthMin: 95, HealthMax: 100, Decimals: 1, View: ViewCardiovascular}},
			{Name: "pH", Display: Display{Label: "Blood pH", Unit: "", Min: 7.0, Max: 7.8, HealthMin: 7.35, HealthMax: 7.45, Decimals: 2, View: ViewCardiovascular}},
			{Name: "CaO2", Display: Display{Label: "O₂ Content", Unit: "mL/dL", Min: 0, Max: 25, HealthMin: 16, HealthMax: 22, Decimals: 1, View: ViewCardiovascular}},
			{Name: "hemoglobin", Display: Display{Label: "Hemoglobin", Unit: "g/dL", Min: 0, Max: 20, HealthMin: 12, HealthMax: 17, Decimals: 1, View: ViewCardiovascular}},

			// Carboxyhemoglobin belongs next to the saturation rather than off on
			// its own, because the pair is the lesson: when this rises the
			// saturation does not fall, and reading one without the other is how
			// the poisoning gets missed. A non-smoker sits near zero and a smoker
			// near 5%; above 10% is a poisoning.
			{Name: "COHb", Display: Display{Label: "Carboxyhemoglobin", Unit: "%", Min: 0, Max: 60, HealthMin: 0, HealthMax: 5, Decimals: 1, View: ViewCardiovascular}},
			{Name: "volume_l", Display: Display{Label: "Blood Volume", Unit: "L", Min: 0, Max: 6, HealthMin: 4.5, HealthMax: 5.5, Decimals: 2, View: ViewCardiovascular}},

			// The endocrine answer, on its two clocks. Adrenaline arrives in
			// seconds and is gone in minutes; cortisol takes minutes to arrive
			// and hours to leave.
			{Name: "adrenaline", Display: Display{Label: "Adrenaline", Unit: "", Min: 0, Max: 1, HealthMin: 0, HealthMax: 0.3, Decimals: 2, View: ViewNervous}},
			{Name: "cortisol", Display: Display{Label: "Cortisol", Unit: "", Min: 0, Max: 1, HealthMin: 0, HealthMax: 0.3, Decimals: 2, View: ViewNervous}},

			// The pancreatic pair, shown against blood sugar rather than beside
			// the stress hormones, because they are only legible next to the
			// number they are arguing about. Either being high is normal at some
			// point in a day, so neither has a healthy range worth drawing -- what
			// matters is whether the right one is high.
			{Name: "insulin", Display: Display{Label: "Insulin", Unit: "", Min: 0, Max: 1, HealthMin: 0, HealthMax: 1, Decimals: 2, View: ViewMetabolic}},
			{Name: "glucagon", Display: Display{Label: "Glucagon", Unit: "", Min: 0, Max: 1, HealthMin: 0, HealthMax: 1, Decimals: 2, View: ViewMetabolic}},
		},
	},

	// --- Respiratory ----------------------------------------------------
	{
		Port: "respiratory_rate", Source: "respiratory_rate",
		Display: &Display{
			Label: "Resp Rate", Unit: "/min", Min: 8, Max: 30,
			HealthMin: 10, HealthMax: 20, Decimals: 0, View: ViewRespiratory,
		},
	},
	{
		Port: "pleural_pressure", Source: "pleural_pressure",
		Display: &Display{
			Label: "Pleural P", Unit: "cmH₂O", Min: -10, Max: 0,
			HealthMin: -8, HealthMax: -3, Decimals: 1, View: ViewRespiratory,
		},
	},
	{
		Port: "inspired_gas", Source: "inspired_gas",
		Scalars: airScalars("Inspired", ViewRespiratory),
	},
}

// feelingScalars describes the body's self-report, one scalar per feeling.
//
// The names and labels come from common, the same place physiology:affect reads
// them, so a feeling can never be published under one name and drawn under
// another.
func feelingScalars() []Scalar {
	scalars := make([]Scalar, 0, len(body.Feelings))
	for _, name := range body.Feelings {
		scalars = append(scalars, Scalar{
			Name: name,
			Display: Display{
				Label: body.FeelingLabels[name], Unit: "",
				Min: 0, Max: 1, HealthMin: 0, HealthMax: 1,
				Decimals: 2, View: ViewAffect,
			},
		})
	}
	return scalars
}

// airScalars describes the composition a breathable-gas signal carries.
//
// Every gas signal in the body (inspired, exhaled) uses the same scalar names,
// set by atmosphere.Pack, so the descriptions are generated rather than repeated.
func airScalars(prefix string, view View) []Scalar {
	gases := []struct {
		scalar   string
		label    string
		min, max float64
	}{
		{"composition:oxygen", "O₂", 0, 100},
		{"composition:carbon_dioxide", "CO₂", 0, 100},
		{"composition:nitrogen", "N₂", 0, 100},
		{"composition:argon", "Ar", 0, 100},
		{"composition:pollution", "Pollution", 0, 100},
	}

	scalars := make([]Scalar, 0, len(gases)+2)
	for _, g := range gases {
		scalars = append(scalars, Scalar{
			Name: g.scalar,
			Display: Display{
				Label: prefix + " " + g.label, Unit: "%",
				Min: g.min, Max: g.max, HealthMin: g.min, HealthMax: g.max,
				Decimals: 1, View: view,
			},
		})
	}

	return append(scalars,
		Scalar{Name: "temperature", Display: Display{
			Label: prefix + " Temp", Unit: "°C", Min: -40, Max: 50,
			HealthMin: -40, HealthMax: 50, Decimals: 1, View: view,
		}},
		Scalar{Name: "humidity", Display: Display{
			Label: prefix + " Humidity", Unit: "%", Min: 0, Max: 100,
			HealthMin: 0, HealthMax: 100, Decimals: 1, View: view,
		}},
	)
}

// lungMetrics returns the per-side lung metrics.
//
// The lungs are one component instantiated twice, so their metrics are generated
// per side rather than written out twice and drifting apart.
func lungMetrics(side string) []Metric {
	prefix := "lung_" + side + "_"
	label := strings.ToUpper(side[:1]) + " Lung"

	return []Metric{
		{
			Port: prefix + "volume", Source: prefix + "volume",
			Display: &Display{
				Label: label + " Vol", Unit: "mL", Min: 600, Max: 3000,
				HealthMin: 1000, HealthMax: 2500, Decimals: 0, View: ViewRespiratory,
			},
		},
		{
			Port: prefix + "flow", Source: prefix + "flow",
			Display: &Display{
				Label: label + " Flow", Unit: "mL/s", Min: -500, Max: 500,
				HealthMin: -400, HealthMax: 400, Decimals: 0, View: ViewRespiratory,
			},
		},
		{
			Port: prefix + "alveolar_pressure", Source: prefix + "alveolar_pressure",
			Display: &Display{
				Label: label + " Alv P", Unit: "cmH₂O", Min: -10, Max: 10,
				HealthMin: -8, HealthMax: 8, Decimals: 1, View: ViewRespiratory,
			},
		},
		{
			Port: prefix + "exhaled_gas", Source: prefix + "exhaled_gas",
			Scalars: airScalars(label+" Exhaled", ViewRespiratory),
		},
		{
			Port: prefix + "alveolar_gas", Source: prefix + "alveolar_gas",
			Scalars: []Scalar{
				{Name: "O2_vol", Display: Display{Label: label + " Alv O₂", Unit: "mL", Min: 0, Max: 200, HealthMin: 0, HealthMax: 200, Decimals: 1, View: ViewRespiratory}},
				{Name: "CO2_vol", Display: Display{Label: label + " Alv CO₂", Unit: "mL", Min: 0, Max: 200, HealthMin: 0, HealthMax: 200, Decimals: 1, View: ViewRespiratory}},
				{Name: "tick_volume", Display: Display{Label: label + " Tidal", Unit: "mL", Min: 0, Max: 200, HealthMin: 0, HealthMax: 200, Decimals: 2, View: ViewRespiratory}},
			},
		},
	}
}

// DamagedOrgans are the organs that carry the damage plugin and publish a damage
// level, in the order the Body view lists them.
var DamagedOrgans = []struct{ Port, Label string }{
	{"brain", "Brain"},
	{"heart", "Heart"},
	{"lung_left", "Left Lung"},
	{"lung_right", "Right Lung"},
	{"diaphragm", "Diaphragm"},
	{"kidney", "Kidney"},
	{"liver", "Liver"},
	{"pancreas", "Pancreas"},
}

func init() {
	// Both lungs publish the same metrics, so append them rather than writing
	// two near-identical blocks into the literal above.
	for _, side := range []string{"left", "right"} {
		Catalog = append(Catalog, lungMetrics(side)...)
	}

	// Every damageable organ publishes a 0..1 damage level on the Body view.
	for _, organ := range DamagedOrgans {
		port := organ.Port + "_damage"
		Catalog = append(Catalog, Metric{
			Port: port, Source: port,
			Display: &Display{
				Label: organ.Label, Unit: "", Min: 0, Max: 1,
				HealthMin: 0, HealthMax: 0.5, Decimals: 2, View: ViewBody,
			},
		})
	}
}

// Ports returns every port the body publishes on, which is both the set of
// outputs on physiology:observable_state and the set of outputs on the human
// component.
func Ports() []string {
	ports := make([]string, 0, len(Catalog))
	for _, m := range Catalog {
		ports = append(ports, m.Port)
	}
	return ports
}

// SourcePorts returns the inputs physiology:observable_state needs in order to
// receive the values it publishes.
func SourcePorts() []string {
	sources := make([]string, 0, len(Catalog))
	for _, m := range Catalog {
		if m.Source != "" && !slices.Contains(sources, m.Source) {
			sources = append(sources, m.Source)
		}
	}
	return sources
}

// PassThrough returns the metrics observable_state forwards unchanged, as
// (source, port) pairs. Computed metrics are excluded: they have handwritten
// handlers in observable_state.go.
func PassThrough() []Metric {
	var passThrough []Metric
	for _, m := range Catalog {
		if m.Source != "" && !m.Computed {
			passThrough = append(passThrough, m)
		}
	}
	return passThrough
}

// AwaitingProducer returns the metrics that have a port and a place in the UI
// but nothing writing to them yet. It is the catalog's own to-do list.
func AwaitingProducer() []Metric {
	var pending []Metric
	for _, m := range Catalog {
		if m.AwaitingProducer {
			pending = append(pending, m)
		}
	}
	return pending
}

// Paths returns the "component::port" paths the habitat aggregator subscribes to.
func Paths(subject string) []string {
	paths := make([]string, 0, len(Catalog))
	for _, m := range Catalog {
		paths = append(paths, m.Key(subject))
	}
	return paths
}

// Signals flattens the catalog into every drawable value on the wire, with the
// key a UI will see it under.
//
// A metric with a plain numeric payload contributes one signal; a composite
// contributes one per described scalar, since the composite's own payload is a
// type tag rather than a measurement.
func Signals(subject string) []Signal {
	var signals []Signal
	for _, m := range Catalog {
		key := m.Key(subject)
		if m.Display != nil {
			signals = append(signals, Signal{Key: key, Display: *m.Display})
		}
		for _, s := range m.Scalars {
			signals = append(signals, Signal{Key: key + ScalarSeparator + s.Name, Display: s.Display})
		}
	}
	return signals
}

// Validate reports structural problems in the catalog: duplicate ports, metrics
// that publish nothing drawable, or scalars declared on a metric that also has a
// plain payload.
func Validate() error {
	seenPorts := make(map[string]bool, len(Catalog))
	seenKeys := make(map[string]bool)

	for _, m := range Catalog {
		if m.Port == "" {
			return fmt.Errorf("catalog contains a metric with no port")
		}
		if seenPorts[m.Port] {
			return fmt.Errorf("duplicate metric port %q", m.Port)
		}
		seenPorts[m.Port] = true

		if m.Display != nil && len(m.Scalars) > 0 {
			return fmt.Errorf("metric %q declares both a payload display and scalars; a signal carries one or the other", m.Port)
		}
		if m.Display == nil && len(m.Scalars) == 0 {
			return fmt.Errorf("metric %q publishes nothing drawable: give it a Display or Scalars", m.Port)
		}
		if m.AwaitingProducer && (m.Computed || m.Source != "") {
			return fmt.Errorf("metric %q is marked AwaitingProducer but already has a producer", m.Port)
		}

		for _, s := range m.Scalars {
			if s.Name == "" {
				return fmt.Errorf("metric %q has a scalar with no name", m.Port)
			}
			key := m.Port + ScalarSeparator + s.Name
			if seenKeys[key] {
				return fmt.Errorf("duplicate scalar %q on metric %q", s.Name, m.Port)
			}
			seenKeys[key] = true
		}
	}
	return nil
}
