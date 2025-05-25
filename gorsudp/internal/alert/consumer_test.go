package alert

import (
	"testing"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/config"
	"github.com/tisayama/gorsudp/pkg/shakenet"
)

func TestNewAlertConsumer(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)

	tests := []struct {
		name        string
		config      config.Alert
		expectError bool
	}{
		{
			name: "valid config",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       5.0,
				LTA:       30.0,
				Threshold: 1.6,
				Reset:     1.55,
			},
			expectError: false,
		},
		{
			name: "disabled config",
			config: config.Alert{
				Enabled: false,
			},
			expectError: true,
		},
		{
			name: "invalid STA/LTA",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       30.0,
				LTA:       30.0,
				Threshold: 1.6,
				Reset:     1.55,
			},
			expectError: true,
		},
		{
			name: "negative threshold",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       5.0,
				LTA:       30.0,
				Threshold: -1.6,
				Reset:     1.55,
			},
			expectError: true,
		},
		{
			name: "with filter",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       5.0,
				LTA:       30.0,
				Threshold: 1.6,
				Reset:     1.55,
				Highpass:  1.0,
				Lowpass:   10.0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewAlertConsumer(tt.config, testBroker)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if consumer == nil {
				t.Error("Consumer is nil")
			}

			if consumer.GetID() != "alert" {
				t.Errorf("ID = %s, want alert", consumer.GetID())
			}
		})
	}
}

func TestAlertConsumer_resolveChannel(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       5.0,
		LTA:       30.0,
		Threshold: 1.6,
		Reset:     1.55,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"HZ", "EHZ"},
		{"Z", "EHZ"},
		{"HN", "EHN"},
		{"N", "EHN"},
		{"HE", "EHE"},
		{"E", "EHE"},
		{"ALL", "EHZ"},
		{"EHZ", "EHZ"},
		{"CUSTOM", "CUSTOM"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := consumer.resolveChannel(tt.input)
			if result != tt.expected {
				t.Errorf("resolveChannel(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAlertConsumer_GetChannelFilters(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       5.0,
		LTA:       30.0,
		Threshold: 1.6,
		Reset:     1.55,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	filters := consumer.GetChannelFilters()
	if len(filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(filters))
	}

	if filters[0] != "EHZ" {
		t.Errorf("Expected filter EHZ, got %s", filters[0])
	}
}

func TestAlertConsumer_isTargetChannel(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       5.0,
		LTA:       30.0,
		Threshold: 1.6,
		Reset:     1.55,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	tests := []struct {
		channel  string
		expected bool
	}{
		{"EHZ", true},
		{"SHZ", true},
		{"HHZ", true},
		{"EHN", false},
		{"EHE", false},
		{"OTHER", false},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result := consumer.isTargetChannel(tt.channel)
			if result != tt.expected {
				t.Errorf("isTargetChannel(%s) = %v, want %v", tt.channel, result, tt.expected)
			}
		})
	}
}

func TestAlertConsumer_ProcessEvent(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	// Test data event
	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	dataEvent := broker.Event{
		Type:      broker.EventData,
		Timestamp: time.Now(),
		Data: broker.DataEvent{
			Packet: packet,
		},
	}

	err = consumer.ProcessEvent(dataEvent)
	if err != nil {
		t.Errorf("ProcessEvent failed: %v", err)
	}

	// Test term event
	termEvent := broker.Event{
		Type:      broker.EventTerm,
		Timestamp: time.Now(),
		Data: broker.TermEvent{
			Reason: "shutdown",
		},
	}

	err = consumer.ProcessEvent(termEvent)
	if err != nil {
		t.Errorf("ProcessEvent failed for term event: %v", err)
	}

	// Test other event type (should be ignored)
	alarmEvent := broker.Event{
		Type:      broker.EventAlarm,
		Timestamp: time.Now(),
		Data: broker.AlarmEvent{
			EventTime: time.Now(),
			Channel:   "HZ",
			Ratio:     2.5,
		},
	}

	err = consumer.ProcessEvent(alarmEvent)
	if err != nil {
		t.Errorf("ProcessEvent should handle unknown events: %v", err)
	}
}

func TestAlertConsumer_processData(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	err := testBroker.Start()
	if err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}
	defer testBroker.Stop()

	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	// Test with target channel
	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	dataEvent := broker.Event{
		Type:      broker.EventData,
		Timestamp: time.Now(),
		Data: broker.DataEvent{
			Packet: packet,
		},
	}

	err = consumer.processData(dataEvent)
	if err != nil {
		t.Errorf("processData failed: %v", err)
	}

	// Test with non-target channel
	packet.Channel = "EHN"
	err = consumer.processData(dataEvent)
	if err != nil {
		t.Errorf("processData should handle non-target channels: %v", err)
	}

	// Test with invalid event data
	invalidEvent := broker.Event{
		Type:      broker.EventData,
		Timestamp: time.Now(),
		Data:      "invalid",
	}

	err = consumer.processData(invalidEvent)
	if err == nil {
		t.Error("processData should fail with invalid event data")
	}
}

func TestAlertConsumer_GetStats(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	stats := consumer.GetStats()

	if stats.TotalSamples != 0 {
		t.Errorf("Initial TotalSamples = %d, want 0", stats.TotalSamples)
	}

	if stats.TotalTriggers != 0 {
		t.Errorf("Initial TotalTriggers = %d, want 0", stats.TotalTriggers)
	}

	if stats.CurrentlyTriggered {
		t.Error("Should not be triggered initially")
	}
}

func TestAlertConsumer_IsCurrentlyTriggered(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	if consumer.IsCurrentlyTriggered() {
		t.Error("Should not be triggered initially")
	}
}

func TestAlertConsumer_GetCurrentRatio(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	ratio := consumer.GetCurrentRatio()
	if ratio != 0.0 {
		t.Errorf("Initial ratio = %f, want 0.0", ratio)
	}
}

func TestAlertConsumer_Reset(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	// Reset should not cause any errors
	consumer.Reset()

	stats := consumer.GetStats()
	if stats.TotalSamples != 0 {
		t.Errorf("After reset TotalSamples = %d, want 0", stats.TotalSamples)
	}

	if stats.TotalTriggers != 0 {
		t.Errorf("After reset TotalTriggers = %d, want 0", stats.TotalTriggers)
	}
}

func TestAlertConsumer_wasTriggered(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		t.Fatalf("NewAlertConsumer failed: %v", err)
	}

	// Initially not triggered
	if consumer.wasTriggered() {
		t.Error("Should not be triggered initially")
	}

	// Set alarm time
	consumer.lastAlarmTime = time.Now()
	if !consumer.wasTriggered() {
		t.Error("Should be triggered after alarm time set")
	}

	// Set reset time after alarm
	time.Sleep(time.Millisecond)
	consumer.lastResetTime = time.Now()
	if consumer.wasTriggered() {
		t.Error("Should not be triggered after reset")
	}
}

func TestAlertConsumer_createFilter(t *testing.T) {
	testBroker := broker.NewMessageBroker(10)

	tests := []struct {
		name     string
		config   config.Alert
		hasError bool
	}{
		{
			name: "bandpass filter",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       1.0,
				LTA:       5.0,
				Threshold: 2.0,
				Reset:     1.5,
				Highpass:  1.0,
				Lowpass:   10.0,
			},
			hasError: false,
		},
		{
			name: "lowpass filter",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       1.0,
				LTA:       5.0,
				Threshold: 2.0,
				Reset:     1.5,
				Lowpass:   10.0,
			},
			hasError: false,
		},
		{
			name: "highpass filter",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       1.0,
				LTA:       5.0,
				Threshold: 2.0,
				Reset:     1.5,
				Highpass:  1.0,
			},
			hasError: false,
		},
		{
			name: "no filter",
			config: config.Alert{
				Enabled:   true,
				Channel:   "HZ",
				STA:       1.0,
				LTA:       5.0,
				Threshold: 2.0,
				Reset:     1.5,
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewAlertConsumer(tt.config, testBroker)

			if tt.hasError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if consumer == nil {
				t.Error("Consumer is nil")
			}

			// Check if filter was created when expected
			hasFilterConfig := tt.config.Highpass > 0 || tt.config.Lowpass > 0
			hasFilter := consumer.filter != nil

			if hasFilterConfig && !hasFilter {
				t.Error("Filter should be created when configured")
			}

			if !hasFilterConfig && hasFilter {
				t.Error("Filter should not be created when not configured")
			}
		})
	}
}

func BenchmarkAlertConsumer_ProcessEvent(b *testing.B) {
	testBroker := broker.NewMessageBroker(10)
	config := config.Alert{
		Enabled:   true,
		Channel:   "HZ",
		STA:       1.0,
		LTA:       5.0,
		Threshold: 2.0,
		Reset:     1.5,
	}

	consumer, err := NewAlertConsumer(config, testBroker)
	if err != nil {
		b.Fatalf("NewAlertConsumer failed: %v", err)
	}

	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      make([]int32, 100),
	}

	// Fill with test data
	for i := range packet.Data {
		packet.Data[i] = int32(i * 10)
	}

	event := broker.Event{
		Type:      broker.EventData,
		Timestamp: time.Now(),
		Data: broker.DataEvent{
			Packet: packet,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consumer.ProcessEvent(event)
	}
}