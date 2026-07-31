package models

import (
	"sync"
	"time"

	"github.com/hovsep/fmesh-examples/life/telemetry"
)

// Telemetry keys for the simulation clock, published as scalars of the habitat's
// tick signal. Views use these to show and animate against simulated time.
const (
	SimDurationKey  = "time::tick:sim_duration_ms"
	SimTickCountKey = "time::tick:tick_count"
)

// AppState holds all application state.
//
// It is written from the background ingestion goroutine and read from the
// Bubble Tea render goroutine, so all access goes through the mutex.
type AppState struct {
	Signals     map[string]*SignalData // Key -> SignalData
	StartTime   time.Time
	TickCount   int64
	IsAlive     bool
	CurrentView ViewType
	lastError   error
	mu          sync.RWMutex
}

// ViewType is which dashboard screen is showing. It is telemetry.View, so the
// tabs and the catalog can never disagree about which metrics belong where.
type ViewType = telemetry.View

// Views are the screens, in tab order.
// Screens is the tab bar, in order: what each screen is and what it is called.
//
// One list rather than three. The order of the tabs, the label on each and the
// set that exists were previously a slice, a switch and a second switch in the
// app, which is three places to add a screen and two places to forget.
var Screens = []Screen{
	{telemetry.ViewOverview, "Overview"},
	{telemetry.ViewCardiovascular, "Cardiovascular"},
	{telemetry.ViewRespiratory, "Respiratory"},
	{telemetry.ViewNervous, "Nervous"},
	{telemetry.ViewMetabolic, "Metabolic"},
	{telemetry.ViewAffect, "Feelings"},
	{telemetry.ViewBody, "Body"},
}

// Screen is one tab: the view it shows and the label on it.
type Screen struct {
	View  ViewType
	Title string
}

// Views lists the screens in tab order.
func Views() []ViewType {
	out := make([]ViewType, 0, len(Screens))
	for _, s := range Screens {
		out = append(out, s.View)
	}
	return out
}

const (
	ViewOverview       = telemetry.ViewOverview
	ViewCardiovascular = telemetry.ViewCardiovascular
	ViewRespiratory    = telemetry.ViewRespiratory
	ViewNervous        = telemetry.ViewNervous
	ViewMetabolic      = telemetry.ViewMetabolic
	ViewAffect         = telemetry.ViewAffect
	ViewBody           = telemetry.ViewBody
)

// ViewName is the tab label for a screen.
func ViewName(v ViewType) string {
	for _, s := range Screens {
		if s.View == v {
			return s.Title
		}
	}
	return "Unknown"
}

// NewAppState creates a new application state
func NewAppState(maxDataPoints int) *AppState {
	state := &AppState{
		Signals:     make(map[string]*SignalData),
		StartTime:   time.Now(),
		TickCount:   0,
		IsAlive:     true,
		CurrentView: ViewOverview,
	}

	// Pre-initialize signals from registry
	for key := range SignalRegistry {
		state.Signals[key] = NewSignalData(key, maxDataPoints)
	}

	return state
}

// UpdateSignal updates or creates a signal with a new value
func (s *AppState) UpdateSignal(key string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	signal, exists := s.Signals[key]
	if !exists {
		// Create new signal if not in registry
		signal = NewSignalData(key, 2000)
		s.Signals[key] = signal
	}

	signal.Add(value)

	// Update special state tracking
	if key == "human-Leon::is_alive" {
		s.IsAlive = value > 0
	}
}

// GetSignal safely retrieves a signal
func (s *AppState) GetSignal(key string) (*SignalData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	signal, exists := s.Signals[key]
	return signal, exists
}

// GetLatestValue safely retrieves the latest value for a signal
func (s *AppState) GetLatestValue(key string) float64 {
	signal, exists := s.GetSignal(key)
	if !exists {
		return 0
	}
	return signal.Latest()
}

// IncrementTick increments the tick counter
func (s *AppState) IncrementTick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TickCount++
}

// GetTickCount safely returns the current tick count
func (s *AppState) GetTickCount() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TickCount
}

// GetIsAlive safely returns the alive status
func (s *AppState) GetIsAlive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsAlive
}

// SetLastError records the most recent protocol error
func (s *AppState) SetLastError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

// GetLastError returns the most recent protocol error, if any
func (s *AppState) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// GetElapsedTime returns time since start
func (s *AppState) GetElapsedTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.StartTime)
}

// SetView changes the current view
func (s *AppState) SetView(view ViewType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentView = view
}

// GetView returns the current view
func (s *AppState) GetView() ViewType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentView
}

// NextView cycles to the next view
func (s *AppState) NextView() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentView = (s.CurrentView + 1) % ViewType(len(Screens))
}

// PrevView cycles to the previous view
func (s *AppState) PrevView() {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := ViewType(len(Screens))
	s.CurrentView = (s.CurrentView - 1 + count) % count
}
