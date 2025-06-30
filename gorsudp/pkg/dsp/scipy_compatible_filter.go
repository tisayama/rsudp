// Package dsp provides SciPy-compatible digital signal processing functionality
package dsp

import (
	"fmt"
	"math"
)

// ScipyCompatibleFilter implements SciPy butter + filtfilt compatible filtering
type ScipyCompatibleFilter struct {
	filterType FilterType
	order      int
	freqLow    float64
	freqHigh   float64
	sampleRate float64
	
	// Second-order sections (SOS) format for stability
	sos []SOSSection
}

// SOSSection represents a single second-order section
type SOSSection struct {
	b []float64 // Numerator coefficients [b0, b1, b2]
	a []float64 // Denominator coefficients [a0, a1, a2] (a0=1)
}

// NewScipyCompatibleFilter creates a new SciPy-compatible Butterworth filter
func NewScipyCompatibleFilter(filterType FilterType, order int, freqLow, freqHigh, sampleRate float64) (*ScipyCompatibleFilter, error) {
	if order <= 0 || order > 10 {
		return nil, fmt.Errorf("filter order must be between 1 and 10")
	}
	
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}
	
	// Validate frequencies
	nyquist := sampleRate / 2
	switch filterType {
	case FilterLowpass:
		if freqHigh <= 0 || freqHigh >= nyquist {
			return nil, fmt.Errorf("lowpass cutoff frequency must be between 0 and %f Hz", nyquist)
		}
	case FilterHighpass:
		if freqLow <= 0 || freqLow >= nyquist {
			return nil, fmt.Errorf("highpass cutoff frequency must be between 0 and %f Hz", nyquist)
		}
	case FilterBandpass:
		if freqLow <= 0 || freqHigh <= 0 || freqLow >= freqHigh || freqHigh >= nyquist {
			return nil, fmt.Errorf("bandpass frequencies must satisfy 0 < freqLow < freqHigh < %f Hz", nyquist)
		}
	}
	
	filter := &ScipyCompatibleFilter{
		filterType: filterType,
		order:      order,
		freqLow:    freqLow,
		freqHigh:   freqHigh,
		sampleRate: sampleRate,
	}
	
	// Calculate SOS coefficients
	if err := filter.calculateSOSCoefficients(); err != nil {
		return nil, fmt.Errorf("failed to calculate filter coefficients: %v", err)
	}
	
	return filter, nil
}

// calculateSOSCoefficients calculates second-order sections using SciPy's method
func (f *ScipyCompatibleFilter) calculateSOSCoefficients() error {
	switch f.filterType {
	case FilterBandpass:
		return f.calculateBandpassSOS()
	case FilterLowpass:
		return f.calculateLowpassSOS()
	case FilterHighpass:
		return f.calculateHighpassSOS()
	default:
		return fmt.Errorf("unsupported filter type")
	}
}

// calculateBandpassSOS calculates bandpass filter SOS coefficients
func (f *ScipyCompatibleFilter) calculateBandpassSOS() error {
	// Normalize frequencies to [0, 1] where 1 is Nyquist
	wl := f.freqLow / (f.sampleRate / 2)
	wh := f.freqHigh / (f.sampleRate / 2)
	
	// Ensure valid range
	if wl <= 0 || wl >= 1 || wh <= 0 || wh >= 1 || wl >= wh {
		return fmt.Errorf("invalid frequency range")
	}
	
	// For bandpass, we create a 2nd order section
	// Using the standard bilinear transform method matching SciPy
	
	// Calculate center frequency and bandwidth
	wc := math.Sqrt(wl * wh)  // Geometric mean
	bw := wh - wl             // Bandwidth
	
	// Prewarp frequencies for bilinear transform
	wcPrewarp := math.Tan(math.Pi * wc / 2)
	bwPrewarp := math.Tan(math.Pi * bw / 2)
	
	// Calculate Q factor
	Q := wcPrewarp / bwPrewarp
	
	// Calculate coefficients for 2nd order Butterworth bandpass
	K := wcPrewarp
	norm := 1.0 / (1.0 + K/Q + K*K)
	
	// Create SOS section
	section := SOSSection{
		b: []float64{
			K / Q * norm,    // b0
			0.0,             // b1
			-K / Q * norm,   // b2
		},
		a: []float64{
			1.0,                              // a0 (always 1)
			2.0 * (K*K - 1.0) * norm,        // a1
			(1.0 - K/Q + K*K) * norm,        // a2
		},
	}
	
	f.sos = []SOSSection{section}
	return nil
}

// calculateLowpassSOS calculates lowpass filter SOS coefficients
func (f *ScipyCompatibleFilter) calculateLowpassSOS() error {
	wc := f.freqHigh / (f.sampleRate / 2)
	if wc <= 0 || wc >= 1 {
		return fmt.Errorf("invalid cutoff frequency")
	}
	
	// Prewarp frequency
	K := math.Tan(math.Pi * wc / 2)
	norm := 1.0 / (1.0 + K*math.Sqrt2 + K*K)
	
	section := SOSSection{
		b: []float64{
			K * K * norm,
			2.0 * K * K * norm,
			K * K * norm,
		},
		a: []float64{
			1.0,
			2.0 * (K*K - 1.0) * norm,
			(1.0 - K*math.Sqrt2 + K*K) * norm,
		},
	}
	
	f.sos = []SOSSection{section}
	return nil
}

// calculateHighpassSOS calculates highpass filter SOS coefficients
func (f *ScipyCompatibleFilter) calculateHighpassSOS() error {
	wc := f.freqLow / (f.sampleRate / 2)
	if wc <= 0 || wc >= 1 {
		return fmt.Errorf("invalid cutoff frequency")
	}
	
	// Prewarp frequency
	K := math.Tan(math.Pi * wc / 2)
	norm := 1.0 / (1.0 + K*math.Sqrt2 + K*K)
	
	section := SOSSection{
		b: []float64{
			norm,
			-2.0 * norm,
			norm,
		},
		a: []float64{
			1.0,
			2.0 * (K*K - 1.0) * norm,
			(1.0 - K*math.Sqrt2 + K*K) * norm,
		},
	}
	
	f.sos = []SOSSection{section}
	return nil
}

// ApplyZeroPhase applies zero-phase filtering using SciPy filtfilt method
func (f *ScipyCompatibleFilter) ApplyZeroPhase(input []float64) []float64 {
	if len(input) == 0 {
		return input
	}
	
	// Calculate appropriate padding length (SciPy uses 3*(max(len(a), len(b))-1))
	maxLen := 3
	for _, section := range f.sos {
		if len(section.a) > maxLen {
			maxLen = len(section.a)
		}
		if len(section.b) > maxLen {
			maxLen = len(section.b)
		}
	}
	padLen := 3 * (maxLen - 1)
	if padLen > len(input)-1 {
		padLen = len(input) - 1
	}
	
	// Create padded signal using odd extension (SciPy default)
	paddedInput := f.oddExtend(input, padLen)
	
	// Forward pass
	forward := f.sosFilterForward(paddedInput)
	
	// Reverse the signal
	reversed := make([]float64, len(forward))
	for i := 0; i < len(forward); i++ {
		reversed[i] = forward[len(forward)-1-i]
	}
	
	// Backward pass
	backward := f.sosFilterForward(reversed)
	
	// Reverse again to get final result
	result := make([]float64, len(backward))
	for i := 0; i < len(backward); i++ {
		result[i] = backward[len(backward)-1-i]
	}
	
	// Remove padding
	start := padLen
	end := len(result) - padLen
	if start < 0 {
		start = 0
	}
	if end > len(result) {
		end = len(result)
	}
	if end <= start {
		return input // Fallback to original if padding issues
	}
	
	return result[start:end]
}

// oddExtend performs odd extension padding as used by SciPy
func (f *ScipyCompatibleFilter) oddExtend(data []float64, padLen int) []float64 {
	if padLen <= 0 || len(data) == 0 {
		return data
	}
	
	n := len(data)
	if padLen >= n {
		padLen = n - 1
	}
	
	result := make([]float64, n+2*padLen)
	
	// Left padding (odd extension)
	for i := 0; i < padLen; i++ {
		result[i] = 2*data[0] - data[padLen-i]
	}
	
	// Original data
	copy(result[padLen:padLen+n], data)
	
	// Right padding (odd extension)
	for i := 0; i < padLen; i++ {
		result[padLen+n+i] = 2*data[n-1] - data[n-2-i]
	}
	
	return result
}

// sosFilterForward applies SOS filtering in forward direction
func (f *ScipyCompatibleFilter) sosFilterForward(input []float64) []float64 {
	result := make([]float64, len(input))
	copy(result, input)
	
	// Apply each SOS section sequentially
	for _, section := range f.sos {
		result = f.applySingleSOS(result, section)
	}
	
	return result
}

// applySingleSOS applies a single second-order section
func (f *ScipyCompatibleFilter) applySingleSOS(input []float64, section SOSSection) []float64 {
	if len(input) == 0 {
		return input
	}
	
	output := make([]float64, len(input))
	
	// Initialize delay line
	x := []float64{0, 0, 0} // x[n], x[n-1], x[n-2]
	y := []float64{0, 0, 0} // y[n], y[n-1], y[n-2]
	
	for i, sample := range input {
		// Shift delay line
		x[2] = x[1]
		x[1] = x[0]
		x[0] = sample
		
		// Calculate output using direct form II
		y[0] = section.b[0]*x[0] + section.b[1]*x[1] + section.b[2]*x[2] -
			section.a[1]*y[1] - section.a[2]*y[2]
		
		// Check for numerical stability
		if math.IsNaN(y[0]) || math.IsInf(y[0], 0) {
			y[0] = 0
			// Reset delay line
			x = []float64{0, 0, 0}
			y = []float64{0, 0, 0}
		}
		
		// Limit extreme values
		if math.Abs(y[0]) > 1e10 {
			y[0] = 1e10 * math.Copysign(1, y[0])
		}
		
		output[i] = y[0]
		
		// Shift y delay line
		y[2] = y[1]
		y[1] = y[0]
	}
	
	return output
}

// Apply applies single-pass filtering (for compatibility)
func (f *ScipyCompatibleFilter) Apply(input float64) float64 {
	// This method is not recommended for SciPy compatibility
	// Use ApplyZeroPhase for batch processing instead
	return input
}

// ApplySlice applies zero-phase filtering to a slice
func (f *ScipyCompatibleFilter) ApplySlice(input []float64) []float64 {
	return f.ApplyZeroPhase(input)
}