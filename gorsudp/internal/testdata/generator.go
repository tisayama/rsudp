// Package testdata provides test data generation and loading functionality
package testdata

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// TestDataGenerator generates synthetic UDP packets for testing
type TestDataGenerator struct {
	channels    []string
	sampleRate  float64
	amplitude   float64
	noiseLevel  float64
	rand        *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(channels []string, sampleRate float64) *TestDataGenerator {
	return &TestDataGenerator{
		channels:   channels,
		sampleRate: sampleRate,
		amplitude:  10000.0,
		noiseLevel: 100.0,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GeneratePacket generates a single UDP packet
func (g *TestDataGenerator) GeneratePacket(channel string, timestamp time.Time, duration time.Duration) *shakenet.UDPPacket {
	samplesPerPacket := int(g.sampleRate * duration.Seconds())
	data := make([]int32, samplesPerPacket)
	
	// Generate synthetic seismic data
	for i := 0; i < samplesPerPacket; i++ {
		t := float64(i) / g.sampleRate
		
		// Base signal: low frequency oscillation
		signal := g.amplitude * math.Sin(2*math.Pi*0.1*t)
		
		// Add higher frequency components
		signal += g.amplitude * 0.3 * math.Sin(2*math.Pi*1.0*t)
		signal += g.amplitude * 0.1 * math.Sin(2*math.Pi*5.0*t)
		
		// Add random noise
		noise := g.noiseLevel * (g.rand.Float64() - 0.5)
		
		data[i] = int32(signal + noise)
	}
	
	return &shakenet.UDPPacket{
		Channel:   channel,
		Timestamp: timestamp,
		Data:      data,
	}
}

// GenerateSeismicEvent generates packets representing a seismic event
func (g *TestDataGenerator) GenerateSeismicEvent(channel string, startTime time.Time, eventDuration time.Duration, packetDuration time.Duration) []*shakenet.UDPPacket {
	var packets []*shakenet.UDPPacket
	
	currentTime := startTime
	endTime := startTime.Add(eventDuration)
	
	for currentTime.Before(endTime) {
		packet := g.generateEventPacket(channel, currentTime, packetDuration, startTime, eventDuration)
		packets = append(packets, packet)
		currentTime = currentTime.Add(packetDuration)
	}
	
	return packets
}

// generateEventPacket generates a packet with seismic event characteristics
func (g *TestDataGenerator) generateEventPacket(channel string, timestamp time.Time, duration time.Duration, eventStart time.Time, eventDuration time.Duration) *shakenet.UDPPacket {
	samplesPerPacket := int(g.sampleRate * duration.Seconds())
	data := make([]int32, samplesPerPacket)
	
	// Calculate position within the event
	eventProgress := timestamp.Sub(eventStart).Seconds() / eventDuration.Seconds()
	
	// Create envelope (attack, sustain, decay)
	var envelope float64
	if eventProgress < 0.1 {
		// Attack phase
		envelope = eventProgress / 0.1
	} else if eventProgress < 0.7 {
		// Sustain phase
		envelope = 1.0
	} else {
		// Decay phase
		envelope = (1.0 - eventProgress) / 0.3
	}
	
	if envelope < 0 {
		envelope = 0
	}
	
	for i := 0; i < samplesPerPacket; i++ {
		t := float64(i) / g.sampleRate
		globalTime := timestamp.Sub(eventStart).Seconds() + t
		
		// P-wave component (high frequency)
		pWave := g.amplitude * envelope * 2.0 * math.Sin(2*math.Pi*10.0*globalTime) * math.Exp(-globalTime*0.5)
		
		// S-wave component (lower frequency, higher amplitude)
		sWave := g.amplitude * envelope * 3.0 * math.Sin(2*math.Pi*3.0*globalTime) * math.Exp(-globalTime*0.2)
		
		// Surface waves (lowest frequency, sustained)
		surfaceWave := g.amplitude * envelope * 1.5 * math.Sin(2*math.Pi*0.5*globalTime) * math.Exp(-globalTime*0.1)
		
		// Background noise
		noise := g.noiseLevel * (g.rand.Float64() - 0.5)
		
		signal := pWave + sWave + surfaceWave + noise
		data[i] = int32(signal)
	}
	
	return &shakenet.UDPPacket{
		Channel:   channel,
		Timestamp: timestamp,
		Data:      data,
	}
}

// GenerateWhiteNoise generates packets with white noise
func (g *TestDataGenerator) GenerateWhiteNoise(channel string, timestamp time.Time, duration time.Duration, amplitude float64) *shakenet.UDPPacket {
	samplesPerPacket := int(g.sampleRate * duration.Seconds())
	data := make([]int32, samplesPerPacket)
	
	for i := 0; i < samplesPerPacket; i++ {
		noise := amplitude * (g.rand.Float64() - 0.5)
		data[i] = int32(noise)
	}
	
	return &shakenet.UDPPacket{
		Channel:   channel,
		Timestamp: timestamp,
		Data:      data,
	}
}

// SavePacketsToFile saves packets to a file in the Python test data format
func SavePacketsToFile(packets []*shakenet.UDPPacket, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()
	
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	
	for _, packet := range packets {
		line := packet.String()
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write packet: %v", err)
		}
	}
	
	// Add termination marker
	if _, err := writer.WriteString("TERM\n"); err != nil {
		return fmt.Errorf("failed to write termination marker: %v", err)
	}
	
	return nil
}

// LoadPacketsFromFile loads packets from a file
func LoadPacketsFromFile(filename string) ([]*shakenet.UDPPacket, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	
	var packets []*shakenet.UDPPacket
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for termination marker
		if line == "TERM" {
			break
		}
		
		// Skip empty lines
		if len(line) == 0 {
			continue
		}
		
		packet, err := shakenet.ParseUDPPacket([]byte(line))
		if err != nil {
			// Log error but continue processing
			fmt.Printf("Warning: failed to parse packet: %v\n", err)
			continue
		}
		
		packets = append(packets, packet)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	
	return packets, nil
}

// GenerateTestScenario generates a complete test scenario
func GenerateTestScenario(config TestScenarioConfig) ([]*shakenet.UDPPacket, error) {
	generator := NewTestDataGenerator(config.Channels, config.SampleRate)
	var allPackets []*shakenet.UDPPacket
	
	currentTime := config.StartTime
	packetDuration := time.Duration(float64(time.Second) * float64(config.SamplesPerPacket) / config.SampleRate)
	
	for currentTime.Before(config.StartTime.Add(config.Duration)) {
		for _, channel := range config.Channels {
			var packet *shakenet.UDPPacket
			
			// Check if this time falls within any event
			inEvent := false
			for _, event := range config.Events {
				if currentTime.After(event.StartTime) && currentTime.Before(event.StartTime.Add(event.Duration)) {
					// Generate event packet
					packet = generator.generateEventPacket(channel, currentTime, packetDuration, event.StartTime, event.Duration)
					inEvent = true
					break
				}
			}
			
			if !inEvent {
				// Generate normal background noise
				packet = generator.GenerateWhiteNoise(channel, currentTime, packetDuration, config.BackgroundNoise)
			}
			
			allPackets = append(allPackets, packet)
		}
		
		currentTime = currentTime.Add(packetDuration)
	}
	
	return allPackets, nil
}

// TestScenarioConfig defines a test scenario
type TestScenarioConfig struct {
	StartTime        time.Time
	Duration         time.Duration
	Channels         []string
	SampleRate       float64
	SamplesPerPacket int
	BackgroundNoise  float64
	Events           []EventConfig
}

// EventConfig defines a seismic event
type EventConfig struct {
	StartTime time.Time
	Duration  time.Duration
	Magnitude float64
}

// CreateStandardTestData creates standard test datasets
func CreateStandardTestData(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	
	// Test scenario 1: Normal background noise
	config1 := TestScenarioConfig{
		StartTime:        time.Now(),
		Duration:         5 * time.Minute,
		Channels:         []string{"EHZ", "EHN", "EHE"},
		SampleRate:       100.0,
		SamplesPerPacket: 25,
		BackgroundNoise:  100.0,
		Events:           []EventConfig{},
	}
	
	packets1, err := GenerateTestScenario(config1)
	if err != nil {
		return fmt.Errorf("failed to generate test scenario 1: %v", err)
	}
	
	if err := SavePacketsToFile(packets1, filepath.Join(outputDir, "background_noise.dat")); err != nil {
		return fmt.Errorf("failed to save test scenario 1: %v", err)
	}
	
	// Test scenario 2: Single earthquake event
	config2 := TestScenarioConfig{
		StartTime:        time.Now(),
		Duration:         10 * time.Minute,
		Channels:         []string{"EHZ", "EHN", "EHE"},
		SampleRate:       100.0,
		SamplesPerPacket: 25,
		BackgroundNoise:  100.0,
		Events: []EventConfig{
			{
				StartTime: time.Now().Add(2 * time.Minute),
				Duration:  3 * time.Minute,
				Magnitude: 4.5,
			},
		},
	}
	
	packets2, err := GenerateTestScenario(config2)
	if err != nil {
		return fmt.Errorf("failed to generate test scenario 2: %v", err)
	}
	
	if err := SavePacketsToFile(packets2, filepath.Join(outputDir, "earthquake_event.dat")); err != nil {
		return fmt.Errorf("failed to save test scenario 2: %v", err)
	}
	
	fmt.Printf("Created test data in %s\n", outputDir)
	return nil
}