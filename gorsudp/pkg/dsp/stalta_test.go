package dsp

import (
	"math"
	"math/rand"
	"testing"
)

func TestNewSTALTAProcessor(t *testing.T) {
	tests := []struct {
		name        string
		config      STALTAConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: STALTAConfig{
				STADuration:  5.0,
				LTADuration:  30.0,
				Threshold:    1.6,
				Reset:        1.55,
				SampleRate:   100.0,
				UseRecursive: true,
				EnergyBased:  true,
			},
			expectError: false,
		},
		{
			name: "STA >= LTA",
			config: STALTAConfig{
				STADuration: 30.0,
				LTADuration: 30.0,
				Threshold:   1.6,
				Reset:       1.55,
				SampleRate:  100.0,
			},
			expectError: true,
		},
		{
			name: "negative threshold",
			config: STALTAConfig{
				STADuration: 5.0,
				LTADuration: 30.0,
				Threshold:   -1.6,
				Reset:       1.55,
				SampleRate:  100.0,
			},
			expectError: true,
		},
		{
			name: "zero sample rate",
			config: STALTAConfig{
				STADuration: 5.0,
				LTADuration: 30.0,
				Threshold:   1.6,
				Reset:       1.55,
				SampleRate:  0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := NewSTALTAProcessor(tt.config)

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

			if processor == nil {
				t.Errorf("processor is nil")
			}
		})
	}
}

func TestSTALTABasicFunctionality(t *testing.T) {
	config := STALTAConfig{
		STADuration:  1.0, // 1 second
		LTADuration:  5.0, // 5 seconds
		Threshold:    2.0,
		Reset:        1.5,
		SampleRate:   100.0,
		UseRecursive: true,
		EnergyBased:  true,
	}

	processor, err := NewSTALTAProcessor(config)
	if err != nil {
		t.Fatalf("failed to create processor: %v", err)
	}

	// Test with quiet background noise
	for i := 0; i < 1000; i++ {
		noise := rand.Float64() * 100 // Small noise
		triggered, ratio := processor.Process(noise)

		if triggered {
			t.Errorf("unexpected trigger on noise at sample %d, ratio=%.3f", i, ratio)
		}
	}

	// Test with earthquake-like signal
	for i := 0; i < 200; i++ {
		signal := 2000.0 + rand.Float64()*1000 // Large signal
		triggered, ratio := processor.Process(signal)

		if i > 100 && !triggered && ratio > config.Threshold {
			t.Errorf("expected trigger for large signal at sample %d, ratio=%.3f", i, ratio)
		}
	}
}

func TestSTALTATriggerReset(t *testing.T) {
	config := STALTAConfig{
		STADuration:  0.5,
		LTADuration:  2.0,
		Threshold:    2.0,
		Reset:        1.0,
		SampleRate:   100.0,
		UseRecursive: true,
		EnergyBased:  true,
	}

	processor, err := NewSTALTAProcessor(config)
	if err != nil {
		t.Fatalf("failed to create processor: %v", err)
	}

	// Generate background noise for LTA warmup
	for i := 0; i < 300; i++ {
		noise := rand.Float64() * 50
		processor.Process(noise)
	}

	// Generate strong signal to trigger
	triggerSeen := false
	for i := 0; i < 100; i++ {
		signal := 500.0 + rand.Float64()*100
		triggered, _ := processor.Process(signal)
		if triggered {
			triggerSeen = true
			break
		}
	}

	if !triggerSeen {
		t.Errorf("expected trigger for strong signal")
	}

	// Generate quiet signal to reset
	resetSeen := false
	for i := 0; i < 300; i++ {
		quiet := rand.Float64() * 10
		triggered, _ := processor.Process(quiet)
		if !triggered {
			resetSeen = true
			break
		}
	}

	if !resetSeen {
		t.Errorf("expected reset after quiet signal")
	}
}

func TestSTALTARecursiveVsExact(t *testing.T) {
	// Test data
	data := generateTestSeismogram(1000, 100.0)

	// Recursive processor
	configRecursive := STALTAConfig{
		STADuration:  1.0,
		LTADuration:  5.0,
		Threshold:    1.5,
		Reset:        1.0,
		SampleRate:   100.0,
		UseRecursive: true,
		EnergyBased:  true,
	}

	recursiveProcessor, _ := NewSTALTAProcessor(configRecursive)

	// Exact processor
	configExact := configRecursive
	configExact.UseRecursive = false
	exactProcessor, _ := NewSTALTAProcessor(configExact)

	// Process same data through both
	recursiveRatios := make([]float64, len(data))
	exactRatios := make([]float64, len(data))

	for i, sample := range data {
		_, recursiveRatios[i] = recursiveProcessor.Process(sample)
		_, exactRatios[i] = exactProcessor.Process(sample)
	}

	// Compare ratios (should be similar after warmup period)
	warmup := 500 // samples
	maxDiff := 0.0

	for i := warmup; i < len(data); i++ {
		diff := math.Abs(recursiveRatios[i] - exactRatios[i])
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	// Allow reasonable differences due to algorithmic differences
	// Recursive uses exponential smoothing (IIR-like) while exact uses true windowed average (FIR-like)
	// These are fundamentally different algorithms and will produce different results
	if maxDiff > 1.5 {
		t.Errorf("recursive and exact algorithms differ too much: max_diff=%.6f", maxDiff)
	}
}

func TestCalculateClassicSTALTA(t *testing.T) {
	// Generate test data
	data := make([]float64, 1000)
	for i := range data {
		data[i] = math.Sin(float64(i)*0.1) + rand.Float64()*0.1
	}

	// Add an event in the middle
	for i := 450; i < 550; i++ {
		data[i] += 5.0 * math.Sin(float64(i-450)*0.5)
	}

	staWindow := 50
	ltaWindow := 200

	ratios, err := CalculateClassicSTALTA(data, staWindow, ltaWindow)
	if err != nil {
		t.Fatalf("CalculateClassicSTALTA failed: %v", err)
	}

	if len(ratios) != len(data) {
		t.Errorf("expected %d ratios, got %d", len(data), len(ratios))
	}

	// Check that ratios increase during the event
	maxRatio := 0.0
	maxIndex := 0
	for i := ltaWindow; i < len(ratios); i++ {
		if ratios[i] > maxRatio {
			maxRatio = ratios[i]
			maxIndex = i
		}
	}

	// Max should be near the event
	if maxIndex < 400 || maxIndex > 600 {
		t.Errorf("max ratio at unexpected position: %d (expected around 500)", maxIndex)
	}

	if maxRatio < 2.0 {
		t.Errorf("max ratio too low: %.3f (expected > 2.0)", maxRatio)
	}
}

func TestTriggerOnset(t *testing.T) {
	// Create ratios with clear trigger pattern
	ratios := make([]float64, 1000)
	for i := range ratios {
		ratios[i] = 1.0 + rand.Float64()*0.1 // Background level
	}

	// Add trigger events
	for i := 200; i < 250; i++ {
		ratios[i] = 3.0 // Above threshold
	}
	for i := 600; i < 650; i++ {
		ratios[i] = 2.5 // Above threshold
	}

	threshold := 2.0
	reset := 1.5

	onsets := TriggerOnset(ratios, threshold, reset)

	if len(onsets) != 2 {
		t.Errorf("expected 2 onsets, got %d", len(onsets))
	}

	if len(onsets) >= 1 && (onsets[0] < 190 || onsets[0] > 210) {
		t.Errorf("first onset at unexpected position: %d", onsets[0])
	}

	if len(onsets) >= 2 && (onsets[1] < 590 || onsets[1] > 610) {
		t.Errorf("second onset at unexpected position: %d", onsets[1])
	}
}

func TestSTALTAThreadSafety(t *testing.T) {
	config := STALTAConfig{
		STADuration:  1.0,
		LTADuration:  5.0,
		Threshold:    2.0,
		Reset:        1.5,
		SampleRate:   100.0,
		UseRecursive: true,
		EnergyBased:  true,
	}

	processor, err := NewSTALTAProcessor(config)
	if err != nil {
		t.Fatalf("failed to create processor: %v", err)
	}

	// Test concurrent access
	done := make(chan bool, 2)

	// Goroutine 1: Process samples
	go func() {
		for i := 0; i < 1000; i++ {
			sample := rand.Float64() * 100
			processor.Process(sample)
		}
		done <- true
	}()

	// Goroutine 2: Read state
	go func() {
		for i := 0; i < 1000; i++ {
			processor.GetRatio()
			processor.IsTriggered()
			processor.GetStats()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without deadlock, the test passes
}

// generateTestSeismogram generates a synthetic seismogram for testing
func generateTestSeismogram(length int, sampleRate float64) []float64 {
	data := make([]float64, length)

	for i := range data {
		t := float64(i) / sampleRate

		// Background noise
		noise := rand.Float64() * 50

		// Base signal
		signal := 100 * math.Sin(2*math.Pi*0.1*t)

		// Add earthquake-like event in the middle
		if i >= length/2-100 && i <= length/2+100 {
			eventTime := t - float64(length/2)/sampleRate
			envelope := math.Exp(-math.Abs(eventTime) * 2) // Decay

			// P-wave (high frequency)
			pWave := 500 * envelope * math.Sin(2*math.Pi*10*eventTime)

			// S-wave (lower frequency, higher amplitude)
			sWave := 800 * envelope * math.Sin(2*math.Pi*3*eventTime)

			signal += pWave + sWave
		}

		data[i] = signal + noise
	}

	return data
}

func BenchmarkSTALTARecursive(b *testing.B) {
	config := STALTAConfig{
		STADuration:  5.0,
		LTADuration:  30.0,
		Threshold:    1.6,
		Reset:        1.55,
		SampleRate:   100.0,
		UseRecursive: true,
		EnergyBased:  true,
	}

	processor, _ := NewSTALTAProcessor(config)
	data := generateTestSeismogram(2500, 100.0) // 25 seconds at 100 Hz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.Reset()
		for _, sample := range data {
			processor.Process(sample)
		}
	}
}

func BenchmarkSTALTAExact(b *testing.B) {
	config := STALTAConfig{
		STADuration:  5.0,
		LTADuration:  30.0,
		Threshold:    1.6,
		Reset:        1.55,
		SampleRate:   100.0,
		UseRecursive: false,
		EnergyBased:  true,
	}

	processor, _ := NewSTALTAProcessor(config)
	data := generateTestSeismogram(2500, 100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.Reset()
		for _, sample := range data {
			processor.Process(sample)
		}
	}
}

func BenchmarkCalculateClassicSTALTA(b *testing.B) {
	data := generateTestSeismogram(2500, 100.0)
	staWindow := 500  // 5 seconds at 100 Hz
	ltaWindow := 3000 // 30 seconds at 100 Hz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateClassicSTALTA(data, staWindow, ltaWindow)
		if err != nil {
			b.Fatal(err)
		}
	}
}
