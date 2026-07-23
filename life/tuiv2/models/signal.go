package models

import (
	"sync"
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

// SignalMetadata contains display information for a signal
type SignalMetadata struct {
	Key           string
	Label         string
	Unit          string
	MinRange      float64
	MaxRange      float64
	HealthMin     float64 // Below this is concerning
	HealthMax     float64 // Above this is concerning
	DecimalPlaces int
}

// SignalRegistry holds metadata for all known signals
var SignalRegistry = map[string]SignalMetadata{
	// Cardiovascular
	"human-Leon::heart_rate": {
		Key:           "human-Leon::heart_rate",
		Label:         "Heart Rate",
		Unit:          "BPM",
		MinRange:      40,
		MaxRange:      200,
		HealthMin:     50,
		HealthMax:     100,
		DecimalPlaces: 0,
	},
	"human-Leon::blood_o2_level": {
		Key:           "human-Leon::blood_o2_level",
		Label:         "Blood O₂",
		Unit:          "%",
		MinRange:      0,
		MaxRange:      100,
		HealthMin:     90,
		HealthMax:     100,
		DecimalPlaces: 1,
	},
	"human-Leon::blood_co2_level": {
		Key:           "human-Leon::blood_co2_level",
		Label:         "Blood CO₂",
		Unit:          "%",
		MinRange:      0,
		MaxRange:      100,
		HealthMin:     30,
		HealthMax:     45,
		DecimalPlaces: 1,
	},
	"human-Leon::heart_cardiac_activation": {
		Key:           "human-Leon::heart_cardiac_activation",
		Label:         "Cardiac Activation",
		Unit:          "",
		MinRange:      0,
		MaxRange:      1,
		HealthMin:     0,
		HealthMax:     1,
		DecimalPlaces: 3,
	},

	// Respiratory
	"human-Leon::respiratory_rate": {
		Key:           "human-Leon::respiratory_rate",
		Label:         "Resp Rate",
		Unit:          "/min",
		MinRange:      8,
		MaxRange:      30,
		HealthMin:     10,
		HealthMax:     20,
		DecimalPlaces: 0,
	},
	"human-Leon::pleural_pressure": {
		Key:           "human-Leon::pleural_pressure",
		Label:         "Pleural P",
		Unit:          "cmH₂O",
		MinRange:      -10,
		MaxRange:      0,
		HealthMin:     -8,
		HealthMax:     -3,
		DecimalPlaces: 1,
	},
	"human-Leon::lung_left_volume": {
		Key:           "human-Leon::lung_left_volume",
		Label:         "L Lung Vol",
		Unit:          "mL",
		MinRange:      600,
		MaxRange:      3000,
		HealthMin:     1000,
		HealthMax:     2500,
		DecimalPlaces: 0,
	},
	"human-Leon::lung_left_flow": {
		Key:           "human-Leon::lung_left_flow",
		Label:         "L Lung Flow",
		Unit:          "mL/s",
		MinRange:      -500,
		MaxRange:      500,
		HealthMin:     -400,
		HealthMax:     400,
		DecimalPlaces: 0,
	},
	"human-Leon::lung_right_volume": {
		Key:           "human-Leon::lung_right_volume",
		Label:         "R Lung Vol",
		Unit:          "mL",
		MinRange:      600,
		MaxRange:      3000,
		HealthMin:     1000,
		HealthMax:     2500,
		DecimalPlaces: 0,
	},
	"human-Leon::lung_right_flow": {
		Key:           "human-Leon::lung_right_flow",
		Label:         "R Lung Flow",
		Unit:          "mL/s",
		MinRange:      -500,
		MaxRange:      500,
		HealthMin:     -400,
		HealthMax:     400,
		DecimalPlaces: 0,
	},

	// Nervous
	"human-Leon::brain_activity": {
		Key:           "human-Leon::brain_activity",
		Label:         "Brain Activity",
		Unit:          "",
		MinRange:      0,
		MaxRange:      1,
		HealthMin:     0.1,
		HealthMax:     0.8,
		DecimalPlaces: 2,
	},
	"human-Leon::brain_activity_trend": {
		Key:           "human-Leon::brain_activity_trend",
		Label:         "Brain Trend",
		Unit:          "",
		MinRange:      0,
		MaxRange:      1,
		HealthMin:     0.1,
		HealthMax:     0.8,
		DecimalPlaces: 2,
	},

	// Status
	"human-Leon::is_alive": {
		Key:           "human-Leon::is_alive",
		Label:         "Status",
		Unit:          "",
		MinRange:      0,
		MaxRange:      1,
		HealthMin:     1,
		HealthMax:     1,
		DecimalPlaces: 0,
	},
	"human-Leon::body_temperature": {
		Key:           "human-Leon::body_temperature",
		Label:         "Body Temp",
		Unit:          "°C",
		MinRange:      35,
		MaxRange:      42,
		HealthMin:     36.5,
		HealthMax:     37.5,
		DecimalPlaces: 1,
	},
}

// HealthStatus returns the health status of a value
func (m SignalMetadata) HealthStatus(value float64) HealthLevel {
	if value < m.HealthMin || value > m.HealthMax {
		return HealthCritical
	}

	// Warning zone: within 10% of health boundaries
	margin := (m.HealthMax - m.HealthMin) * 0.1
	if value < m.HealthMin+margin || value > m.HealthMax-margin {
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
