package testdata

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// LiveTestDataGenerator generates real-time synthetic seismic data
type LiveTestDataGenerator struct {
	config       LiveGeneratorConfig
	conn         *net.UDPConn
	channels     []LiveChannelGenerator
	stopChan     chan struct{}
	isRunning    bool
	packetsSent  int64
	startTime    time.Time
}

// LiveGeneratorConfig contains configuration for live test data generation
type LiveGeneratorConfig struct {
	TargetHost       string        `json:"target_host"`
	TargetPort       int           `json:"target_port"`
	SampleRate       float64       `json:"sample_rate"`     // Samples per second
	PacketInterval   time.Duration `json:"packet_interval"` // Time between packets
	SamplesPerPacket int           `json:"samples_per_packet"`
	Channels         []string      `json:"channels"`
	NoiseLevel       float64       `json:"noise_level"`     // Background noise amplitude
	EventProbability float64       `json:"event_probability"` // Probability of earthquake event per minute
	TriggerEvents    bool          `json:"trigger_events"`  // Whether to automatically trigger events
}

// LiveChannelGenerator generates data for a specific channel in real-time
type LiveChannelGenerator struct {
	Name           string
	BaseAmplitude  float64
	NoiseLevel     float64
	CurrentEvent   *LiveSeismicEvent
	LastSample     float64
	SampleCount    int64
	Phase          float64 // Phase for continuous wave generation
}

// LiveSeismicEvent represents a simulated earthquake event
type LiveSeismicEvent struct {
	StartTime     time.Time
	Duration      time.Duration
	Magnitude     float64
	Frequency     float64
	PeakAmplitude float64
	Elapsed       time.Duration
	Type          string // "local", "regional", "teleseismic"
}

// LiveGeneratorStats contains statistics about live data generation
type LiveGeneratorStats struct {
	PacketsSent      int64         `json:"packets_sent"`
	Duration         time.Duration `json:"duration"`
	SamplesPerSecond float64       `json:"samples_per_second"`
	IsRunning        bool          `json:"is_running"`
	ActiveEvents     int           `json:"active_events"`
	TotalEvents      int           `json:"total_events"`
}

// NewLiveTestDataGenerator creates a new live test data generator
func NewLiveTestDataGenerator(config LiveGeneratorConfig) *LiveTestDataGenerator {
	// Set defaults
	if config.TargetHost == "" {
		config.TargetHost = "localhost"
	}
	if config.TargetPort == 0 {
		config.TargetPort = 8888
	}
	if config.SampleRate == 0 {
		config.SampleRate = 100.0
	}
	if config.PacketInterval == 0 {
		config.PacketInterval = 1 * time.Second
	}
	if config.SamplesPerPacket == 0 {
		config.SamplesPerPacket = 100
	}
	if len(config.Channels) == 0 {
		config.Channels = []string{"EHZ", "EHN", "EHE"}
	}
	if config.NoiseLevel == 0 {
		config.NoiseLevel = 100.0
	}
	if config.EventProbability == 0 {
		config.EventProbability = 0.05 // 5% chance per minute
	}

	generator := &LiveTestDataGenerator{
		config:    config,
		channels:  make([]LiveChannelGenerator, len(config.Channels)),
		stopChan:  make(chan struct{}),
		startTime: time.Now(),
	}

	// Initialize channel generators
	for i, channelName := range config.Channels {
		generator.channels[i] = LiveChannelGenerator{
			Name:          channelName,
			BaseAmplitude: 1000.0,
			NoiseLevel:    config.NoiseLevel,
			Phase:         rand.Float64() * 2 * math.Pi, // Random starting phase
		}
	}

	return generator
}

// Start starts the live test data generator
func (ltdg *LiveTestDataGenerator) Start() error {
	if ltdg.isRunning {
		return fmt.Errorf("generator is already running")
	}

	// Create UDP connection
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ltdg.config.TargetHost, ltdg.config.TargetPort))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	ltdg.conn, err = net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %v", err)
	}

	ltdg.isRunning = true
	ltdg.startTime = time.Now()

	// Start generation loop
	go ltdg.generateLoop()

	log.Printf("Live test data generator started: target=%s:%d, rate=%.1f Hz, channels=%v",
		ltdg.config.TargetHost, ltdg.config.TargetPort, ltdg.config.SampleRate, ltdg.config.Channels)

	return nil
}

// Stop stops the live test data generator
func (ltdg *LiveTestDataGenerator) Stop() error {
	if !ltdg.isRunning {
		return nil
	}

	log.Println("Stopping live test data generator...")
	close(ltdg.stopChan)
	ltdg.isRunning = false

	if ltdg.conn != nil {
		ltdg.conn.Close()
	}

	log.Printf("Live test data generator stopped. Sent %d packets in %v",
		ltdg.packetsSent, time.Since(ltdg.startTime))

	return nil
}

// TriggerEvent manually triggers a seismic event
func (ltdg *LiveTestDataGenerator) TriggerEvent(magnitude float64, duration time.Duration) {
	log.Printf("Manually triggering seismic event: magnitude=%.1f, duration=%.1fs", magnitude, duration)
	
	for i := range ltdg.channels {
		channel := &ltdg.channels[i]
		
		// Don't override existing events
		if channel.CurrentEvent == nil {
			event := ltdg.createEvent(time.Now(), magnitude, duration, "manual")
			channel.CurrentEvent = event
		}
	}
}

// generateLoop is the main generation loop
func (ltdg *LiveTestDataGenerator) generateLoop() {
	ticker := time.NewTicker(ltdg.config.PacketInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ltdg.generatePackets(); err != nil {
				log.Printf("Error generating packets: %v", err)
			}
		case <-ltdg.stopChan:
			return
		}
	}
}

// generatePackets generates and sends packets for all channels
func (ltdg *LiveTestDataGenerator) generatePackets() error {
	timestamp := float64(time.Now().Unix()) + float64(time.Now().Nanosecond())/1e9

	for i := range ltdg.channels {
		channel := &ltdg.channels[i]

		// Generate samples for this channel
		samples := ltdg.generateSamples(channel, ltdg.config.SamplesPerPacket)

		// Create and send packet
		packet := &shakenet.UDPPacket{
			Channel: channel.Name,
			Data:    samples,
		}
		packet.SetTimestamp(timestamp)

		if err := ltdg.sendPacket(packet); err != nil {
			return fmt.Errorf("failed to send packet for channel %s: %v", channel.Name, err)
		}

		channel.SampleCount += int64(len(samples))
	}

	ltdg.packetsSent++
	return nil
}

// generateSamples generates synthetic seismic samples for real-time streaming
func (ltdg *LiveTestDataGenerator) generateSamples(channel *LiveChannelGenerator, count int) []int32 {
	samples := make([]int32, count)
	sampleInterval := 1.0 / ltdg.config.SampleRate
	currentTime := time.Now()

	for i := 0; i < count; i++ {
		sampleTime := currentTime.Add(time.Duration(float64(i) * sampleInterval * float64(time.Second)))
		
		// Update or create seismic event
		if ltdg.config.TriggerEvents {
			ltdg.updateSeismicEvent(channel, sampleTime)
		}

		// Generate sample value
		sample := ltdg.generateRealtimeSample(channel, sampleTime, sampleInterval)
		samples[i] = int32(sample)
		
		channel.LastSample = sample
	}

	return samples
}

// updateSeismicEvent updates or creates seismic events for a channel
func (ltdg *LiveTestDataGenerator) updateSeismicEvent(channel *LiveChannelGenerator, currentTime time.Time) {
	// Check if current event has ended
	if channel.CurrentEvent != nil {
		channel.CurrentEvent.Elapsed = currentTime.Sub(channel.CurrentEvent.StartTime)
		if channel.CurrentEvent.Elapsed >= channel.CurrentEvent.Duration {
			// Event has ended
			log.Printf("Seismic event ended on channel %s (magnitude %.1f, duration %.1fs)",
				channel.Name, channel.CurrentEvent.Magnitude, channel.CurrentEvent.Duration.Seconds())
			channel.CurrentEvent = nil
		}
	}

	// Check if we should start a new event (per minute probability)
	minuteProbability := ltdg.config.EventProbability * ltdg.config.PacketInterval.Minutes()
	if channel.CurrentEvent == nil && rand.Float64() < minuteProbability {
		// Start new event
		magnitude := 2.0 + rand.Float64()*3.0 // Magnitude 2-5
		duration := time.Duration(10+rand.Intn(120)) * time.Second // 10-130 seconds
		eventType := ltdg.chooseEventType()
		
		event := ltdg.createEvent(currentTime, magnitude, duration, eventType)
		channel.CurrentEvent = event
		
		log.Printf("Seismic event started on channel %s (type=%s, magnitude=%.1f, duration=%.1fs)",
			channel.Name, event.Type, event.Magnitude, event.Duration.Seconds())
	}
}

// chooseEventType randomly chooses an event type
func (ltdg *LiveTestDataGenerator) chooseEventType() string {
	eventTypes := []string{"local", "regional", "teleseismic"}
	weights := []float64{0.6, 0.3, 0.1} // Local events are most common
	
	r := rand.Float64()
	cumulative := 0.0
	
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return eventTypes[i]
		}
	}
	
	return "local"
}

// createEvent creates a seismic event with specified parameters
func (ltdg *LiveTestDataGenerator) createEvent(startTime time.Time, magnitude float64, duration time.Duration, eventType string) *LiveSeismicEvent {
	var frequency float64
	var amplitudeMultiplier float64
	
	switch eventType {
	case "local":
		frequency = 5.0 + rand.Float64()*10.0 // 5-15 Hz
		amplitudeMultiplier = 1.0
	case "regional":
		frequency = 1.0 + rand.Float64()*5.0 // 1-6 Hz
		amplitudeMultiplier = 0.7
	case "teleseismic":
		frequency = 0.1 + rand.Float64()*1.0 // 0.1-1.1 Hz
		amplitudeMultiplier = 0.3
	default:
		frequency = 5.0
		amplitudeMultiplier = 1.0
	}

	// Calculate peak amplitude based on magnitude
	peakAmplitude := math.Pow(10, magnitude-2) * 1000 * amplitudeMultiplier

	return &LiveSeismicEvent{
		StartTime:     startTime,
		Duration:      duration,
		Magnitude:     magnitude,
		Frequency:     frequency,
		PeakAmplitude: peakAmplitude,
		Elapsed:       0,
		Type:          eventType,
	}
}

// generateRealtimeSample generates a single sample value optimized for real-time streaming
func (ltdg *LiveTestDataGenerator) generateRealtimeSample(channel *LiveChannelGenerator, sampleTime time.Time, sampleInterval float64) float64 {
	// Update phase for continuous wave generation
	channel.Phase += 2 * math.Pi * sampleInterval
	if channel.Phase > 2*math.Pi {
		channel.Phase -= 2 * math.Pi
	}

	// Base noise with continuous characteristics
	noise := ltdg.generateContinuousNoise(channel, sampleInterval)
	
	// Add seismic event if active
	eventSignal := 0.0
	if channel.CurrentEvent != nil {
		eventSignal = ltdg.generateEventSignal(channel.CurrentEvent, sampleTime)
	}

	// Add low-frequency background (Earth hum, microseisms)
	backgroundSignal := ltdg.generateBackgroundSignal(sampleTime, channel.Phase)

	// Combine signals
	totalSignal := noise + eventSignal + backgroundSignal

	// Add some correlation to previous sample for realistic continuity
	correlation := 0.98
	totalSignal = correlation*channel.LastSample + (1-correlation)*totalSignal

	return totalSignal
}

// generateContinuousNoise generates continuous realistic noise
func (ltdg *LiveTestDataGenerator) generateContinuousNoise(channel *LiveChannelGenerator, sampleInterval float64) float64 {
	// Generate colored noise (not white noise)
	whiteNoise := (rand.Float64()*2 - 1) * channel.NoiseLevel
	
	// Apply simple low-pass filter to make it more realistic
	alpha := 0.1
	filteredNoise := alpha*whiteNoise + (1-alpha)*channel.LastSample*0.1
	
	return filteredNoise
}

// generateBackgroundSignal generates realistic background seismic signals
func (ltdg *LiveTestDataGenerator) generateBackgroundSignal(sampleTime time.Time, phase float64) float64 {
	t := float64(sampleTime.UnixNano()) / 1e9
	
	// Primary microseism (14-20 second period)
	primaryMicroseism := 10.0 * math.Sin(2*math.Pi*t/17.0 + phase*0.1)
	
	// Secondary microseism (6-10 second period)
	secondaryMicroseism := 15.0 * math.Sin(2*math.Pi*t/8.0 + phase*0.2)
	
	// Cultural noise (higher frequency, varies with time of day)
	hour := sampleTime.Hour()
	culturalFactor := 0.5 + 0.5*math.Sin(2*math.Pi*float64(hour)/24.0) // Varies throughout day
	culturalNoise := culturalFactor * 5.0 * math.Sin(2*math.Pi*t*0.5 + phase*0.3)
	
	// Tidal effects (very low frequency)
	tidalEffect := 2.0 * math.Sin(2*math.Pi*t/43200.0) // 12-hour period
	
	return primaryMicroseism + secondaryMicroseism + culturalNoise + tidalEffect
}

// generateEventSignal generates realistic earthquake signals
func (ltdg *LiveTestDataGenerator) generateEventSignal(event *LiveSeismicEvent, sampleTime time.Time) float64 {
	elapsed := sampleTime.Sub(event.StartTime).Seconds()
	if elapsed < 0 {
		return 0
	}

	normalizedTime := elapsed / event.Duration.Seconds()
	if normalizedTime > 1 {
		return 0
	}

	// Create realistic earthquake envelope based on event type
	var amplitude float64
	
	switch event.Type {
	case "local":
		// Quick onset, rapid decay
		if normalizedTime < 0.1 {
			amplitude = event.PeakAmplitude * (normalizedTime / 0.1) * (normalizedTime / 0.1)
		} else {
			amplitude = event.PeakAmplitude * math.Exp(-5*(normalizedTime-0.1))
		}
	case "regional":
		// Gradual onset, moderate decay
		if normalizedTime < 0.2 {
			amplitude = event.PeakAmplitude * (normalizedTime / 0.2)
		} else {
			amplitude = event.PeakAmplitude * math.Exp(-2*(normalizedTime-0.2))
		}
	case "teleseismic":
		// Very gradual onset, slow decay
		if normalizedTime < 0.3 {
			amplitude = event.PeakAmplitude * (normalizedTime / 0.3)
		} else {
			amplitude = event.PeakAmplitude * math.Exp(-1*(normalizedTime-0.3))
		}
	default:
		amplitude = event.PeakAmplitude * math.Exp(-normalizedTime)
	}

	// Generate complex waveform
	t := elapsed
	
	// Primary frequency component
	primaryWave := math.Sin(2 * math.Pi * event.Frequency * t)
	
	// Add harmonics and overtones
	harmonic1 := 0.3 * math.Sin(2 * math.Pi * event.Frequency * 1.5 * t)
	harmonic2 := 0.1 * math.Sin(2 * math.Pi * event.Frequency * 2.5 * t)
	
	// Add some modulation for realism
	modulation := 1.0 + 0.1*math.Sin(2*math.Pi*0.1*t)
	
	waveform := (primaryWave + harmonic1 + harmonic2) * modulation
	
	// Add some random variation
	randomFactor := 0.95 + 0.1*rand.Float64()
	
	return amplitude * waveform * randomFactor
}

// sendPacket sends a UDP packet in Raspberry Shake format
func (ltdg *LiveTestDataGenerator) sendPacket(packet *shakenet.UDPPacket) error {
	// Format packet as string (Raspberry Shake format)
	data := fmt.Sprintf("{'%s', %.6f", packet.Channel, packet.GetTimestamp())
	for _, sample := range packet.Data {
		data += fmt.Sprintf(", %d", sample)
	}
	data += "}\n"

	// Send UDP packet
	_, err := ltdg.conn.Write([]byte(data))
	return err
}

// GetStats returns live generation statistics
func (ltdg *LiveTestDataGenerator) GetStats() LiveGeneratorStats {
	duration := time.Since(ltdg.startTime)
	var samplesPerSecond float64
	if duration > 0 {
		totalSamples := ltdg.packetsSent * int64(ltdg.config.SamplesPerPacket) * int64(len(ltdg.config.Channels))
		samplesPerSecond = float64(totalSamples) / duration.Seconds()
	}

	return LiveGeneratorStats{
		PacketsSent:      ltdg.packetsSent,
		Duration:         duration,
		SamplesPerSecond: samplesPerSecond,
		IsRunning:        ltdg.isRunning,
		ActiveEvents:     ltdg.countActiveEvents(),
		TotalEvents:      ltdg.countTotalEvents(),
	}
}

// countActiveEvents counts currently active events
func (ltdg *LiveTestDataGenerator) countActiveEvents() int {
	count := 0
	for _, channel := range ltdg.channels {
		if channel.CurrentEvent != nil {
			count++
		}
	}
	return count
}

// countTotalEvents counts total events that have occurred
func (ltdg *LiveTestDataGenerator) countTotalEvents() int {
	// This is a simplified implementation
	// In a real implementation, you'd track this separately
	return int(ltdg.packetsSent / 100) // Rough estimate
}