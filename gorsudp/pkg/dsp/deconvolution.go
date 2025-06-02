// Package dsp provides deconvolution functionality for instrument response removal
package dsp

import (
	"fmt"
)

// DeconvolutionType represents different types of unit conversion
type DeconvolutionType int

const (
	DeconvVelocity     DeconvolutionType = iota // VEL - velocity (m/s)
	DeconvAcceleration                          // ACC - acceleration (m/s²)
	DeconvDisplacement                          // DISP - displacement (m)
	DeconvGravity                               // GRAV - fraction of gravity (g)
	DeconvChannel                               // CHAN - channel-specific
)

func (d DeconvolutionType) String() string {
	switch d {
	case DeconvVelocity:
		return "VEL"
	case DeconvAcceleration:
		return "ACC"
	case DeconvDisplacement:
		return "DISP"
	case DeconvGravity:
		return "GRAV"
	case DeconvChannel:
		return "CHAN"
	default:
		return "UNKNOWN"
	}
}

// ParseDeconvolutionType parses a string to DeconvolutionType
func ParseDeconvolutionType(s string) (DeconvolutionType, error) {
	switch s {
	case "VEL":
		return DeconvVelocity, nil
	case "ACC":
		return DeconvAcceleration, nil
	case "DISP":
		return DeconvDisplacement, nil
	case "GRAV":
		return DeconvGravity, nil
	case "CHAN":
		return DeconvChannel, nil
	default:
		return DeconvVelocity, fmt.Errorf("unknown deconvolution type: %s", s)
	}
}

// Units contains unit information for different deconvolution types
type Units struct {
	Name   string
	Symbol string
}

// GetUnits returns the units for a given deconvolution type
func GetUnits(deconvType DeconvolutionType) Units {
	switch deconvType {
	case DeconvVelocity:
		return Units{Name: "Velocity", Symbol: "m/s"}
	case DeconvAcceleration:
		return Units{Name: "Acceleration", Symbol: "m/s²"}
	case DeconvDisplacement:
		return Units{Name: "Displacement", Symbol: "m"}
	case DeconvGravity:
		return Units{Name: "Earth gravity", Symbol: "g"}
	case DeconvChannel:
		return Units{Name: "channel-specific", Symbol: "Counts"}
	default:
		return Units{Name: "Unknown", Symbol: ""}
	}
}

// InstrumentResponse represents a simplified instrument response
type InstrumentResponse struct {
	Sensitivity float64      `json:"sensitivity"`  // Overall sensitivity (counts/unit)
	SampleRate  float64      `json:"sample_rate"`  // Sample rate (Hz)
	Poles       []complex128 `json:"poles"`        // Complex poles
	Zeros       []complex128 `json:"zeros"`        // Complex zeros
	Gain        float64      `json:"gain"`         // Normalization gain
	InputUnits  string       `json:"input_units"`  // Input units (e.g., "M/S")
	OutputUnits string       `json:"output_units"` // Output units (e.g., "COUNTS")
}

// Deconvolver performs instrument response removal
type Deconvolver struct {
	response     InstrumentResponse
	deconvType   DeconvolutionType
	sampleRate   float64
	earthGravity float64 // Earth gravity constant (9.81 m/s²)
}

// NewDeconvolver creates a new deconvolver
func NewDeconvolver(response InstrumentResponse, deconvType DeconvolutionType, sampleRate float64) (*Deconvolver, error) {
	if response.Sensitivity == 0 {
		return nil, fmt.Errorf("instrument sensitivity cannot be zero")
	}

	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}

	return &Deconvolver{
		response:     response,
		deconvType:   deconvType,
		sampleRate:   sampleRate,
		earthGravity: 9.81,
	}, nil
}

// Deconvolve removes instrument response from data
func (d *Deconvolver) Deconvolve(data []float64) ([]float64, error) {
	// Convert counts to physical units using sensitivity
	physicalData := make([]float64, len(data))
	for i, sample := range data {
		// Remove sensitivity to get base physical units
		physicalData[i] = sample / d.response.Sensitivity
	}

	// Apply unit conversion based on deconvolution type
	return d.convertUnits(physicalData)
}

// convertUnits converts data to the desired physical units
func (d *Deconvolver) convertUnits(data []float64) ([]float64, error) {
	result := make([]float64, len(data))

	switch d.deconvType {
	case DeconvVelocity:
		// Already in velocity units for geophone data
		copy(result, data)

	case DeconvAcceleration:
		// Convert velocity to acceleration (differentiation)
		result = d.differentiate(data)

	case DeconvDisplacement:
		// Convert velocity to displacement (integration)
		result = d.integrate(data)

	case DeconvGravity:
		// Convert to fraction of earth gravity
		// Assume input is acceleration in m/s²
		accelData := d.differentiate(data)
		for i, sample := range accelData {
			result[i] = sample / d.earthGravity
		}

	case DeconvChannel:
		// Keep original units (no conversion)
		copy(result, data)

	default:
		return nil, fmt.Errorf("unsupported deconvolution type: %v", d.deconvType)
	}

	return result, nil
}

// differentiate approximates differentiation using finite differences
func (d *Deconvolver) differentiate(data []float64) []float64 {
	if len(data) < 2 {
		return data
	}

	result := make([]float64, len(data))
	dt := 1.0 / d.sampleRate

	// Forward difference for first point
	result[0] = (data[1] - data[0]) / dt

	// Central difference for middle points
	for i := 1; i < len(data)-1; i++ {
		result[i] = (data[i+1] - data[i-1]) / (2 * dt)
	}

	// Backward difference for last point
	result[len(data)-1] = (data[len(data)-1] - data[len(data)-2]) / dt

	return result
}

// integrate approximates integration using trapezoidal rule
func (d *Deconvolver) integrate(data []float64) []float64 {
	if len(data) == 0 {
		return data
	}

	result := make([]float64, len(data))
	dt := 1.0 / d.sampleRate

	result[0] = 0 // Initial condition

	// Trapezoidal integration
	for i := 1; i < len(data); i++ {
		result[i] = result[i-1] + (data[i-1]+data[i])*dt/2
	}

	// Remove DC component (high-pass filter effect)
	mean := d.calculateMean(result)
	for i := range result {
		result[i] -= mean
	}

	return result
}

// calculateMean calculates the mean of a data slice
func (d *Deconvolver) calculateMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// SimpleDeconvolver provides simplified deconvolution without full response
type SimpleDeconvolver struct {
	sensitivity float64
	deconvType  DeconvolutionType
	sampleRate  float64
	channelType string // Channel type for channel-specific conversion
}

// NewSimpleDeconvolver creates a simplified deconvolver
func NewSimpleDeconvolver(sensitivity float64, deconvType DeconvolutionType, sampleRate float64, channelType string) (*SimpleDeconvolver, error) {
	if sensitivity == 0 {
		return nil, fmt.Errorf("sensitivity cannot be zero")
	}

	if sampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}

	return &SimpleDeconvolver{
		sensitivity: sensitivity,
		deconvType:  deconvType,
		sampleRate:  sampleRate,
		channelType: channelType,
	}, nil
}

// Deconvolve performs simplified deconvolution
func (s *SimpleDeconvolver) Deconvolve(data []float64) ([]float64, error) {
	result := make([]float64, len(data))

	switch s.deconvType {
	case DeconvVelocity:
		// Convert counts to velocity (typical for geophone channels)
		for i, sample := range data {
			result[i] = sample / s.sensitivity
		}

	case DeconvAcceleration:
		// Convert counts to acceleration (typical for accelerometer channels)
		for i, sample := range data {
			result[i] = sample / s.sensitivity
		}

	case DeconvGravity:
		// Convert counts to fraction of gravity
		for i, sample := range data {
			accel := sample / s.sensitivity
			result[i] = accel / 9.81
		}

	case DeconvDisplacement:
		// Convert to displacement (requires integration from velocity)
		velocity := make([]float64, len(data))
		for i, sample := range data {
			velocity[i] = sample / s.sensitivity
		}
		result = s.integrate(velocity)

	case DeconvChannel:
		// Channel-specific conversion
		result = s.channelSpecificConversion(data)

	default:
		return nil, fmt.Errorf("unsupported deconvolution type: %v", s.deconvType)
	}

	return result, nil
}

// channelSpecificConversion applies channel-specific unit conversion
func (s *SimpleDeconvolver) channelSpecificConversion(data []float64) []float64 {
	result := make([]float64, len(data))

	// Determine appropriate conversion based on channel type
	switch {
	case containsString(s.channelType, []string{"EHZ", "EHN", "EHE", "SHZ", "SHN", "SHE"}):
		// Geophone channels - convert to velocity
		for i, sample := range data {
			result[i] = sample / s.sensitivity
		}

	case containsString(s.channelType, []string{"ENZ", "ENN", "ENE"}):
		// Accelerometer channels - convert to acceleration
		for i, sample := range data {
			result[i] = sample / s.sensitivity
		}

	case containsString(s.channelType, []string{"HDF"}):
		// Pressure channels - keep as pressure units
		for i, sample := range data {
			result[i] = sample / s.sensitivity
		}

	default:
		// Unknown channel type - keep as counts
		copy(result, data)
	}

	return result
}

// integrate performs integration for displacement calculation
func (s *SimpleDeconvolver) integrate(data []float64) []float64 {
	if len(data) == 0 {
		return data
	}

	result := make([]float64, len(data))
	dt := 1.0 / s.sampleRate

	result[0] = 0
	for i := 1; i < len(data); i++ {
		result[i] = result[i-1] + (data[i-1]+data[i])*dt/2
	}

	// Remove DC component
	mean := 0.0
	for _, v := range result {
		mean += v
	}
	mean /= float64(len(result))

	for i := range result {
		result[i] -= mean
	}

	return result
}

// containsString checks if a string is in a slice of strings
func containsString(s string, slice []string) bool {
	for _, item := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// DefaultRaspberryShakeResponse creates a default response for Raspberry Shake devices
func DefaultRaspberryShakeResponse(channelType string, sampleRate float64) InstrumentResponse {
	// Default sensitivities for different channel types (counts per unit)
	sensitivity := 1.0
	inputUnits := "COUNTS"
	outputUnits := "COUNTS"

	switch {
	case containsString(channelType, []string{"EHZ", "EHN", "EHE", "SHZ", "SHN", "SHE"}):
		// Geophone channels (velocity)
		sensitivity = 1.0e6 // Typical sensitivity for geophones
		inputUnits = "M/S"
		outputUnits = "COUNTS"

	case containsString(channelType, []string{"ENZ", "ENN", "ENE"}):
		// Accelerometer channels
		sensitivity = 1.0e8 // Typical sensitivity for accelerometers
		inputUnits = "M/S**2"
		outputUnits = "COUNTS"

	case containsString(channelType, []string{"HDF"}):
		// Pressure channels
		sensitivity = 1.0e5 // Typical sensitivity for pressure sensors
		inputUnits = "PA"
		outputUnits = "COUNTS"
	}

	return InstrumentResponse{
		Sensitivity: sensitivity,
		SampleRate:  sampleRate,
		Poles:       []complex128{}, // Simplified - no poles/zeros
		Zeros:       []complex128{},
		Gain:        1.0,
		InputUnits:  inputUnits,
		OutputUnits: outputUnits,
	}
}
