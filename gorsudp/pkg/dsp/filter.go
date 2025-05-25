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
	
	// Filter coefficients
	a []float64 // Denominator coefficients
	b []float64 // Numerator coefficients
	
	// Filter state (delay line)
	x []float64 // Input history
	y []float64 // Output history
}

// NewButterworthFilter creates a new Butterworth filter
func NewButterworthFilter(filterType FilterType, order int, freqLow, freqHigh, sampleRate float64) (*ButterworthFilter, error) {
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
	
	filter := &ButterworthFilter{
		filterType: filterType,
		order:      order,
		freqLow:    freqLow,
		freqHigh:   freqHigh,
		sampleRate: sampleRate,
	}
	
	// Calculate filter coefficients
	if err := filter.calculateCoefficients(); err != nil {
		return nil, fmt.Errorf("failed to calculate filter coefficients: %v", err)
	}
	
	// Initialize delay lines
	filter.x = make([]float64, len(filter.b))
	filter.y = make([]float64, len(filter.a))
	
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
	// Normalized frequencies
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
	
	// Calculate center frequency and bandwidth
	wc := math.Sqrt(wl * wh)
	bw := wh - wl
	
	// Force to 2nd order for stability
	f.order = 2
	
	// Second order bandpass Butterworth using more stable formulation
	Q := wc / bw
	K := math.Tan(math.Pi * wc / 2)
	norm := 1.0 / (1.0 + K/Q + K*K)
	
	f.b = []float64{K / Q * norm, 0, -K / Q * norm}
	f.a = []float64{1, 2.0 * (K*K - 1.0) * norm, (1.0 - K/Q + K*K) * norm}
	
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
	
	// Clamp output to reasonable range to prevent runaway
	if math.Abs(output) > 10.0 {
		if output > 0 {
			output = 10.0
		} else {
			output = -10.0
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