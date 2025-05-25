// Package alert provides earthquake detection functionality using STA/LTA algorithm
package alert

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/config"
	"github.com/tisayama/gorsudp/pkg/dsp"
	"github.com/tisayama/gorsudp/pkg/shakenet"
	"github.com/tisayama/gorsudp/pkg/stream"
)

// AlertConsumer implements earthquake detection using STA/LTA algorithm
type AlertConsumer struct {
	id            string
	config        config.Alert
	processor     *dsp.STALTAProcessor
	filter        *dsp.ButterworthFilter
	broker        *broker.MessageBroker
	targetChannel string
	
	// Stream processing
	stream        *stream.Stream
	lastAlarmTime time.Time
	lastResetTime time.Time
	maxRatio      float64
	
	// Duration-based triggering
	triggerTimer     *time.Timer
	triggerStartTime time.Time
	durationMode     bool
	
	// Periodic logging
	logTimer        *time.Timer
	lastLogTime     time.Time
	logInterval     time.Duration
	
	// Statistics
	totalSamples   int64
	totalTriggers  int64
	falseAlarms    int64
	lastTriggerAt  time.Time
	
	mutex sync.RWMutex
}

// NewAlertConsumer creates a new alert consumer
func NewAlertConsumer(cfg config.Alert, messageBroker *broker.MessageBroker) (*AlertConsumer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("alert consumer is disabled")
	}
	
	// Validate configuration
	if cfg.STA <= 0 || cfg.LTA <= 0 {
		return nil, fmt.Errorf("STA and LTA durations must be positive")
	}
	if cfg.STA >= cfg.LTA {
		return nil, fmt.Errorf("STA duration must be less than LTA duration")
	}
	if cfg.Threshold <= 0 || cfg.Reset <= 0 {
		return nil, fmt.Errorf("threshold and reset values must be positive")
	}
	
	consumer := &AlertConsumer{
		id:           "alert",
		config:       cfg,
		broker:       messageBroker,
		stream:       stream.NewStream(),
		durationMode: cfg.Duration > 0,
	}
	
	// Set up periodic logging if configured
	if cfg.LogInterval > 0 {
		consumer.logInterval = time.Duration(cfg.LogInterval * float64(time.Second))
		consumer.startPeriodicLogging()
	}
	
	// Determine target channel
	consumer.targetChannel = consumer.resolveChannel(cfg.Channel)
	
	// Create STA/LTA processor
	staltagConfig := dsp.STALTAConfig{
		STADuration:  cfg.STA,
		LTADuration:  cfg.LTA,
		Threshold:    cfg.Threshold,
		Reset:        cfg.Reset,
		SampleRate:   100.0, // Default sample rate, will be updated from data
		UseRecursive: true,  // Use recursive for performance
		EnergyBased:  true,  // Use energy-based calculation
	}
	
	var err error
	consumer.processor, err = dsp.NewSTALTAProcessor(staltagConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create STA/LTA processor: %v", err)
	}
	
	// Create filter if specified
	if cfg.Highpass > 0 || cfg.Lowpass > 0 {
		if err := consumer.createFilter(); err != nil {
			return nil, fmt.Errorf("failed to create filter: %v", err)
		}
	}
	
	log.Printf("Alert consumer created: channel=%s, STA=%.1fs, LTA=%.1fs, threshold=%.2f", 
		consumer.targetChannel, cfg.STA, cfg.LTA, cfg.Threshold)
	
	return consumer, nil
}

// startPeriodicLogging starts the periodic STA/LTA logging timer
func (c *AlertConsumer) startPeriodicLogging() {
	if c.logInterval <= 0 {
		return
	}
	
	c.logTimer = time.NewTimer(c.logInterval)
	go func() {
		for {
			select {
			case <-c.logTimer.C:
				c.logCurrentSTALTA()
				c.logTimer.Reset(c.logInterval)
			}
		}
	}()
	
	log.Printf("Periodic STA/LTA logging enabled: interval=%.1fs", c.logInterval.Seconds())
}

// logCurrentSTALTA logs the current STA/LTA ratio periodically
func (c *AlertConsumer) logCurrentSTALTA() {
	c.mutex.RLock()
	ratio := c.processor.GetRatio()
	isTriggered := c.processor.IsTriggered()
	stats := c.processor.GetStats()
	c.mutex.RUnlock()
	
	now := time.Now()
	
	// Log current STA/LTA values similar to Python implementation
	if isTriggered {
		log.Printf("STA/LTA: %.3f (TRIGGERED) - STA=%.3f LTA=%.3f samples=%d at %s",
			ratio, stats.STAMean, stats.LTAMean, stats.SampleCount, now.Format("15:04:05.000"))
	} else {
		log.Printf("STA/LTA: %.3f - STA=%.3f LTA=%.3f samples=%d at %s",
			ratio, stats.STAMean, stats.LTAMean, stats.SampleCount, now.Format("15:04:05.000"))
	}
}

// resolveChannel resolves channel names to actual channel codes
func (c *AlertConsumer) resolveChannel(channel string) string {
	channel = strings.ToUpper(channel)
	
	// Handle partial channel names
	switch channel {
	case "HZ", "Z":
		return "EHZ" // Default to geophone vertical
	case "HN", "N":
		return "EHN" // Default to geophone north
	case "HE", "E":
		return "EHE" // Default to geophone east
	case "ALL":
		return "EHZ" // Default to vertical for "all"
	default:
		return channel
	}
}

// createFilter creates a bandpass filter based on configuration
func (c *AlertConsumer) createFilter() error {
	sampleRate := 100.0 // Default sample rate
	
	var filterType dsp.FilterType
	var freqLow, freqHigh float64
	
	if c.config.Highpass > 0 && c.config.Lowpass > 0 {
		// Bandpass filter
		filterType = dsp.FilterBandpass
		freqLow = c.config.Highpass
		freqHigh = c.config.Lowpass
	} else if c.config.Lowpass > 0 {
		// Lowpass filter
		filterType = dsp.FilterLowpass
		freqHigh = c.config.Lowpass
	} else if c.config.Highpass > 0 {
		// Highpass filter
		filterType = dsp.FilterHighpass
		freqLow = c.config.Highpass
	} else {
		return nil // No filtering
	}
	
	var err error
	c.filter, err = dsp.NewButterworthFilter(filterType, 2, freqLow, freqHigh, sampleRate)
	if err != nil {
		return err
	}
	
	log.Printf("Alert filter created: type=%s, freqLow=%.1f, freqHigh=%.1f", 
		filterType, freqLow, freqHigh)
	
	return nil
}

// GetID returns the consumer ID
func (c *AlertConsumer) GetID() string {
	return c.id
}

// GetChannelFilters returns the channels this consumer wants to receive
func (c *AlertConsumer) GetChannelFilters() []string {
	return []string{c.targetChannel}
}

// ProcessEvent processes events from the message broker
func (c *AlertConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventData:
		return c.processData(event)
	case broker.EventTerm:
		return c.processTerm(event)
	default:
		// Ignore other event types
		return nil
	}
}

// processData processes incoming data events
func (c *AlertConsumer) processData(event broker.Event) error {
	dataEvent, ok := event.Data.(broker.DataEvent)
	if !ok {
		return fmt.Errorf("invalid data event type")
	}
	
	packet := dataEvent.Packet
	
	// Check if this packet is for our target channel
	if !c.isTargetChannel(packet.Channel) {
		return nil // Not our channel
	}
	
	// Update stream with packet data
	err := c.stream.UpdateFromPacket(packet, "STATION", "AM")
	if err != nil {
		return fmt.Errorf("failed to update stream: %v", err)
	}
	
	// Process the data for earthquake detection
	return c.processSeismicData(packet)
}

// isTargetChannel checks if the packet channel matches our target
func (c *AlertConsumer) isTargetChannel(channel string) bool {
	// Exact match
	if channel == c.targetChannel {
		return true
	}
	
	// Partial match for HZ channels
	if c.targetChannel == "EHZ" && strings.HasSuffix(channel, "HZ") {
		return true
	}
	if c.targetChannel == "EHN" && strings.HasSuffix(channel, "HN") {
		return true
	}
	if c.targetChannel == "EHE" && strings.HasSuffix(channel, "HE") {
		return true
	}
	
	return false
}

// processSeismicData processes seismic data through the STA/LTA algorithm
func (c *AlertConsumer) processSeismicData(packet *shakenet.UDPPacket) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Convert int32 data to float64
	data := make([]float64, len(packet.Data))
	for i, sample := range packet.Data {
		data[i] = float64(sample)
	}
	
	// Apply filter if configured
	if c.filter != nil {
		data = c.filter.ApplySlice(data)
	}
	
	// Process each sample through STA/LTA
	for i, sample := range data {
		c.totalSamples++
		
		// Calculate sample timestamp
		sampleTime := packet.Timestamp.Add(time.Duration(i) * time.Second / time.Duration(packet.SampleRate()))
		
		// Process through STA/LTA
		triggered, ratio := c.processor.Process(sample)
		
		// Track maximum ratio during alarm state
		if c.processor.IsTriggered() && ratio > c.maxRatio {
			c.maxRatio = ratio
		}
		
		// Handle trigger state changes
		if triggered && !c.wasTriggered() {
			// New trigger detected
			if c.durationMode {
				c.handleDurationTrigger(sampleTime, ratio)
			} else {
				c.handleImmediateTrigger(sampleTime, ratio)
			}
		} else if !triggered && c.wasTriggered() {
			// Trigger reset
			c.handleTriggerReset(sampleTime)
		}
	}
	
	return nil
}

// wasTriggered checks if we were previously in triggered state
func (c *AlertConsumer) wasTriggered() bool {
	return !c.lastAlarmTime.IsZero() && c.lastResetTime.Before(c.lastAlarmTime)
}

// handleImmediateTrigger handles immediate triggering (no duration requirement)
func (c *AlertConsumer) handleImmediateTrigger(triggerTime time.Time, ratio float64) {
	c.lastAlarmTime = triggerTime
	c.totalTriggers++
	c.lastTriggerAt = triggerTime
	c.maxRatio = ratio
	
	log.Printf("EARTHQUAKE ALERT: STA/LTA=%.3f threshold=%.3f at %s", 
		ratio, c.config.Threshold, triggerTime.Format("2006-01-02 15:04:05.000"))
	
	// Send alarm event to broker
	if err := c.broker.PublishAlarm(triggerTime, c.targetChannel, ratio); err != nil {
		log.Printf("Failed to publish alarm event: %v", err)
	}
}

// handleDurationTrigger handles duration-based triggering
func (c *AlertConsumer) handleDurationTrigger(triggerTime time.Time, ratio float64) {
	if c.triggerTimer == nil {
		// Start duration timer
		c.triggerStartTime = triggerTime
		c.triggerTimer = time.NewTimer(time.Duration(c.config.Duration * float64(time.Second)))
		
		go func() {
			<-c.triggerTimer.C
			c.mutex.Lock()
			defer c.mutex.Unlock()
			
			// Check if still triggered after duration
			if c.processor.IsTriggered() {
				c.handleImmediateTrigger(c.triggerStartTime, c.processor.GetRatio())
			}
			c.triggerTimer = nil
		}()
		
		log.Printf("STA/LTA trigger started, waiting %.1fs for confirmation", c.config.Duration)
	}
}

// handleTriggerReset handles trigger reset events
func (c *AlertConsumer) handleTriggerReset(resetTime time.Time) {
	// Cancel duration timer if active
	if c.triggerTimer != nil {
		c.triggerTimer.Stop()
		c.triggerTimer = nil
		log.Printf("STA/LTA trigger cancelled before duration requirement met")
		return
	}
	
	c.lastResetTime = resetTime
	
	log.Printf("EARTHQUAKE RESET: max_ratio=%.3f reset=%.3f at %s", 
		c.maxRatio, c.config.Reset, resetTime.Format("2006-01-02 15:04:05.000"))
	
	// Send reset event to broker
	if err := c.broker.PublishReset(resetTime, c.targetChannel); err != nil {
		log.Printf("Failed to publish reset event: %v", err)
	}
	
	// Reset max ratio
	c.maxRatio = 0
}

// processTerm handles termination events
func (c *AlertConsumer) processTerm(event broker.Event) error {
	log.Printf("Alert consumer %s received termination signal", c.id)
	
	// Cancel any active timers
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if c.triggerTimer != nil {
		c.triggerTimer.Stop()
		c.triggerTimer = nil
	}
	
	if c.logTimer != nil {
		c.logTimer.Stop()
		c.logTimer = nil
	}
	
	return nil
}

// GetStats returns current statistics
func (c *AlertConsumer) GetStats() AlertStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return AlertStats{
		TotalSamples:     c.totalSamples,
		TotalTriggers:    c.totalTriggers,
		FalseAlarms:      c.falseAlarms,
		LastTriggerAt:    c.lastTriggerAt,
		LastAlarmTime:    c.lastAlarmTime,
		LastResetTime:    c.lastResetTime,
		MaxRatio:         c.maxRatio,
		CurrentRatio:     c.processor.GetRatio(),
		CurrentlyTriggered: c.processor.IsTriggered(),
		STALTAStats:      c.processor.GetStats(),
	}
}

// AlertStats contains statistics about the alert consumer
type AlertStats struct {
	TotalSamples       int64            `json:"total_samples"`
	TotalTriggers      int64            `json:"total_triggers"`
	FalseAlarms        int64            `json:"false_alarms"`
	LastTriggerAt      time.Time        `json:"last_trigger_at"`
	LastAlarmTime      time.Time        `json:"last_alarm_time"`
	LastResetTime      time.Time        `json:"last_reset_time"`
	MaxRatio           float64          `json:"max_ratio"`
	CurrentRatio       float64          `json:"current_ratio"`
	CurrentlyTriggered bool             `json:"currently_triggered"`
	STALTAStats        dsp.STALTAStats  `json:"stalta_stats"`
}

// IsCurrentlyTriggered returns whether an alert is currently active
func (c *AlertConsumer) IsCurrentlyTriggered() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return c.processor.IsTriggered()
}

// GetCurrentRatio returns the current STA/LTA ratio
func (c *AlertConsumer) GetCurrentRatio() float64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return c.processor.GetRatio()
}

// Reset resets the alert consumer state
func (c *AlertConsumer) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.processor.Reset()
	c.lastAlarmTime = time.Time{}
	c.lastResetTime = time.Time{}
	c.maxRatio = 0
	c.totalSamples = 0
	c.totalTriggers = 0
	c.falseAlarms = 0
	c.lastTriggerAt = time.Time{}
	
	if c.triggerTimer != nil {
		c.triggerTimer.Stop()
		c.triggerTimer = nil
	}
	
	// Note: Don't stop logTimer during Reset as it should continue running
	// Only stop it during termination
	
	if c.filter != nil {
		c.filter.Reset()
	}
	
	log.Printf("Alert consumer %s reset", c.id)
}