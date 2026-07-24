package models

import (
	"sync"

	"github.com/hovsep/fmesh-examples/life/telemetry"
)

// SignalData represents a single signal's time-series data
type SignalData struct {
	Key    string    // e.g. "human-Leon::heart_rate"
	Values []float64 // Ring buffer of values
	Head   int       // Current write position in ring buffer
	Size   int       // Number of values written (min(writes, maxSize))
	mu     sync.RWMutex
}

// NewSignalData creates a new signal data store with a ring buffer
func NewSignalData(key string, maxSize int) *SignalData {
	return &SignalData{
		Key:    key,
		Values: make([]float64, maxSize),
		Head:   0,
		Size:   0,
	}
}

// Add appends a new value to the ring buffer
func (s *SignalData) Add(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Values[s.Head] = value
	s.Head = (s.Head + 1) % len(s.Values)
	if s.Size < len(s.Values) {
		s.Size++
	}
}

// Latest returns the most recent value
func (s *SignalData) Latest() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Size == 0 {
		return 0
	}

	// Head points to next write position, so latest is Head-1
	idx := (s.Head - 1 + len(s.Values)) % len(s.Values)
	return s.Values[idx]
}

// GetAll returns all values in chronological order (oldest to newest)
func (s *SignalData) GetAll() []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Size == 0 {
		return []float64{}
	}

	result := make([]float64, s.Size)

	// If buffer not full yet, just return from start
	if s.Size < len(s.Values) {
		copy(result, s.Values[:s.Size])
		return result
	}

	// Buffer is full, need to unwrap from Head position
	tailIdx := s.Head
	copy(result, s.Values[tailIdx:])
	copy(result[len(s.Values)-tailIdx:], s.Values[:tailIdx])

	return result
}

// GetLast returns the last N values in chronological order
func (s *SignalData) GetLast(n int) []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Size == 0 {
		return []float64{}
	}

	count := n
	if count > s.Size {
		count = s.Size
	}

	result := make([]float64, count)

	// Start from most recent and work backwards
	for i := 0; i < count; i++ {
		idx := (s.Head - 1 - i + len(s.Values)) % len(s.Values)
		result[count-1-i] = s.Values[idx]
	}

	return result
}

// SignalMetadata contains display information for a signal.
//
// It mirrors telemetry.Display, which is the source of truth: the mesh and this
// UI derive their view of a metric from the same catalog entry, so a value can
// never arrive on the wire with no idea how to draw it.
type SignalMetadata struct {
	Key           string
	Label         string
	Unit          string
	MinRange      float64
	MaxRange      float64
	HealthMin     float64 // Below this is concerning
	HealthMax     float64 // Above this is concerning
	DecimalPlaces int
	View          telemetry.View
}

// SignalRegistry holds metadata for every signal the body publishes, built from
// telemetry.Catalog.
var SignalRegistry = buildSignalRegistry(telemetry.DefaultSubject)

func buildSignalRegistry(subject string) map[string]SignalMetadata {
	signals := telemetry.Signals(subject)

	registry := make(map[string]SignalMetadata, len(signals))
	for _, s := range signals {
		registry[s.Key] = SignalMetadata{
			Key:           s.Key,
			Label:         s.Label,
			Unit:          s.Unit,
			MinRange:      s.Min,
			MaxRange:      s.Max,
			HealthMin:     s.HealthMin,
			HealthMax:     s.HealthMax,
			DecimalPlaces: s.Decimals,
			View:          s.View,
		}
	}
	return registry
}

// KeysForView returns the registry keys belonging to one view, in catalog order
// so a screen's layout stays stable between runs.
func KeysForView(subject string, view telemetry.View) []string {
	var keys []string
	for _, s := range telemetry.Signals(subject) {
		if s.View == view {
			keys = append(keys, s.Key)
		}
	}
	return keys
}

// HealthStatus returns the health status of a value
func (m SignalMetadata) HealthStatus(value float64) HealthLevel {
	if value < m.HealthMin || value > m.HealthMax {
		return HealthCritical
	}

	// Warning zone: within 10% of a health boundary, but only where that boundary
	// is a real limit rather than the end of the scale.
	//
	// An empty bladder or bowel sits exactly at the bottom of its range, and that
	// is the healthiest it gets; warning about it would put a caution marker on
	// the best possible reading.
	margin := (m.HealthMax - m.HealthMin) * 0.1
	if m.HealthMin > m.MinRange && value < m.HealthMin+margin {
		return HealthWarning
	}
	if m.HealthMax < m.MaxRange && value > m.HealthMax-margin {
		return HealthWarning
	}

	return HealthNormal
}

// HealthLevel represents the health status of a signal
type HealthLevel int

const (
	HealthNormal HealthLevel = iota
	HealthWarning
	HealthCritical
)

func (h HealthLevel) String() string {
	switch h {
	case HealthNormal:
		return "✓"
	case HealthWarning:
		return "⚠"
	case HealthCritical:
		return "✗"
	default:
		return "?"
	}
}
