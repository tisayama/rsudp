package screenshot

import (
	"fmt"
	"log"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/config"
)

// ScreenshotConsumer processes screenshot events from the message broker
type ScreenshotConsumer struct {
	id       string
	config   config.Plot // Use plot config as it contains screenshot settings
	capturer *ScreenshotCapturer
}

// NewScreenshotConsumer creates a new screenshot consumer
func NewScreenshotConsumer(cfg config.Plot) (*ScreenshotConsumer, error) {
	if !cfg.Enabled || !cfg.EqScreenshots {
		return nil, fmt.Errorf("screenshot consumer is disabled")
	}

	// Create screenshot configuration
	screenshotConfig := ScreenshotConfig{
		Enabled:       cfg.EqScreenshots,
		OutputDir:     "./screenshots", // TODO: Get from global config
		Format:        "png",
		Quality:       90,
		Width:         1024,
		Height:        768,
		Delay:         2 * time.Second,
		IncludeAlerts: true,
		Watermark:     true,
	}

	// Create screenshot capturer
	capturer := NewScreenshotCapturer(screenshotConfig)

	consumer := &ScreenshotConsumer{
		id:       "screenshot",
		config:   cfg,
		capturer: capturer,
	}

	return consumer, nil
}

// Start starts the screenshot consumer
func (sc *ScreenshotConsumer) Start() error {
	log.Println("Starting screenshot consumer...")
	return sc.capturer.Start(2) // Start with 2 workers
}

// Stop stops the screenshot consumer
func (sc *ScreenshotConsumer) Stop() error {
	log.Println("Stopping screenshot consumer...")
	return sc.capturer.Stop()
}

// GetID returns the consumer ID
func (sc *ScreenshotConsumer) GetID() string {
	return sc.id
}

// GetChannelFilters returns the channels this consumer wants to receive
func (sc *ScreenshotConsumer) GetChannelFilters() []string {
	return []string{} // Accept all channels
}

// ProcessEvent processes events from the message broker
func (sc *ScreenshotConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventAlarm:
		return sc.processAlarm(event)
	case broker.EventReset:
		return sc.processReset(event)
	case broker.EventTerm:
		return sc.processTerm(event)
	default:
		// Ignore other event types
		return nil
	}
}

// processAlarm processes alarm events and captures screenshots
func (sc *ScreenshotConsumer) processAlarm(event broker.Event) error {
	alarmEvent, ok := event.Data.(broker.AlarmEvent)
	if !ok {
		return fmt.Errorf("invalid alarm event format")
	}

	// Create alert info for screenshot
	alertInfo := AlertInfo{
		Channel:     alarmEvent.Channel,
		Timestamp:   alarmEvent.EventTime,
		STALTARatio: alarmEvent.Ratio,
		Threshold:   0.0,     // Not available in broker event
		Duration:    0.0,     // Will be calculated later
		Station:     "Z0000", // TODO: Get from config
		Network:     "AM",    // TODO: Get from config
	}

	// Capture screenshot
	err := sc.capturer.CaptureAlertScreenshot(alertInfo)
	if err != nil {
		log.Printf("Failed to capture alert screenshot: %v", err)
		return err
	}

	log.Printf("Screenshot captured for alert on channel %s", alarmEvent.Channel)
	return nil
}

// processReset processes reset events
func (sc *ScreenshotConsumer) processReset(event broker.Event) error {
	resetEvent, ok := event.Data.(broker.ResetEvent)
	if !ok {
		return fmt.Errorf("invalid reset event format")
	}

	// Optionally capture reset screenshot
	request := ScreenshotRequest{
		ID:        fmt.Sprintf("reset_%d", time.Now().UnixNano()),
		Timestamp: resetEvent.ResetTime,
		EventType: "reset",
		Channel:   resetEvent.Channel,
		Priority:  5, // Lower priority than alerts
		Metadata: map[string]interface{}{
			"reset_time": resetEvent.ResetTime,
		},
	}

	err := sc.capturer.CaptureScreenshot(request)
	if err != nil {
		log.Printf("Failed to capture reset screenshot: %v", err)
		return err
	}

	log.Printf("Screenshot captured for reset on channel %s", resetEvent.Channel)
	return nil
}

// processTerm processes termination events
func (sc *ScreenshotConsumer) processTerm(event broker.Event) error {
	log.Println("Screenshot consumer received termination signal")

	// Print final statistics
	stats := sc.capturer.GetStats()
	log.Printf("Screenshot Statistics:")
	log.Printf("  Total Captured: %d", stats.TotalCaptured)
	log.Printf("  Successful: %d", stats.SuccessfulCaptured)
	log.Printf("  Failed: %d", stats.FailedCaptured)
	log.Printf("  Average File Size: %d KB", stats.AverageFileSize/1024)

	return nil
}

// GetStats returns screenshot capture statistics
func (sc *ScreenshotConsumer) GetStats() ScreenshotStats {
	return sc.capturer.GetStats()
}
