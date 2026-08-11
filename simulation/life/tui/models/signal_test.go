package models

import "testing"

func TestHealthStatus(t *testing.T) {
	// A reading with real limits on both sides: heart rate.
	heartRate := SignalMetadata{MinRange: 40, MaxRange: 200, HealthMin: 50, HealthMax: 100}

	// A reading whose healthy state is the bottom of its scale: an empty bladder.
	bladder := SignalMetadata{MinRange: 0, MaxRange: 100, HealthMin: 0, HealthMax: 90}

	tests := []struct {
		name     string
		metadata SignalMetadata
		value    float64
		want     HealthLevel
	}{
		{"comfortably normal", heartRate, 75, HealthNormal},
		{"below the healthy band", heartRate, 45, HealthCritical},
		{"above the healthy band", heartRate, 150, HealthCritical},
		{"approaching the lower bound", heartRate, 53, HealthWarning},
		{"approaching the upper bound", heartRate, 97, HealthWarning},

		// The case that put a caution marker on the best possible reading.
		{"empty is the healthiest a bladder gets", bladder, 0, HealthNormal},
		{"a nearly full bladder still warns", bladder, 85, HealthWarning},
		{"an overfull bladder is critical", bladder, 95, HealthCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.metadata.HealthStatus(tt.value); got != tt.want {
				t.Fatalf("HealthStatus(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
