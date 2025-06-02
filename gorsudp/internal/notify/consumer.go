package notify

import (
	"fmt"
	"log"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/config"
)

// NotificationConsumer processes notification events from the message broker
type NotificationConsumer struct {
	id      string
	config  config.Config
	manager *NotificationManager
}

// NewNotificationConsumer creates a new notification consumer
func NewNotificationConsumer(cfg config.Config) (*NotificationConsumer, error) {
	// Check if any notification providers are enabled
	if !isAnyNotificationEnabled(cfg) {
		return nil, fmt.Errorf("no notification providers are enabled")
	}

	// Create notification manager config
	notifyConfig := NotificationConfig{
		Enabled:    true,
		Workers:    3,
		QueueSize:  100,
		RetryDelay: 5 * time.Second,
		MaxRetries: 3,
		Timeout:    30 * time.Second,
	}

	// Create notification manager
	manager := NewNotificationManager(notifyConfig)

	consumer := &NotificationConsumer{
		id:      "notification",
		config:  cfg,
		manager: manager,
	}

	// Register providers
	if err := consumer.registerProviders(); err != nil {
		return nil, fmt.Errorf("failed to register providers: %v", err)
	}

	return consumer, nil
}

// Start starts the notification consumer
func (nc *NotificationConsumer) Start() error {
	log.Println("Starting notification consumer...")
	return nc.manager.Start()
}

// Stop stops the notification consumer
func (nc *NotificationConsumer) Stop() error {
	log.Println("Stopping notification consumer...")
	return nc.manager.Stop()
}

// GetID returns the consumer ID
func (nc *NotificationConsumer) GetID() string {
	return nc.id
}

// GetChannelFilters returns the channels this consumer wants to receive
func (nc *NotificationConsumer) GetChannelFilters() []string {
	return []string{} // Accept all channels
}

// ProcessEvent processes events from the message broker
func (nc *NotificationConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventAlarm:
		return nc.processAlarm(event)
	case broker.EventReset:
		return nc.processReset(event)
	case broker.EventTerm:
		return nc.processTerm(event)
	default:
		// Ignore other event types
		return nil
	}
}

// processAlarm processes alarm events
func (nc *NotificationConsumer) processAlarm(event broker.Event) error {
	alarmEvent, ok := event.Data.(broker.AlarmEvent)
	if !ok {
		return fmt.Errorf("invalid alarm event format")
	}

	// Convert to notification alert format
	alert := Alert{
		Channel:     alarmEvent.Channel,
		Timestamp:   alarmEvent.EventTime,
		STALTARatio: alarmEvent.Ratio,
		Threshold:   0.0, // Not available in broker event
		Duration:    0.0, // Will be calculated from duration
		Station:     nc.config.Settings.Station,
		Network:     nc.config.Settings.Network,
	}

	// Send notification
	err := nc.manager.SendAlert(alert)
	if err != nil {
		log.Printf("Failed to send alert notification: %v", err)
		return err
	}

	log.Printf("Alert notification sent for channel %s (STA/LTA: %.2f)",
		alarmEvent.Channel, alarmEvent.Ratio)

	return nil
}

// processReset processes reset events
func (nc *NotificationConsumer) processReset(event broker.Event) error {
	resetEvent, ok := event.Data.(broker.ResetEvent)
	if !ok {
		return fmt.Errorf("invalid reset event format")
	}

	// Convert to notification reset format
	reset := Reset{
		Channel:   resetEvent.Channel,
		Timestamp: resetEvent.ResetTime,
		Station:   nc.config.Settings.Station,
		Network:   nc.config.Settings.Network,
		Duration:  0.0, // Will be calculated from duration
	}

	// Send notification
	err := nc.manager.SendReset(reset)
	if err != nil {
		log.Printf("Failed to send reset notification: %v", err)
		return err
	}

	log.Printf("Reset notification sent for channel %s", resetEvent.Channel)

	return nil
}

// processTerm processes termination events
func (nc *NotificationConsumer) processTerm(event broker.Event) error {
	log.Println("Notification consumer received termination signal")

	// Send shutdown notification to all providers
	message := fmt.Sprintf("🔴 System Shutdown\n\n"+
		"Station: %s.%s\n"+
		"Time: %s\n\n"+
		"GoRSUDP monitoring system is shutting down.",
		nc.config.Settings.Network, nc.config.Settings.Station,
		time.Now().Format("2006-01-02 15:04:05 UTC"))

	providers := nc.manager.GetProviders()
	for _, provider := range providers {
		if err := nc.manager.SendCustomMessage(provider, message); err != nil {
			log.Printf("Failed to send shutdown notification to %s: %v", provider, err)
		}
	}

	// Give some time for notifications to be sent
	time.Sleep(2 * time.Second)

	return nil
}

// registerProviders registers all enabled notification providers
func (nc *NotificationConsumer) registerProviders() error {
	var registeredCount int

	// Register Telegram provider
	if nc.config.Telegram.Enabled {
		telegramProvider := NewTelegramProvider(nc.config.Telegram)
		if err := telegramProvider.ValidateConfig(); err != nil {
			log.Printf("Telegram configuration invalid: %v", err)
		} else {
			if err := nc.manager.RegisterProvider(telegramProvider); err != nil {
				log.Printf("Failed to register Telegram provider: %v", err)
			} else {
				registeredCount++
			}
		}
	}

	// Register Discord provider
	if nc.config.Discord.Enabled {
		discordProvider := NewDiscordProvider(nc.config.Discord)
		if err := discordProvider.ValidateConfig(); err != nil {
			log.Printf("Discord configuration invalid: %v", err)
		} else {
			if err := nc.manager.RegisterProvider(discordProvider); err != nil {
				log.Printf("Failed to register Discord provider: %v", err)
			} else {
				registeredCount++
			}
		}
	}

	// TODO: Register other providers (Twitter, etc.)

	if registeredCount == 0 {
		return fmt.Errorf("no notification providers were successfully registered")
	}

	log.Printf("Registered %d notification providers", registeredCount)
	return nil
}

// isAnyNotificationEnabled checks if any notification provider is enabled
func isAnyNotificationEnabled(cfg config.Config) bool {
	return cfg.Telegram.Enabled ||
		cfg.Discord.Enabled ||
		cfg.Twitter.Enabled ||
		cfg.Bluesky.Enabled ||
		cfg.GoogleChat.Enabled ||
		cfg.LINE.Enabled ||
		cfg.SNS.Enabled
}
