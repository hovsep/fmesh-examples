package codec

import (
	"testing"
)

func TestBitsWithoutLastBit(t *testing.T) {
	tests := []struct {
		name     string
		input    Bits
		expected Bits
	}{
		{
			name:     "remove last bit from non-empty slice",
			input:    Bits{true, false, true},
			expected: Bits{true, false},
		},
		{
			name:     "remove last bit from single bit",
			input:    Bits{true},
			expected: Bits{},
		},
		{
			name:     "remove from empty slice",
			input:    Bits{},
			expected: Bits{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.WithoutLastBit()
			if !result.Equals(tt.expected) {
				t.Errorf("WithoutLastBit() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBitsWithLastBitSwitched(t *testing.T) {
	tests := []struct {
		name     string
		input    Bits
		expected Bits
	}{
		{
			name:     "switch last bit from true to false",
			input:    Bits{true, false, true},
			expected: Bits{true, false, false},
		},
		{
			name:     "switch last bit from false to true",
			input:    Bits{true, false, false},
			expected: Bits{true, false, true},
		},
		{
			name:     "switch single bit",
			input:    Bits{true},
			expected: Bits{false},
		},
		{
			name:     "switch from empty slice",
			input:    Bits{},
			expected: Bits{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.WithLastBitSwitched()
			if !result.Equals(tt.expected) {
				t.Errorf("WithLastBitSwitched() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBitsWithLastBitReplaced(t *testing.T) {
	tests := []struct {
		name     string
		input    Bits
		newBit   Bit
		expected Bits
	}{
		{
			name:     "replace last bit with true",
			input:    Bits{true, false, false},
			newBit:   true,
			expected: Bits{true, false, true},
		},
		{
			name:     "replace last bit with false",
			input:    Bits{true, false, true},
			newBit:   false,
			expected: Bits{true, false, false},
		},
		{
			name:     "replace single bit",
			input:    Bits{true},
			newBit:   false,
			expected: Bits{false},
		},
		{
			name:     "replace in empty slice",
			input:    Bits{},
			newBit:   true,
			expected: Bits{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.WithLastBitReplaced(tt.newBit)
			if !result.Equals(tt.expected) {
				t.Errorf("WithLastBitReplaced() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBitsImmutability(t *testing.T) {
	// Test that the original slice is not modified
	original := Bits{true, false, true}

	_ = original.WithoutLastBit()
	if !original.Equals(Bits{true, false, true}) {
		t.Errorf("WithoutLastBit() modified original slice")
	}

	_ = original.WithLastBitSwitched()
	if !original.Equals(Bits{true, false, true}) {
		t.Errorf("WithLastBitSwitched() modified original slice")
	}

	_ = original.WithLastBitReplaced(false)
	if !original.Equals(Bits{true, false, true}) {
		t.Errorf("WithLastBitReplaced() modified original slice")
	}
}
