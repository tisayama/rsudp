// Package dsp provides digital signal processing functionality
package dsp

import (
	"fmt"
	"math"
)

// FilterType represents different types of digital filters
type FilterType int

const (
	FilterBandpass FilterType = iota
	FilterLowpass
	FilterHighpass
)

func (f FilterType) String() string {
	switch f {
	case FilterBandpass:
		return "bandpass"
	case FilterLowpass:
		return "lowpass"
	case FilterHighpass:
		return "highpass"
	default:
		return "unknown"
	}
}

// ButterworthFilter implements a Butterworth IIR filter
type ButterworthFilter struct {
	filterType FilterType
	order      int
	freqLow    float64 // Low cutoff frequency (Hz)
	freqHigh   float64 // High cutoff frequency (Hz)
	sampleRate float64 // Sample rate (Hz)

	// Filter coefficients (legacy)
	a []float64 // Denominator coefficients
	b []float64 // Numerator coefficients

	// Filter state (delay line) (legacy)
	x []float64 // Input history
	y []float64 // Output history
	
	// SciPy-compatible filter implementation
	scipyFilter *ScipyCompatibleFilter
}

// NewButterworthFilter creates a new Butterworth filter using SciPy-compatible implementation
func NewButterworthFilter(filterType FilterType, order int, freqLow, freqHigh, sampleRate float64) (*ButterworthFilter, error) {
	// Create SciPy-compatible filter
	scipyFilter, err := NewScipyCompatibleFilter(filterType, order, freqLow, freqHigh, sampleRate)
	if err != nil {
		return nil, err
	}

	// Create ButterworthFilter wrapper for backward compatibility
	filter := &ButterworthFilter{
		filterType: filterType,
		order:      order,
		freqLow:    freqLow,
		freqHigh:   freqHigh,
		sampleRate: sampleRate,
		scipyFilter: scipyFilter,
	}

	return filter, nil
}

// calculateCoefficients calculates the filter coefficients using the bilinear transform
func (f *ButterworthFilter) calculateCoefficients() error {
	switch f.filterType {
	case FilterLowpass:
		return f.calculateLowpassCoefficients()
	case FilterHighpass:
		return f.calculateHighpassCoefficients()
	case FilterBandpass:
		return f.calculateBandpassCoefficients()
	default:
		return fmt.Errorf("unsupported filter type")
	}
}

// calculateLowpassCoefficients calculates coefficients for a lowpass filter
func (f *ButterworthFilter) calculateLowpassCoefficients() error {
	// Normalized cutoff frequency (0 to 1, where 1 is Nyquist)
	wc := f.freqHigh / (f.sampleRate / 2)

	// Ensure frequency is in valid range
	if wc >= 1.0 {
		wc = 0.99
	}

	// Use bilinear transform with proper pre-warping
	switch f.order {
	case 1:
		// First order lowpass: H(s) = 1/(s+1)
		// Bilinear transform: s = 2(z-1)/(z+1)
		c := math.Tan(math.Pi * wc / 2)
		a1 := (1 - c) / (1 + c)
		b0 := c / (1 + c)

		f.b = []float64{b0, b0}
		f.a = []float64{1, a1}

	case 2:
		// Second order lowpass Butterworth using more stable formulation
		k := math.Tan(math.Pi * wc / 2)
		norm := 1.0 / (1.0 + k*math.Sqrt2 + k*k)

		f.b = []float64{k * k * norm, 2.0 * k * k * norm, k * k * norm}
		f.a = []float64{1, 2.0 * (k*k - 1.0) * norm, (1.0 - k*math.Sqrt2 + k*k) * norm}

	default:
		// For higher orders, limit to 2nd order for stability
		f.order = 2
		return f.calculateLowpassCoefficients()
	}

	return nil
}

// calculateHighpassCoefficients calculates coefficients for a highpass filter
func (f *ButterworthFilter) calculateHighpassCoefficients() error {
	// Normalized cutoff frequency
	wc := f.freqLow / (f.sampleRate / 2)

	// Ensure frequency is in valid range
	if wc >= 1.0 {
		wc = 0.99
	}
	if wc <= 0.0 {
		wc = 0.01
	}

	switch f.order {
	case 1:
		// First order highpass
		c := math.Tan(math.Pi * wc / 2)
		a1 := (1 - c) / (1 + c)
		b0 := 1.0 / (1 + c)

		f.b = []float64{b0, -b0}
		f.a = []float64{1, a1}

	case 2:
		// Second order highpass Butterworth using more stable formulation
		k := math.Tan(math.Pi * wc / 2)
		norm := 1.0 / (1.0 + k*math.Sqrt2 + k*k)

		f.b = []float64{norm, -2.0 * norm, norm}
		f.a = []float64{1, 2.0 * (k*k - 1.0) * norm, (1.0 - k*math.Sqrt2 + k*k) * norm}

	default:
		// For higher orders, limit to 2nd order for stability
		f.order = 2
		return f.calculateHighpassCoefficients()
	}

	return nil
}

// calculateBandpassCoefficients calculates coefficients for a bandpass filter
func (f *ButterworthFilter) calculateBandpassCoefficients() error {
	// Force to 2nd order for stability and compatibility with Python implementation
	f.order = 2
	
	// Normalized frequencies (0 to 1, where 1 is Nyquist)
	wl := f.freqLow / (f.sampleRate / 2)
	wh := f.freqHigh / (f.sampleRate / 2)

	// Ensure frequencies are in valid range
	if wl <= 0.0 {
		wl = 0.01
	}
	if wh >= 1.0 {
		wh = 0.99
	}
	if wl >= wh {
		wl = wh - 0.01
	}

	// Design bandpass filter using standard approach
	// Calculate center frequency and bandwidth
	wc := math.Sqrt(wl * wh)  // Geometric mean
	bw := wh - wl             // Bandwidth
	
	// Calculate Q factor
	Q := wc / bw
	
	// Convert to analog domain for bilinear transform
	// Prewarp frequencies
	wcPrewarp := math.Tan(math.Pi * wc / 2)
	
	// Calculate digital filter coefficients using bilinear transform
	// For 2nd order Butterworth bandpass
	K := wcPrewarp
	norm := 1.0 / (1.0 + K/Q + K*K)
	
	// Numerator coefficients (zeros)
	f.b = []float64{
		K / Q * norm,    // b0
		0.0,             // b1
		-K / Q * norm,   // b2
	}
	
	// Denominator coefficients (poles)
	f.a = []float64{
		1.0,                           // a0 (always 1)
		2.0 * (K*K - 1.0) * norm,     // a1
		(1.0 - K/Q + K*K) * norm,     // a2
	}

	return nil
}

// calculateHighOrderCoefficients handles higher order filters using cascade
func (f *ButterworthFilter) calculateHighOrderCoefficients() error {
	// For higher orders, this would implement a cascade of second-order sections
	// For now, we'll use a simplified approach

	// Default to 2nd order coefficients and warn
	switch f.filterType {
	case FilterLowpass:
		f.order = 2
		return f.calculateLowpassCoefficients()
	case FilterHighpass:
		f.order = 2
		return f.calculateHighpassCoefficients()
	case FilterBandpass:
		f.order = 2
		return f.calculateBandpassCoefficients()
	}

	return fmt.Errorf("high order filter implementation not complete")
}

// Apply applies the filter to a single sample
func (f *ButterworthFilter) Apply(input float64) float64 {
	// Check for invalid input
	if math.IsNaN(input) || math.IsInf(input, 0) {
		input = 0.0
	}

	// Limit input to prevent numerical issues
	const maxInput = 1e6
	if math.Abs(input) > maxInput {
		input = maxInput * (input / math.Abs(input))
	}

	// Shift input history
	for i := len(f.x) - 1; i > 0; i-- {
		f.x[i] = f.x[i-1]
	}
	f.x[0] = input

	// Calculate output using difference equation
	output := 0.0

	// Feed-forward part (numerator)
	for i := 0; i < len(f.b) && i < len(f.x); i++ {
		output += f.b[i] * f.x[i]
	}

	// Feedback part (denominator, excluding a[0] which is always 1)
	for i := 1; i < len(f.a) && i < len(f.y); i++ {
		output -= f.a[i] * f.y[i]
	}

	// Check for numerical instability
	if math.IsNaN(output) || math.IsInf(output, 0) {
		output = 0.0
		// Reset filter state to recover from instability
		for j := range f.x {
			f.x[j] = 0.0
		}
		for j := range f.y {
			f.y[j] = 0.0
		}
	}

	// Limit extreme values to prevent numerical issues
	const maxAmplitude = 1e6
	if math.Abs(output) > maxAmplitude {
		output = maxAmplitude * (output / math.Abs(output))
		// Also reset state to prevent continued instability
		for j := range f.x {
			f.x[j] = 0.0
		}
		for j := range f.y {
			f.y[j] = 0.0
		}
	}

	// Shift output history
	for i := len(f.y) - 1; i > 0; i-- {
		f.y[i] = f.y[i-1]
	}
	f.y[0] = output

	return output
}

// ApplySlice applies the filter to a slice of data
func (f *ButterworthFilter) ApplySlice(input []float64) []float64 {
	output := make([]float64, len(input))
	for i, sample := range input {
		output[i] = f.Apply(sample)
	}
	return output
}

// ApplyZeroPhase applies the filter in forward and backward direction for zero-phase response
// This is similar to ObsPy's filtfilt (forward-backward filtering)
func (f *ButterworthFilter) ApplyZeroPhase(input []float64) []float64 {
	if f.scipyFilter != nil {
		return f.scipyFilter.ApplyZeroPhase(input)
	}
	
	// Legacy implementation fallback (not recommended)
	if len(input) == 0 {
		return input
	}
	
	// Create a working copy
	data := make([]float64, len(input))
	copy(data, input)
	
	// Apply padding to reduce edge effects (like SciPy's filtfilt)
	padLen := 3 * (len(f.a) - 1)
	if padLen > len(data)/2 {
		padLen = len(data) / 2
	}
	
	if padLen > 0 {
		// Pad at beginning
		padBegin := make([]float64, padLen)
		for i := 0; i < padLen; i++ {
			padBegin[i] = 2*data[0] - data[padLen-i]
		}
		
		// Pad at end
		padEnd := make([]float64, padLen)
		for i := 0; i < padLen; i++ {
			padEnd[i] = 2*data[len(data)-1] - data[len(data)-1-i-1]
		}
		
		// Combine padded data
		paddedData := make([]float64, 0, len(padBegin)+len(data)+len(padEnd))
		paddedData = append(paddedData, padBegin...)
		paddedData = append(paddedData, data...)
		paddedData = append(paddedData, padEnd...)
		
		data = paddedData
	}
	
	// First pass: forward filtering
	forward := make([]float64, len(data))
	f.Reset() // Reset filter state
	for i, sample := range data {
		// Check for numerical issues
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			forward[i] = 0.0
		} else {
			forward[i] = f.Apply(sample)
		}
	}

	// Second pass: backward filtering  
	f.Reset() // Reset filter state
	for i := len(forward) - 1; i >= 0; i-- {
		sample := forward[i]
		// Check for numerical issues
		if math.IsNaN(sample) || math.IsInf(sample, 0) {
			forward[i] = 0.0
		} else {
			forward[i] = f.Apply(sample)
		}
	}

	// Reverse the output to get correct time order
	for i, j := 0, len(forward)-1; i < j; i, j = i+1, j-1 {
		forward[i], forward[j] = forward[j], forward[i]
	}
	
	// Remove padding and return
	if padLen > 0 && len(forward) > 2*padLen {
		return forward[padLen : len(forward)-padLen]
	}
	
	return forward
}

// Reset resets the filter's internal state
func (f *ButterworthFilter) Reset() {
	for i := range f.x {
		f.x[i] = 0
	}
	for i := range f.y {
		f.y[i] = 0
	}
}

// GetFrequencyResponse calculates the frequency response at given frequencies
func (f *ButterworthFilter) GetFrequencyResponse(frequencies []float64) ([]complex128, error) {
	response := make([]complex128, len(frequencies))

	for i, freq := range frequencies {
		// Convert frequency to normalized angular frequency
		omega := 2 * math.Pi * freq / f.sampleRate
		z := complex(math.Cos(omega), math.Sin(omega))

		// Calculate H(z) = B(z)/A(z)
		numerator := complex(0, 0)
		denominator := complex(0, 0)

		// Calculate numerator B(z)
		zPower := complex(1, 0)
		for j := 0; j < len(f.b); j++ {
			numerator += complex(f.b[j], 0) * zPower
			zPower *= z
		}

		// Calculate denominator A(z)
		zPower = complex(1, 0)
		for j := 0; j < len(f.a); j++ {
			denominator += complex(f.a[j], 0) * zPower
			zPower *= z
		}

		// H(z) = B(z)/A(z)
		response[i] = numerator / denominator
	}

	return response, nil
}

// GetMagnitudeResponse calculates the magnitude response
func (f *ButterworthFilter) GetMagnitudeResponse(frequencies []float64) ([]float64, error) {
	response, err := f.GetFrequencyResponse(frequencies)
	if err != nil {
		return nil, err
	}

	magnitude := make([]float64, len(response))
	for i, h := range response {
		magnitude[i] = math.Sqrt(real(h)*real(h) + imag(h)*imag(h))
	}

	return magnitude, nil
}

// GetPhaseResponse calculates the phase response
func (f *ButterworthFilter) GetPhaseResponse(frequencies []float64) ([]float64, error) {
	response, err := f.GetFrequencyResponse(frequencies)
	if err != nil {
		return nil, err
	}

	phase := make([]float64, len(response))
	for i, h := range response {
		phase[i] = math.Atan2(imag(h), real(h))
	}

	return phase, nil
}

// FilterConfig holds filter configuration
type FilterConfig struct {
	Type       FilterType `json:"type"`
	Order      int        `json:"order"`
	FreqLow    float64    `json:"freq_low"`
	FreqHigh   float64    `json:"freq_high"`
	SampleRate float64    `json:"sample_rate"`
}

// NewFilterFromConfig creates a filter from configuration
func NewFilterFromConfig(config FilterConfig) (*ButterworthFilter, error) {
	return NewButterworthFilter(config.Type, config.Order, config.FreqLow, config.FreqHigh, config.SampleRate)
}

// RemoveDCOffset removes the DC offset (mean) from the signal
func RemoveDCOffset(data []float64) []float64 {
	if len(data) == 0 {
		return data
	}

	// Calculate mean
	var sum float64
	for _, value := range data {
		sum += value
	}
	mean := sum / float64(len(data))

	// Subtract mean from each sample
	result := make([]float64, len(data))
	for i, value := range data {
		result[i] = value - mean
	}

	return result
}
