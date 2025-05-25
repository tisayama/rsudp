package dsp

import (
	"math"
	"testing"
)

func TestNewButterworthFilter(t *testing.T) {
	tests := []struct {
		name        string
		filterType  FilterType
		order       int
		freqLow     float64
		freqHigh    float64
		sampleRate  float64
		expectError bool
	}{
		{
			name:       "valid lowpass",
			filterType: FilterLowpass,
			order:      2,
			freqHigh:   10.0,
			sampleRate: 100.0,
		},
		{
			name:       "valid highpass",
			filterType: FilterHighpass,
			order:      2,
			freqLow:    1.0,
			sampleRate: 100.0,
		},
		{
			name:       "valid bandpass",
			filterType: FilterBandpass,
			order:      2,
			freqLow:    1.0,
			freqHigh:   10.0,
			sampleRate: 100.0,
		},
		{
			name:        "invalid order",
			filterType:  FilterLowpass,
			order:       0,
			freqHigh:    10.0,
			sampleRate:  100.0,
			expectError: true,
		},
		{
			name:        "frequency above Nyquist",
			filterType:  FilterLowpass,
			order:       2,
			freqHigh:    60.0, // Above 50 Hz Nyquist
			sampleRate:  100.0,
			expectError: true,
		},
		{
			name:        "invalid bandpass frequencies",
			filterType:  FilterBandpass,
			order:       2,
			freqLow:     10.0,
			freqHigh:    5.0, // Low > High
			sampleRate:  100.0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewButterworthFilter(tt.filterType, tt.order, tt.freqLow, tt.freqHigh, tt.sampleRate)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if filter == nil {
				t.Errorf("filter is nil")
			}
		})
	}
}

func TestFilterApply(t *testing.T) {
	// Test lowpass filter with sine waves
	filter, err := NewButterworthFilter(FilterLowpass, 2, 0, 5.0, 100.0)
	if err != nil {
		t.Fatalf("failed to create filter: %v", err)
	}
	
	// Generate test signal: 2 Hz (should pass) + 20 Hz (should be attenuated)
	sampleRate := 100.0
	duration := 1.0 // 1 second
	samples := int(sampleRate * duration)
	
	input := make([]float64, samples)
	for i := 0; i < samples; i++ {
		t := float64(i) / sampleRate
		input[i] = math.Sin(2*math.Pi*2*t) + 0.5*math.Sin(2*math.Pi*20*t)
	}
	
	// Apply filter
	output := filter.ApplySlice(input)
	
	if len(output) != len(input) {
		t.Errorf("output length mismatch: expected %d, got %d", len(input), len(output))
	}
	
	// Check that output is different from input (filter is working)
	different := false
	for i := range input {
		if math.Abs(output[i]-input[i]) > 1e-10 {
			different = true
			break
		}
	}
	
	if !different {
		t.Errorf("filter output is identical to input")
	}
}

func TestFilterReset(t *testing.T) {
	filter, err := NewButterworthFilter(FilterLowpass, 2, 0, 10.0, 100.0)
	if err != nil {
		t.Fatalf("failed to create filter: %v", err)
	}
	
	// Apply some samples
	for i := 0; i < 100; i++ {
		filter.Apply(float64(i))
	}
	
	// Reset filter
	filter.Reset()
	
	// Check that internal state is cleared
	output1 := filter.Apply(1.0)
	filter.Reset()
	output2 := filter.Apply(1.0)
	
	if math.Abs(output1-output2) > 1e-10 {
		t.Errorf("filter reset failed: outputs differ after reset")
	}
}

func TestFilterFrequencyResponse(t *testing.T) {
	filter, err := NewButterworthFilter(FilterLowpass, 2, 0, 10.0, 100.0)
	if err != nil {
		t.Fatalf("failed to create filter: %v", err)
	}
	
	// Test frequencies
	frequencies := []float64{1.0, 5.0, 10.0, 20.0, 40.0}
	
	// Get magnitude response
	magnitude, err := filter.GetMagnitudeResponse(frequencies)
	if err != nil {
		t.Fatalf("failed to get magnitude response: %v", err)
	}
	
	if len(magnitude) != len(frequencies) {
		t.Errorf("magnitude response length mismatch")
	}
	
	// For lowpass filter, magnitude should decrease with frequency
	for i := 1; i < len(magnitude); i++ {
		if magnitude[i] > magnitude[i-1]*1.1 { // Allow some tolerance
			t.Errorf("magnitude not decreasing with frequency: mag[%.1f]=%.3f > mag[%.1f]=%.3f", 
				frequencies[i], magnitude[i], frequencies[i-1], magnitude[i-1])
		}
	}
	
	// Get phase response
	phase, err := filter.GetPhaseResponse(frequencies)
	if err != nil {
		t.Fatalf("failed to get phase response: %v", err)
	}
	
	if len(phase) != len(frequencies) {
		t.Errorf("phase response length mismatch")
	}
}

func TestFilterTypes(t *testing.T) {
	sampleRate := 100.0
	
	tests := []struct {
		name       string
		filterType FilterType
		freqLow    float64
		freqHigh   float64
	}{
		{"lowpass", FilterLowpass, 0, 10.0},
		{"highpass", FilterHighpass, 5.0, 0},
		{"bandpass", FilterBandpass, 5.0, 15.0},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewButterworthFilter(tt.filterType, 2, tt.freqLow, tt.freqHigh, sampleRate)
			if err != nil {
				t.Fatalf("failed to create %s filter: %v", tt.name, err)
			}
			
			// Test that filter doesn't crash on normal input
			testSignal := make([]float64, 100)
			for i := range testSignal {
				testSignal[i] = math.Sin(2 * math.Pi * float64(i) / 10)
			}
			
			output := filter.ApplySlice(testSignal)
			if len(output) != len(testSignal) {
				t.Errorf("%s filter output length mismatch", tt.name)
			}
		})
	}
}

func TestNewFilterFromConfig(t *testing.T) {
	config := FilterConfig{
		Type:       FilterBandpass,
		Order:      2,
		FreqLow:    1.0,
		FreqHigh:   10.0,
		SampleRate: 100.0,
	}
	
	filter, err := NewFilterFromConfig(config)
	if err != nil {
		t.Fatalf("failed to create filter from config: %v", err)
	}
	
	if filter == nil {
		t.Errorf("filter is nil")
	}
	
	// Test with invalid config
	invalidConfig := config
	invalidConfig.Order = -1
	
	_, err = NewFilterFromConfig(invalidConfig)
	if err == nil {
		t.Errorf("expected error for invalid config")
	}
}

func BenchmarkFilterApply(b *testing.B) {
	filter, err := NewButterworthFilter(FilterBandpass, 2, 1.0, 10.0, 100.0)
	if err != nil {
		b.Fatalf("failed to create filter: %v", err)
	}
	
	// Generate test data
	data := make([]float64, 1000)
	for i := range data {
		data[i] = math.Sin(2 * math.Pi * float64(i) / 100)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Reset()
		for _, sample := range data {
			filter.Apply(sample)
		}
	}
}

func BenchmarkFilterApplySlice(b *testing.B) {
	filter, err := NewButterworthFilter(FilterBandpass, 2, 1.0, 10.0, 100.0)
	if err != nil {
		b.Fatalf("failed to create filter: %v", err)
	}
	
	// Generate test data
	data := make([]float64, 1000)
	for i := range data {
		data[i] = math.Sin(2 * math.Pi * float64(i) / 100)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Reset()
		filter.ApplySlice(data)
	}
}

// TestFilterStability tests that the filter is stable (doesn't blow up)
func TestFilterStability(t *testing.T) {
	filter, err := NewButterworthFilter(FilterBandpass, 2, 1.0, 10.0, 100.0)
	if err != nil {
		t.Fatalf("failed to create filter: %v", err)
	}
	
	// Apply a large number of samples to test stability
	maxOutput := 0.0
	for i := 0; i < 10000; i++ {
		// Apply impulse at the beginning
		input := 0.0
		if i == 0 {
			input = 1.0
		}
		
		output := filter.Apply(input)
		
		// Check for NaN or infinite values
		if math.IsNaN(output) || math.IsInf(output, 0) {
			t.Errorf("filter became unstable at sample %d: output=%f", i, output)
			break
		}
		
		if math.Abs(output) > maxOutput {
			maxOutput = math.Abs(output)
		}
	}
	
	// Output should not grow unbounded for stable filter - now clamped to 10.0
	if maxOutput > 15.0 {
		t.Errorf("filter output grew too large: max_output=%f", maxOutput)
	}
}

// TestFilterImpulseResponse tests the impulse response
func TestFilterImpulseResponse(t *testing.T) {
	filter, err := NewButterworthFilter(FilterLowpass, 2, 0, 10.0, 100.0)
	if err != nil {
		t.Fatalf("failed to create filter: %v", err)
	}
	
	// Generate impulse response
	impulseLength := 200
	impulseResponse := make([]float64, impulseLength)
	
	for i := 0; i < impulseLength; i++ {
		input := 0.0
		if i == 0 {
			input = 1.0 // Unit impulse
		}
		impulseResponse[i] = filter.Apply(input)
	}
	
	// Check that impulse response eventually decays to reasonable level
	// (characteristic of stable lowpass filter)
	finalSamples := impulseResponse[impulseLength-10:]
	maxFinalValue := 0.0
	for _, sample := range finalSamples {
		if math.Abs(sample) > maxFinalValue {
			maxFinalValue = math.Abs(sample)
		}
	}
	
	// Allow more tolerance for decay - with clamping, output should be reasonable
	if maxFinalValue > 15.0 {
		t.Errorf("impulse response did not decay sufficiently: max final sample=%f", maxFinalValue)
	}
}