package plot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"sync"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/internal/screenshot"
	"github.com/tisayama/gorsudp/pkg/config"
	"github.com/tisayama/gorsudp/pkg/dsp"
	"github.com/tisayama/gorsudp/pkg/stream"
)

// PlotConsumer processes events for the plotting system
type PlotConsumer struct {
	id            string
	config        config.Plot
	plotServer    *PlotServer
	stream        *stream.Stream
	channelFilter []string
	screenshotMgr *screenshot.ScreenshotManager
	filters       map[string]*dsp.ButterworthFilter

	// Data buffering
	dataBuffer      []PlotData
	dataBufferMutex sync.Mutex
	bufferTicker    *time.Ticker
	bufferStopChan  chan bool
}

// NewPlotConsumer creates a new plot consumer
func NewPlotConsumer(cfg config.Plot) (*PlotConsumer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("plot consumer is disabled")
	}

	// Set default host if not specified
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	// Set default port if not specified
	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	// Create plot config with defaults
	plotConfig := &PlotConfig{
		Enabled:              cfg.Enabled,
		Channels:             cfg.Channels,
		Duration:             cfg.Duration,
		Spectrogram:          cfg.Spectrogram,
		Fullscreen:           cfg.Fullscreen,
		Kiosk:                cfg.Kiosk,
		EQScreenshots:        cfg.EqScreenshots,
		Deconv:               cfg.Deconv,
		Units:                cfg.Units,
		RefreshInterval:      cfg.RefreshInterval,
		Host:                 host,
		Port:                 port,
		FilterWaveform:       cfg.FilterWaveform,
		FilterSpectrogram:    cfg.FilterSpectrogram,
		FilterHighpass:       cfg.FilterHighpass,
		FilterLowpass:        cfg.FilterLowpass,
		FilterCorners:        cfg.FilterCorners,
		SpectrogramFreqRange: cfg.SpectrogramFreqRange,
		UpperLimit:           cfg.UpperLimit,
		LowerLimit:           cfg.LowerLimit,
		LogarithmicYAxis:     cfg.LogarithmicYAxis,
	}

	consumer := &PlotConsumer{
		id:             "plot",
		config:         cfg,
		plotServer:     NewPlotServer(plotConfig),
		stream:         stream.NewStream(),
		channelFilter:  cfg.Channels,
		dataBuffer:     make([]PlotData, 0, 100), // Pre-allocate buffer
		bufferStopChan: make(chan bool),
		filters:        make(map[string]*dsp.ButterworthFilter),
	}

	// Initialize screenshot manager if eq_screenshots is enabled
	if cfg.EqScreenshots {
		url := fmt.Sprintf("http://%s:%d", host, port)
		consumer.screenshotMgr = screenshot.NewScreenshotManager(url, "/tmp/gorsudp-screenshots")
		log.Printf("Screenshot functionality enabled for URL: %s", url)
	}

	return consumer, nil
}

// Start starts the plot consumer
func (c *PlotConsumer) Start() error {
	log.Println("Starting plot consumer...")

	// Start plot server first
	if err := c.plotServer.Start(); err != nil {
		return err
	}

	// Buffer flush ticker disabled temporarily for debugging
	// c.bufferTicker = time.NewTicker(50 * time.Millisecond)
	// go c.bufferFlushLoop()

	// Start screenshot manager if enabled
	if c.screenshotMgr != nil {
		// Wait a moment for the web server to be ready
		time.Sleep(2 * time.Second)

		if err := c.screenshotMgr.Start(); err != nil {
			log.Printf("Warning: Screenshot manager failed to start: %v", err)
			// Don't fail the entire consumer if screenshots fail
		}
	}

	return nil
}

// Stop stops the plot consumer
func (c *PlotConsumer) Stop() error {
	log.Println("Stopping plot consumer...")

	// Buffer flush ticker disabled temporarily for debugging
	// if c.bufferTicker != nil {
	// 	c.bufferTicker.Stop()
	// 	c.bufferStopChan <- true
	// }
	//
	// // Flush any remaining buffered data
	// c.flushBuffer()

	// Stop screenshot manager first
	if c.screenshotMgr != nil {
		if err := c.screenshotMgr.Stop(); err != nil {
			log.Printf("Warning: Failed to stop screenshot manager: %v", err)
		}
	}

	return c.plotServer.Stop()
}

// GetID returns the consumer ID
func (c *PlotConsumer) GetID() string {
	return c.id
}

// GetChannelFilters returns the channels this consumer wants to receive
func (c *PlotConsumer) GetChannelFilters() []string {
	return c.channelFilter
}

// ProcessEvent processes events from the message broker
func (c *PlotConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventData:
		return c.processData(event)
	case broker.EventAlarm:
		return c.processAlarm(event)
	case broker.EventReset:
		return c.processReset(event)
	case broker.EventTerm:
		return c.processTerm(event)
	default:
		// Ignore other event types
		return nil
	}
}

// processData processes data events
func (c *PlotConsumer) processData(event broker.Event) error {
	dataEvent, ok := event.Data.(broker.DataEvent)
	if !ok {
		return fmt.Errorf("invalid data event format")
	}

	packet := dataEvent.Packet

	// Check if we should process this channel
	if !c.shouldProcessChannel(packet.Channel) {
		return nil
	}

	// Update stream with packet data
	err := c.stream.UpdateFromPacket(packet, "Z0000", "AM")
	if err != nil {
		return fmt.Errorf("failed to update stream: %v", err)
	}

	// Convert packet data to float64
	samples := make([]float64, len(packet.Data))
	for i, val := range packet.Data {
		samples[i] = float64(val)
	}

	// Calculate individual timestamps for each sample
	// Round base timestamp to nearest millisecond to avoid JavaScript precision issues
	baseTimestampNanos := int64(packet.GetTimestamp() * 1e9)
	// Round to nearest millisecond (1e6 nanoseconds)
	baseTimestampMillis := (baseTimestampNanos + 5e5) / 1e6 * 1e6
	baseTimestamp := time.Unix(0, baseTimestampMillis)

	sampleRate := 100.0                               // TODO: Get from packet or config
	sampleInterval := time.Duration(1e9 / sampleRate) // nanoseconds per sample (10ms)

	sampleTimestamps := make([]time.Time, len(samples))
	for i := range samples {
		// Each sample timestamp is exactly 10ms apart
		sampleTimestamps[i] = baseTimestamp.Add(time.Duration(i) * sampleInterval)
	}

	// Apply proper deconvolution: convert raw counts to physical units
	// Based on Python rsudp investigation:
	// - Geophone channels use sensitivity of 1.6e8 counts/(m/s)
	// - Accelerometer channels use sensitivity of 4.2e8 counts/(m/s²)
	physicalSamples := make([]float64, len(samples))
	for i, sample := range samples {
		// Apply correct sensitivity based on channel type
		if strings.Contains(packet.Channel, "EH") || strings.Contains(packet.Channel, "SH") {
			// ジオフォン (EHZ, EHN, EHE, SHZ): 1.6e8 counts/(m/s)
			physicalSamples[i] = sample / 1.6e8
		} else if strings.Contains(packet.Channel, "EN") {
			// 加速度計 (ENZ, ENN, ENE): 4.2e8 counts/(m/s²)
			physicalSamples[i] = sample / 4.2e8
		} else if strings.Contains(packet.Channel, "HDF") {
			// 圧力センサー: 1.0e5 counts/Pa (仮定値)
			physicalSamples[i] = sample / 1.0e5
		} else {
			// その他のチャンネル: raw counts (変換なし)
			physicalSamples[i] = sample
		}
	}

	// Convert to plot data format
	plotData := PlotData{
		Channel:          packet.Channel,
		Timestamp:        baseTimestamp,
		Samples:          physicalSamples,
		SampleRate:       sampleRate,
		Units:            c.getUnits(packet.Channel),
		SampleTimestamps: sampleTimestamps,
	}

	// Apply detrend (remove DC offset) similar to Python implementation
	plotData.Samples = dsp.RemoveDCOffset(plotData.Samples)

	if c.config.FilterWaveform {
		if _, ok := c.filters[packet.Channel]; !ok {
			filter, err := dsp.NewButterworthFilter(dsp.FilterBandpass, c.config.FilterCorners, c.config.FilterHighpass, c.config.FilterLowpass, 100.0)
			if err != nil {
				log.Printf("failed to create filter for channel %s: %v", packet.Channel, err)
			} else {
				c.filters[packet.Channel] = filter
			}
		}
		if filter, ok := c.filters[packet.Channel]; ok {
			// Use zero-phase filtering like ObsPy
			plotData.Samples = filter.ApplyZeroPhase(plotData.Samples)
		}
	}

	// Send directly to plot server (reverted from buffering)
	c.plotServer.AddPlotData(plotData)

	// Buffering disabled temporarily for debugging
	// c.dataBufferMutex.Lock()
	// c.dataBuffer = append(c.dataBuffer, plotData)
	//
	// // Flush if buffer is getting large
	// if len(c.dataBuffer) >= 10 {
	// 	c.flushBufferLocked()
	// }
	// c.dataBufferMutex.Unlock()

	return nil
}

// processAlarm processes alarm events
func (c *PlotConsumer) processAlarm(event broker.Event) error {
	alarmEvent, ok := event.Data.(broker.AlarmEvent)
	if !ok {
		return fmt.Errorf("invalid alarm event format")
	}

	// Create alert message for plot system
	alertMsg := AlertMessage{
		Channel:     alarmEvent.Channel,
		Timestamp:   alarmEvent.EventTime,
		STALTARatio: alarmEvent.Ratio,
		Threshold:   0.0, // Not available in broker event
		Duration:    0.0, // Not available in broker event
		Message:     fmt.Sprintf("Earthquake detected on channel %s", alarmEvent.Channel),
	}

	// Send alert to plot server
	c.plotServer.AddAlert(alertMsg)

	// Take screenshot if enabled and screenshot manager is running
	if c.screenshotMgr != nil && c.screenshotMgr.IsRunning() {
		go c.takeEarthquakeScreenshot(alarmEvent)
	}

	log.Printf("Plot system: Alert sent for channel %s (STA/LTA: %.2f)",
		alarmEvent.Channel, alarmEvent.Ratio)

	return nil
}

// takeEarthquakeScreenshot takes a screenshot when an earthquake is detected
func (c *PlotConsumer) takeEarthquakeScreenshot(alarmEvent broker.AlarmEvent) {
	// Wait a moment for the plot to update with alert information
	time.Sleep(2 * time.Second)

	// Generate filename with timestamp and channel
	timestamp := alarmEvent.EventTime.Format("20060102_150405")
	filename := fmt.Sprintf("earthquake_%s_%s_%.2f.png",
		timestamp, alarmEvent.Channel, alarmEvent.Ratio)

	log.Printf("Taking earthquake screenshot: %s", filename)

	if err := c.screenshotMgr.TakeScreenshot(filename); err != nil {
		log.Printf("Failed to take earthquake screenshot: %v", err)
	} else {
		log.Printf("Earthquake screenshot saved: %s", filename)
	}
}

// processReset processes reset events
func (c *PlotConsumer) processReset(event broker.Event) error {
	resetEvent, ok := event.Data.(broker.ResetEvent)
	if !ok {
		return fmt.Errorf("invalid reset event format")
	}

	// Create reset message for plot system
	alertMsg := AlertMessage{
		Channel:     resetEvent.Channel,
		Timestamp:   resetEvent.ResetTime,
		STALTARatio: 0.0, // Not available in reset event
		Threshold:   0.0, // Not available in reset event
		Duration:    0,
		Message:     fmt.Sprintf("Alert reset on channel %s", resetEvent.Channel),
	}

	// Send reset notification to plot server (with different styling)
	message := WebSocketMessage{
		Type: MessageTypeReset,
		Data: alertMsg,
	}

	c.plotServer.wsManager.Broadcast(message)

	log.Printf("Plot system: Reset notification sent for channel %s", resetEvent.Channel)

	return nil
}

// processTerm processes termination events
func (c *PlotConsumer) processTerm(event broker.Event) error {
	log.Println("Plot consumer received termination signal")

	// Send system shutdown message to connected clients
	message := WebSocketMessage{
		Type: MessageTypeSystemStatus,
		Data: map[string]interface{}{
			"status":    "shutting_down",
			"timestamp": time.Now(),
			"message":   "System is shutting down",
		},
	}

	c.plotServer.wsManager.Broadcast(message)

	// Give clients time to receive the message
	time.Sleep(1 * time.Second)

	return nil
}

// shouldProcessChannel checks if this channel should be processed
func (c *PlotConsumer) shouldProcessChannel(channel string) bool {
	// If no filters specified, process all channels
	if len(c.channelFilter) == 0 {
		return true
	}

	// Check if "all" is specified
	for _, filter := range c.channelFilter {
		if filter == "all" {
			return true
		}
		if filter == channel {
			return true
		}
		// Support wildcard matching (e.g., "H*" matches "HZ", "HN", "HE")
		if strings.HasSuffix(filter, "*") {
			prefix := strings.TrimSuffix(filter, "*")
			if strings.HasPrefix(channel, prefix) {
				return true
			}
		}
	}

	return false
}

// getUnits returns the appropriate units for a channel
func (c *PlotConsumer) getUnits(channel string) string {
	// Use configured units, or determine from channel name
	if c.config.Units != "" && c.config.Units != "CHAN" {
		return c.config.Units
	}

	// Determine units based on channel name
	if len(channel) >= 2 {
		switch channel[1] {
		case 'H': // High gain seismometer (velocity)
			return "m/s"
		case 'L': // Low gain seismometer (velocity)
			return "m/s"
		case 'B': // Broadband seismometer (velocity)
			return "m/s"
		case 'N': // Accelerometer (acceleration)
			return "m/s²"
		case 'E': // Extremely broadband seismometer
			return "m/s"
		default:
			return "counts"
		}
	}

	return "counts"
}

// bufferFlushLoop periodically flushes buffered data
func (c *PlotConsumer) bufferFlushLoop() {
	for {
		select {
		case <-c.bufferTicker.C:
			c.flushBuffer()
		case <-c.bufferStopChan:
			return
		}
	}
}

// flushBuffer sends all buffered data to the plot server
func (c *PlotConsumer) flushBuffer() {
	c.dataBufferMutex.Lock()
	defer c.dataBufferMutex.Unlock()

	c.flushBufferLocked()
}

// flushBufferLocked sends buffered data (must be called with mutex locked)
func (c *PlotConsumer) flushBufferLocked() {
	if len(c.dataBuffer) == 0 {
		return
	}

	// Send all buffered data
	for _, data := range c.dataBuffer {
		c.plotServer.AddPlotData(data)
	}

	// Clear buffer
	c.dataBuffer = c.dataBuffer[:0]
}

// GetPlotServer returns the plot server instance
func (c *PlotConsumer) GetPlotServer() *PlotServer {
	return c.plotServer
}
