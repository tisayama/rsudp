package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tisayama/gorsudp/pkg/dsp"
)

// Config represents STA/LTA configuration
type Config struct {
	STADuration     float64 `json:"sta_duration"`
	LTADuration     float64 `json:"lta_duration"`
	Threshold       float64 `json:"threshold"`
	Reset           float64 `json:"reset"`
	SampleRate      float64 `json:"sample_rate"`
	Deconv          bool    `json:"deconv"`
	FilterEnabled   bool    `json:"filter_enabled"`
	Highpass        float64 `json:"highpass"`
	Lowpass         float64 `json:"lowpass"`
	FilterCorners   int     `json:"filter_corners"`
}

// Packet represents a UDP packet from test data
type Packet struct {
	Channel    string    `json:"channel"`
	Timestamp  string    `json:"timestamp"`
	SampleRate int       `json:"sample_rate"`
	Samples    []int32   `json:"samples"`
}

// InputData represents the test data structure
type InputData struct {
	Metadata struct {
		GeneratedAt  string `json:"generated_at"`
		Duration     int    `json:"duration_sec"`
		SampleRate   int    `json:"sample_rate"`
		TotalSamples int    `json:"total_samples"`
	} `json:"metadata"`
	Packets []Packet `json:"packets"`
}

// Result represents a single STA/LTA calculation result
type Result struct {
	SampleIndex int     `json:"sample_index"`
	Timestamp   string  `json:"timestamp"`
	STAValue    float64 `json:"sta_value"`
	LTAValue    float64 `json:"lta_value"`
	Ratio       float64 `json:"ratio"`
	Triggered   bool    `json:"triggered"`
}

// OutputData represents the output structure
type OutputData struct {
	Metadata struct {
		Timestamp      string `json:"timestamp"`
		Implementation string `json:"implementation"`
		Config         Config `json:"config"`
		TotalSamples   int    `json:"total_samples"`
		TriggerCount   int    `json:"trigger_count"`
	} `json:"metadata"`
	Results []Result `json:"results"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input_file> <output_file> [config_file]\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	// Default configuration matching Go implementation
	config := Config{
		STADuration:   6.0,
		LTADuration:   30.0,
		Threshold:     3.95,
		Reset:         0.9,
		SampleRate:    100.0,
		Deconv:        true,  // Match Python default
		FilterEnabled: false, // Match Python default
		Highpass:      0.8,
		Lowpass:       9.0,
		FilterCorners: 2,
	}

	// Load custom config if provided
	if len(os.Args) > 3 {
		configFile := os.Args[3]
		configData, err := ioutil.ReadFile(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		if err := json.Unmarshal(configData, &config); err != nil {
			log.Fatalf("Failed to parse config: %v", err)
		}
	}

	fmt.Printf("Loading test data from: %s\n", inputFile)

	// Load input data
	inputData, err := ioutil.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var input InputData
	if err := json.Unmarshal(inputData, &input); err != nil {
		log.Fatalf("Failed to parse input data: %v", err)
	}

	fmt.Printf("Loaded %d packets\n", len(input.Packets))

	// Combine all samples
	var allSamples []float64
	var timestamps []time.Time

	for _, packet := range input.Packets {
		// Parse packet timestamp
		packetTime, err := time.Parse(time.RFC3339, packet.Timestamp)
		if err != nil {
			log.Printf("Failed to parse timestamp: %v", err)
			continue
		}

		// Convert samples and apply deconvolution if needed
		for i, sample := range packet.Samples {
			var value float64
			
			// Check if deconvolution is enabled
			if config.Deconv {
				// Apply deconvolution based on channel type
				if strings.Contains(packet.Channel, "EH") || strings.Contains(packet.Channel, "SH") {
					// Geophone channels: 1.6e8 counts/(m/s)
					value = float64(sample) / 1.6e8
				} else if strings.Contains(packet.Channel, "EN") {
					// Accelerometer channels: 4.2e8 counts/(m/s²)
					value = float64(sample) / 4.2e8
				} else {
					// Unknown channel type, use raw counts
					value = float64(sample)
				}
			} else {
				// No deconvolution, use raw counts
				value = float64(sample)
			}
			
			allSamples = append(allSamples, value)
			
			// Calculate sample timestamp
			sampleTime := packetTime.Add(time.Duration(float64(i)/config.SampleRate*1e9) * time.Nanosecond)
			timestamps = append(timestamps, sampleTime)
		}
	}

	fmt.Printf("Total samples: %d\n", len(allSamples))

	// Remove DC offset
	allSamples = dsp.RemoveDCOffset(allSamples)

	// Apply filter if enabled
	if config.FilterEnabled {
		var filter *dsp.ButterworthFilter
		var err error

		if config.Highpass > 0 && config.Lowpass > 0 {
			// Bandpass filter
			filter, err = dsp.NewButterworthFilter(dsp.FilterBandpass, config.FilterCorners,
				config.Highpass, config.Lowpass, config.SampleRate)
		} else if config.Highpass > 0 {
			// Highpass filter
			filter, err = dsp.NewButterworthFilter(dsp.FilterHighpass, config.FilterCorners,
				config.Highpass, 0, config.SampleRate)
		} else if config.Lowpass > 0 {
			// Lowpass filter
			filter, err = dsp.NewButterworthFilter(dsp.FilterLowpass, config.FilterCorners,
				0, config.Lowpass, config.SampleRate)
		}

		if err != nil {
			log.Printf("Failed to create filter: %v", err)
		} else if filter != nil {
			// Apply zero-phase filtering
			allSamples = filter.ApplyZeroPhase(allSamples)
		}
	}

	// Create STA/LTA processor
	staltaConfig := dsp.STALTAConfig{
		STADuration:  config.STADuration,
		LTADuration:  config.LTADuration,
		Threshold:    config.Threshold,
		Reset:        config.Reset,
		SampleRate:   config.SampleRate,
		UseRecursive: true,  // Match Python's recursive implementation
		EnergyBased:  true,  // Match Python's default energy-based calculation
	}

	processor, err := dsp.NewSTALTAProcessor(staltaConfig)
	if err != nil {
		log.Fatalf("Failed to create STA/LTA processor: %v", err)
	}

	fmt.Println("Processing STA/LTA...")

	// Process samples
	results := make([]Result, 0, len(allSamples))
	triggerCount := 0
	wasTriggered := false

	for i, sample := range allSamples {
		// Process sample
		triggered, ratio := processor.Process(sample)

		// Get current statistics
		stats := processor.GetStats()

		// Count new triggers
		if triggered && !wasTriggered {
			triggerCount++
		}
		wasTriggered = triggered

		// Create result
		result := Result{
			SampleIndex: i,
			STAValue:    stats.STAMean,
			LTAValue:    stats.LTAMean,
			Ratio:       ratio,
			Triggered:   triggered,
		}

		// Add timestamp if available
		if i < len(timestamps) {
			result.Timestamp = timestamps[i].Format(time.RFC3339)
		}

		results = append(results, result)
	}

	fmt.Printf("Found %d trigger(s)\n", triggerCount)

	// Create output structure
	output := OutputData{}
	output.Metadata.Timestamp = time.Now().Format(time.RFC3339)
	output.Metadata.Implementation = "go"
	output.Metadata.Config = config
	output.Metadata.TotalSamples = len(allSamples)
	output.Metadata.TriggerCount = triggerCount
	output.Results = results

	// Marshal output
	outputData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal output: %v", err)
	}

	// Write output file
	if err := ioutil.WriteFile(outputFile, outputData, 0644); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Printf("Results saved to: %s\n", outputFile)

	// Print summary statistics
	var ratios []float64
	for _, r := range results {
		if r.Ratio > 0 {
			ratios = append(ratios, r.Ratio)
		}
	}

	if len(ratios) > 0 {
		minRatio, maxRatio := ratios[0], ratios[0]
		sumRatio := 0.0
		for _, r := range ratios {
			if r < minRatio {
				minRatio = r
			}
			if r > maxRatio {
				maxRatio = r
			}
			sumRatio += r
		}
		meanRatio := sumRatio / float64(len(ratios))

		fmt.Println("STA/LTA ratio statistics:")
		fmt.Printf("  Min: %.3f\n", minRatio)
		fmt.Printf("  Max: %.3f\n", maxRatio)
		fmt.Printf("  Mean: %.3f\n", meanRatio)
	}
}