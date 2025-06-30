// Package dsp provides STA/LTA (Short Term Average / Long Term Average) algorithm implementation
package dsp

import (
	"fmt"
	"math"
	"sync"
)

// STALTAProcessor implements the recursive STA/LTA earthquake detection algorithm
type STALTAProcessor struct {
	staWindow  int     // STA window length in samples
	ltaWindow  int     // LTA window length in samples
	threshold  float64 // Trigger threshold
	reset      float64 // Reset threshold
	sampleRate float64 // Sample rate in Hz

	// Recursive variables
	staMean     float64 // Current STA mean
	ltaMean     float64 // Current LTA mean
	staVariance float64 // Current STA variance (for energy calculation)
	ltaVariance float64 // Current LTA variance (for energy calculation)

	// State variables
	triggered   bool    // Whether currently triggered
	ratio       float64 // Current STA/LTA ratio
	sampleCount int64   // Total samples processed

	// Circular buffers for exact calculation (optional)
	staBuffer []float64 // STA sample buffer
	ltaBuffer []float64 // LTA sample buffer
	staIndex  int       // Current index in STA buffer
	ltaIndex  int       // Current index in LTA buffer

	// Configuration
	useRecursive bool // Use recursive calculation (faster) vs exact (more accurate)
	energyBased  bool // Use energy-based (default) vs amplitude-based calculation

	mutex sync.RWMutex // Thread safety
}

// STALTAConfig holds configuration for STA/LTA processor
type STALTAConfig struct {
	STADuration  float64 `json:"sta_duration"`  // STA window duration in seconds
	LTADuration  float64 `json:"lta_duration"`  // LTA window duration in seconds
	Threshold    float64 `json:"threshold"`     // Trigger threshold
	Reset        float64 `json:"reset"`         // Reset threshold
	SampleRate   float64 `json:"sample_rate"`   // Sample rate in Hz
	UseRecursive bool    `json:"use_recursive"` // Use recursive calculation
	EnergyBased  bool    `json:"energy_based"`  // Use energy-based calculation
}

// NewSTALTAProcessor creates a new STA/LTA processor
func NewSTALTAProcessor(config STALTAConfig) (*STALTAProcessor, error) {
	if config.STADuration <= 0 {
		return nil, fmt.Errorf("STA duration must be positive")
	}
	if config.LTADuration <= 0 {
		return nil, fmt.Errorf("LTA duration must be positive")
	}
	if config.STADuration >= config.LTADuration {
		return nil, fmt.Errorf("STA duration must be less than LTA duration")
	}
	if config.Threshold <= 0 {
		return nil, fmt.Errorf("threshold must be positive")
	}
	if config.Reset <= 0 {
		return nil, fmt.Errorf("reset threshold must be positive")
	}
	if config.SampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be positive")
	}

	staWindow := int(config.STADuration * config.SampleRate)
	ltaWindow := int(config.LTADuration * config.SampleRate)

	if staWindow < 1 {
		staWindow = 1
	}
	if ltaWindow < 1 {
		ltaWindow = 1
	}

	processor := &STALTAProcessor{
		staWindow:    staWindow,
		ltaWindow:    ltaWindow,
		threshold:    config.Threshold,
		reset:        config.Reset,
		sampleRate:   config.SampleRate,
		useRecursive: config.UseRecursive,
		energyBased:  config.EnergyBased,
		ratio:        0.0,
		staMean:      0.0,                        // ObsPy initializes STA to 0
		ltaMean:      2.2250738585072014e-308,    // ObsPy initializes LTA to np.finfo(0.0).tiny to avoid zero division
	}

	// Initialize buffers if not using recursive calculation
	if !config.UseRecursive {
		processor.staBuffer = make([]float64, staWindow)
		processor.ltaBuffer = make([]float64, ltaWindow)
	}

	return processor, nil
}

// Process processes a single sample and returns trigger status and current ratio
func (p *STALTAProcessor) Process(sample float64) (bool, float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.sampleCount++

	if p.useRecursive {
		return p.processRecursive(sample)
	}
	return p.processExact(sample)
}

// ProcessSlice processes a slice of samples and returns trigger points and ratios
func (p *STALTAProcessor) ProcessSlice(samples []float64) ([]bool, []float64) {
	triggers := make([]bool, len(samples))
	ratios := make([]float64, len(samples))

	for i, sample := range samples {
		trigger, ratio := p.Process(sample)
		triggers[i] = trigger
		ratios[i] = ratio
	}

	return triggers, ratios
}

// processRecursive implements the recursive STA/LTA algorithm matching ObsPy implementation
func (p *STALTAProcessor) processRecursive(sample float64) (bool, float64) {
	// Calculate the value to use (energy: amplitude^2, matching ObsPy)
	value := sample * sample // ObsPy always uses squared values (energy)

	// Calculate coefficients (matching ObsPy formula)
	csta := 1.0 / float64(p.staWindow)  // 1/nsta
	clta := 1.0 / float64(p.ltaWindow)  // 1/nlta
	icsta := 1.0 - csta                 // 1 - csta
	iclta := 1.0 - clta                 // 1 - clta

	// ObsPy recursive formula: sta = csta * a[i] + icsta * sta
	p.staMean = csta*value + icsta*p.staMean

	// ObsPy recursive formula: lta = clta * a[i] + iclta * lta  
	p.ltaMean = clta*value + iclta*p.ltaMean

	// Calculate STA/LTA ratio (avoid division by zero)
	if p.ltaMean > 0 {
		p.ratio = p.staMean / p.ltaMean
	} else {
		p.ratio = 0
	}

	// Note: Removed forced ratio=0 during warmup period
	// Python/ObsPy implementation actually returns calculated ratios from the first sample

	// Check trigger conditions
	return p.checkTrigger(), p.ratio
}

// processExact implements exact STA/LTA calculation using circular buffers
func (p *STALTAProcessor) processExact(sample float64) (bool, float64) {
	// Calculate the value to use (energy: amplitude^2, matching ObsPy)
	value := sample * sample // ObsPy always uses squared values (energy)

	// Update STA buffer and calculate exact mean
	oldSTAValue := p.staBuffer[p.staIndex]
	p.staBuffer[p.staIndex] = value
	p.staIndex = (p.staIndex + 1) % p.staWindow

	// Calculate exact STA mean
	if p.sampleCount >= int64(p.staWindow) {
		// Use running sum update: remove old value, add new value
		p.staMean = p.staMean + (value-oldSTAValue)/float64(p.staWindow)
	} else {
		// During warmup, calculate exact mean
		staSum := 0.0
		staCount := int(math.Min(float64(p.sampleCount), float64(p.staWindow)))
		for i := 0; i < staCount; i++ {
			staSum += p.staBuffer[i]
		}
		p.staMean = staSum / float64(staCount)
	}

	// Update LTA buffer and calculate exact mean
	oldLTAValue := p.ltaBuffer[p.ltaIndex]
	p.ltaBuffer[p.ltaIndex] = value
	p.ltaIndex = (p.ltaIndex + 1) % p.ltaWindow

	// Calculate exact LTA mean
	if p.sampleCount >= int64(p.ltaWindow) {
		// Use running sum update: remove old value, add new value
		p.ltaMean = p.ltaMean + (value-oldLTAValue)/float64(p.ltaWindow)
	} else {
		// During warmup, calculate exact mean
		ltaSum := 0.0
		ltaCount := int(math.Min(float64(p.sampleCount), float64(p.ltaWindow)))
		for i := 0; i < ltaCount; i++ {
			ltaSum += p.ltaBuffer[i]
		}
		p.ltaMean = ltaSum / float64(ltaCount)
	}

	// Calculate STA/LTA ratio
	if p.ltaMean > 0 {
		p.ratio = p.staMean / p.ltaMean
	} else {
		p.ratio = 0
	}

	// Note: Removed forced ratio=0 during warmup period
	// Python/ObsPy implementation actually returns calculated ratios from the first sample

	// Check trigger conditions
	return p.checkTrigger(), p.ratio
}

// checkTrigger checks trigger conditions and updates trigger state
func (p *STALTAProcessor) checkTrigger() bool {
	// Only start checking for triggers after LTA warmup period
	if p.sampleCount < int64(p.ltaWindow) {
		return false
	}

	// Check for trigger onset
	if !p.triggered && p.ratio > p.threshold {
		p.triggered = true
		return true
	}

	// Check for trigger reset
	if p.triggered && p.ratio < p.reset {
		p.triggered = false
	}

	return p.triggered
}

// IsTriggered returns the current trigger state
func (p *STALTAProcessor) IsTriggered() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.triggered
}

// GetRatio returns the current STA/LTA ratio
func (p *STALTAProcessor) GetRatio() float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.ratio
}

// GetSTA returns the current STA value
func (p *STALTAProcessor) GetSTA() float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.staMean
}

// GetLTA returns the current LTA value
func (p *STALTAProcessor) GetLTA() float64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.ltaMean
}

// Reset resets the processor state (matching ObsPy initialization)
func (p *STALTAProcessor) Reset() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.staMean = 0.0                        // ObsPy initializes STA to 0
	p.ltaMean = 2.2250738585072014e-308    // ObsPy initializes LTA to np.finfo(0.0).tiny
	p.staVariance = 0
	p.ltaVariance = 0
	p.triggered = false
	p.ratio = 0
	p.sampleCount = 0
	p.staIndex = 0
	p.ltaIndex = 0

	// Clear buffers
	if p.staBuffer != nil {
		for i := range p.staBuffer {
			p.staBuffer[i] = 0
		}
	}
	if p.ltaBuffer != nil {
		for i := range p.ltaBuffer {
			p.ltaBuffer[i] = 0
		}
	}
}

// GetStats returns statistics about the processor
func (p *STALTAProcessor) GetStats() STALTAStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return STALTAStats{
		SampleCount: p.sampleCount,
		STAMean:     p.staMean,
		LTAMean:     p.ltaMean,
		Ratio:       p.ratio,
		Triggered:   p.triggered,
		STAWindow:   p.staWindow,
		LTAWindow:   p.ltaWindow,
		Threshold:   p.threshold,
		Reset:       p.reset,
	}
}

// STALTAStats contains statistics about the STA/LTA processor
type STALTAStats struct {
	SampleCount int64   `json:"sample_count"`
	STAMean     float64 `json:"sta_mean"`
	LTAMean     float64 `json:"lta_mean"`
	Ratio       float64 `json:"ratio"`
	Triggered   bool    `json:"triggered"`
	STAWindow   int     `json:"sta_window"`
	LTAWindow   int     `json:"lta_window"`
	Threshold   float64 `json:"threshold"`
	Reset       float64 `json:"reset"`
}

// TriggerOnset finds trigger onset times in STA/LTA ratio data
func TriggerOnset(ratios []float64, threshold, resetThreshold float64) []int {
	var onsets []int
	triggered := false

	for i, ratio := range ratios {
		if !triggered && ratio > threshold {
			onsets = append(onsets, i)
			triggered = true
		} else if triggered && ratio < resetThreshold {
			triggered = false
		}
	}

	return onsets
}

// CalculateClassicSTALTA calculates STA/LTA using the classic algorithm (non-recursive)
// This is useful for validation and comparison with the recursive version
func CalculateClassicSTALTA(data []float64, staWindow, ltaWindow int) ([]float64, error) {
	if len(data) < ltaWindow {
		return nil, fmt.Errorf("data length must be at least LTA window size")
	}

	if staWindow >= ltaWindow {
		return nil, fmt.Errorf("STA window must be smaller than LTA window")
	}

	ratios := make([]float64, len(data))

	for i := ltaWindow; i < len(data); i++ {
		// Calculate STA (energy in short window)
		staSum := 0.0
		for j := i - staWindow; j < i; j++ {
			staSum += data[j] * data[j]
		}
		staMean := staSum / float64(staWindow)

		// Calculate LTA (energy in long window)
		ltaSum := 0.0
		for j := i - ltaWindow; j < i; j++ {
			ltaSum += data[j] * data[j]
		}
		ltaMean := ltaSum / float64(ltaWindow)

		// Calculate ratio
		if ltaMean > 0 {
			ratios[i] = staMean / ltaMean
		} else {
			ratios[i] = 0
		}
	}

	return ratios, nil
}
