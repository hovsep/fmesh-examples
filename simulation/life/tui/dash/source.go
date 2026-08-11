package dash

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/models"
)

// State adapts the telemetry the simulation publishes to what a widget reads.
//
// Widgets name a metric by its catalog port -- "heart_rate", not
// "human-Leon::heart_rate" -- and this is where the subject is put back on.
// That is the whole reason the adapter exists: a screen definition should not
// have to know whose body it is drawing, so renaming the subject cannot empty
// the dashboard.
type State struct {
	state   *models.AppState
	subject string
}

// NewState wraps the app state for a given subject.
func NewState(state *models.AppState, subject string) *State {
	return &State{state: state, subject: subject}
}

func (s *State) key(metric string) string {
	return s.subject + telemetry.PathSeparator + metric
}

// Value is the latest reading of a metric.
func (s *State) Value(metric string) float64 {
	return s.state.GetLatestValue(s.key(metric))
}

// Series is the recent history of a metric, oldest first.
func (s *State) Series(metric string) []float64 {
	sig, ok := s.state.GetSignal(s.key(metric))
	if !ok {
		return nil
	}
	return sig.GetAll()
}

// Scalar is one named number carried inside a composite signal.
func (s *State) Scalar(metric, scalar string) float64 {
	return s.state.GetLatestValue(s.key(metric) + telemetry.ScalarSeparator + scalar)
}

// Display is how the catalog says to draw a metric.
func (s *State) Display(metric string) (telemetry.Display, bool) {
	meta, ok := models.SignalRegistry[s.key(metric)]
	if !ok {
		return telemetry.Display{}, false
	}
	return telemetry.Display{
		Label:     meta.Label,
		Unit:      meta.Unit,
		Min:       meta.MinRange,
		Max:       meta.MaxRange,
		HealthMin: meta.HealthMin,
		HealthMax: meta.HealthMax,
		Decimals:  meta.DecimalPlaces,
	}, true
}

// Elapsed is how long the body has been simulated.
func (s *State) Elapsed() time.Duration { return s.state.GetElapsedTime() }

// Alive reports whether the body is still living.
func (s *State) Alive() bool { return s.state.GetIsAlive() }
