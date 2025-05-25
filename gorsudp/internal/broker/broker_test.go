package broker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// Mock consumer for testing
type mockConsumer struct {
	id       string
	filters  []string
	events   []Event
	errors   bool
	errorMsg string
	done     chan struct{}
	mu       sync.Mutex
}

func (m *mockConsumer) ProcessEvent(event Event) error {
	m.mu.Lock()
	m.events = append(m.events, event)
	m.mu.Unlock()

	if m.done != nil {
		select {
		case m.done <- struct{}{}:
		default:
		}
	}
	if m.errors {
		return fmt.Errorf(m.errorMsg)
	}
	return nil
}

func (m *mockConsumer) GetID() string {
	return m.id
}

func (m *mockConsumer) GetEvents() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Event, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockConsumer) GetEventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *mockConsumer) GetChannelFilters() []string {
	return m.filters
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventData, "DATA"},
		{EventAlarm, "ALARM"},
		{EventReset, "RESET"},
		{EventImagePath, "IMGPATH"},
		{EventTerm, "TERM"},
		{EventType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.expected {
				t.Errorf("EventType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewMessageBroker(t *testing.T) {
	queueSize := 100
	broker := NewMessageBroker(queueSize)

	if broker == nil {
		t.Fatal("NewMessageBroker returned nil")
	}

	if broker.queueSize != queueSize {
		t.Errorf("queueSize = %d, want %d", broker.queueSize, queueSize)
	}

	if broker.consumers == nil {
		t.Error("consumers map is nil")
	}

	if broker.eventQueue == nil {
		t.Error("eventQueue is nil")
	}

	if cap(broker.eventQueue) != queueSize {
		t.Errorf("eventQueue capacity = %d, want %d", cap(broker.eventQueue), queueSize)
	}
}

func TestMessageBroker_StartStop(t *testing.T) {
	broker := NewMessageBroker(10)

	// Test starting
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !broker.running {
		t.Error("broker should be running after Start()")
	}

	// Test starting again (should fail)
	err = broker.Start()
	if err == nil {
		t.Error("Start() should fail when already running")
	}

	// Test stopping
	err = broker.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if broker.running {
		t.Error("broker should not be running after Stop()")
	}

	// Test stopping again (should fail)
	err = broker.Stop()
	if err == nil {
		t.Error("Stop() should fail when not running")
	}
}

func TestMessageBroker_RegisterConsumer(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		// Clean up consumers before stopping
		broker.UnregisterConsumer("test1")
		broker.UnregisterConsumer("test2")
		broker.Stop()
	}()

	consumer1 := &mockConsumer{id: "test1", filters: []string{"HZ"}}
	consumer2 := &mockConsumer{id: "test2", filters: []string{}}

	// Test registering consumers
	err = broker.RegisterConsumer(consumer1)
	if err != nil {
		t.Errorf("RegisterConsumer() failed: %v", err)
	}

	err = broker.RegisterConsumer(consumer2)
	if err != nil {
		t.Errorf("RegisterConsumer() failed: %v", err)
	}

	// Test registering duplicate consumer
	err = broker.RegisterConsumer(consumer1)
	if err == nil {
		t.Error("RegisterConsumer() should fail for duplicate ID")
	}

	// Check consumer info
	info := broker.GetConsumerInfo()
	if len(info) != 2 {
		t.Errorf("Expected 2 consumers, got %d", len(info))
	}

	if !info["test1"].Active {
		t.Error("Consumer test1 should be active")
	}
}

func TestMessageBroker_UnregisterConsumer(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer broker.Stop()

	consumer := &mockConsumer{id: "test", filters: []string{}}

	// Test unregistering non-existent consumer
	err = broker.UnregisterConsumer("nonexistent")
	if err == nil {
		t.Error("UnregisterConsumer() should fail for non-existent consumer")
	}

	// Register and then unregister
	broker.RegisterConsumer(consumer)
	err = broker.UnregisterConsumer("test")
	if err != nil {
		t.Errorf("UnregisterConsumer() failed: %v", err)
	}

	info := broker.GetConsumerInfo()
	if len(info) != 0 {
		t.Errorf("Expected 0 consumers after unregister, got %d", len(info))
	}
}

func TestMessageBroker_PublishEvent(t *testing.T) {
	broker := NewMessageBroker(2) // Small queue for testing full queue
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{}}
	broker.RegisterConsumer(consumer)

	// Test normal event publishing
	event := Event{
		Type:      EventAlarm,
		Timestamp: time.Now(),
		Data:      AlarmEvent{EventTime: time.Now(), Channel: "HZ", Ratio: 2.5},
	}

	err = broker.PublishEvent(event)
	if err != nil {
		t.Errorf("PublishEvent() failed: %v", err)
	}

	// Wait briefly for event processing
	time.Sleep(10 * time.Millisecond)

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	// Test metrics
	metrics := broker.GetMetrics()
	if metrics.TotalEvents != 1 {
		t.Errorf("Expected 1 total event, got %d", metrics.TotalEvents)
	}

	if metrics.EventsByType[EventAlarm] != 1 {
		t.Errorf("Expected 1 alarm event, got %d", metrics.EventsByType[EventAlarm])
	}
}

func TestMessageBroker_PublishData(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{"HZ"}, done: make(chan struct{}, 1)}
	broker.RegisterConsumer(consumer)

	// Create test packet
	packet := &shakenet.UDPPacket{
		Channel:   "HZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	err = broker.PublishData(packet)
	if err != nil {
		t.Errorf("PublishData() failed: %v", err)
	}

	// Wait for event processing
	select {
	case <-consumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event processing")
	}

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	if consumer.GetEvents()[0].Type != EventData {
		t.Errorf("Expected EventData, got %v", consumer.GetEvents()[0].Type)
	}
}

func TestMessageBroker_PublishAlarm(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{}, done: make(chan struct{}, 1)}
	broker.RegisterConsumer(consumer)

	eventTime := time.Now()
	err = broker.PublishAlarm(eventTime, "HZ", 3.2)
	if err != nil {
		t.Errorf("PublishAlarm() failed: %v", err)
	}

	// Wait for event processing
	select {
	case <-consumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for alarm processing")
	}

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	if consumer.GetEvents()[0].Type != EventAlarm {
		t.Errorf("Expected EventAlarm, got %v", consumer.GetEvents()[0].Type)
	}

	alarmData, ok := consumer.GetEvents()[0].Data.(AlarmEvent)
	if !ok {
		t.Error("Event data is not AlarmEvent")
	}

	if alarmData.Channel != "HZ" {
		t.Errorf("Expected channel HZ, got %s", alarmData.Channel)
	}

	if alarmData.Ratio != 3.2 {
		t.Errorf("Expected ratio 3.2, got %f", alarmData.Ratio)
	}
}

func TestMessageBroker_PublishReset(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{}, done: make(chan struct{}, 1)}
	broker.RegisterConsumer(consumer)

	resetTime := time.Now()
	err = broker.PublishReset(resetTime, "HZ")
	if err != nil {
		t.Errorf("PublishReset() failed: %v", err)
	}

	// Wait for event processing
	select {
	case <-consumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for reset processing")
	}

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	if consumer.GetEvents()[0].Type != EventReset {
		t.Errorf("Expected EventReset, got %v", consumer.GetEvents()[0].Type)
	}
}

func TestMessageBroker_PublishImagePath(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{}, done: make(chan struct{}, 1)}
	broker.RegisterConsumer(consumer)

	eventTime := time.Now()
	imagePath := "/tmp/screenshot.png"
	err = broker.PublishImagePath(eventTime, imagePath)
	if err != nil {
		t.Errorf("PublishImagePath() failed: %v", err)
	}

	// Wait for event processing
	select {
	case <-consumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for image path processing")
	}

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	if consumer.GetEvents()[0].Type != EventImagePath {
		t.Errorf("Expected EventImagePath, got %v", consumer.GetEvents()[0].Type)
	}

	imgData, ok := consumer.GetEvents()[0].Data.(ImagePathEvent)
	if !ok {
		t.Error("Event data is not ImagePathEvent")
	}

	if imgData.ImagePath != imagePath {
		t.Errorf("Expected image path %s, got %s", imagePath, imgData.ImagePath)
	}
}

func TestMessageBroker_PublishTerm(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("test")
		broker.Stop()
	}()

	consumer := &mockConsumer{id: "test", filters: []string{}, done: make(chan struct{}, 1)}
	broker.RegisterConsumer(consumer)

	reason := "shutdown"
	err = broker.PublishTerm(reason)
	if err != nil {
		t.Errorf("PublishTerm() failed: %v", err)
	}

	// Wait for event processing
	select {
	case <-consumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for term processing")
	}

	if consumer.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", consumer.GetEventCount())
	}

	if consumer.GetEvents()[0].Type != EventTerm {
		t.Errorf("Expected EventTerm, got %v", consumer.GetEvents()[0].Type)
	}

	termData, ok := consumer.GetEvents()[0].Data.(TermEvent)
	if !ok {
		t.Error("Event data is not TermEvent")
	}

	if termData.Reason != reason {
		t.Errorf("Expected reason %s, got %s", reason, termData.Reason)
	}
}

func TestMessageBroker_ChannelFiltering(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("hz_only")
		broker.UnregisterConsumer("all")
		broker.Stop()
	}()

	// Consumer that only wants HZ channel
	hzConsumer := &mockConsumer{id: "hz_only", filters: []string{"HZ"}, done: make(chan struct{}, 2)}
	// Consumer that wants all channels
	allConsumer := &mockConsumer{id: "all", filters: []string{}, done: make(chan struct{}, 2)}

	broker.RegisterConsumer(hzConsumer)
	broker.RegisterConsumer(allConsumer)

	// Send HZ data
	hzPacket := &shakenet.UDPPacket{Channel: "HZ", Timestamp: time.Now(), Data: []int32{100}}
	broker.PublishData(hzPacket)

	// Send HHZ data
	hhzPacket := &shakenet.UDPPacket{Channel: "HHZ", Timestamp: time.Now(), Data: []int32{200}}
	broker.PublishData(hhzPacket)

	// Wait for all events to be processed
	for i := 0; i < 2; i++ {
		select {
		case <-allConsumer.done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for all consumer processing")
		}
	}

	// Wait for HZ events to be processed
	select {
	case <-hzConsumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for hz consumer processing")
	}

	// HZ consumer should only get HZ data
	if hzConsumer.GetEventCount() != 1 {
		t.Errorf("HZ consumer expected 1 event, got %d", hzConsumer.GetEventCount())
	}

	// All consumer should get both
	if allConsumer.GetEventCount() != 2 {
		t.Errorf("All consumer expected 2 events, got %d", allConsumer.GetEventCount())
	}
}

func TestMessageBroker_ConsumerErrors(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer func() {
		broker.UnregisterConsumer("error")
		broker.Stop()
	}()

	// Consumer that always errors
	errorConsumer := &mockConsumer{
		id:       "error",
		filters:  []string{},
		errors:   true,
		errorMsg: "test error",
		done:     make(chan struct{}, 1),
	}

	broker.RegisterConsumer(errorConsumer)

	event := Event{Type: EventAlarm, Timestamp: time.Now(), Data: AlarmEvent{}}
	broker.PublishEvent(event)

	// Wait for error processing
	select {
	case <-errorConsumer.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for error processing")
	}

	metrics := broker.GetMetrics()
	if metrics.ConsumerErrors != 1 {
		t.Errorf("Expected 1 consumer error, got %d", metrics.ConsumerErrors)
	}
}

func TestMessageBroker_QueueFull(t *testing.T) {
	// Create broker with very small queue
	broker := NewMessageBroker(1)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer broker.Stop()

	// Fill the queue
	event1 := Event{Type: EventAlarm, Timestamp: time.Now(), Data: AlarmEvent{}}
	event2 := Event{Type: EventAlarm, Timestamp: time.Now(), Data: AlarmEvent{}}

	// First should succeed
	err = broker.PublishEvent(event1)
	if err != nil {
		t.Errorf("First PublishEvent() should succeed: %v", err)
	}

	// Second should fail (queue full)
	err = broker.PublishEvent(event2)
	if err == nil {
		t.Error("Second PublishEvent() should fail with full queue")
	}

	metrics := broker.GetMetrics()
	if metrics.DroppedEvents != 1 {
		t.Errorf("Expected 1 dropped event, got %d", metrics.DroppedEvents)
	}
}

func TestMessageBroker_StoppedPublish(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Stop the broker
	broker.Stop()

	// Brief delay to ensure context cancellation takes effect
	time.Sleep(time.Millisecond)

	// Try to publish after stopping
	event := Event{Type: EventAlarm, Timestamp: time.Now(), Data: AlarmEvent{}}
	err = broker.PublishEvent(event)
	if err == nil {
		t.Error("PublishEvent() should fail after broker is stopped")
	}
}

func TestMessageBroker_GetMetrics(t *testing.T) {
	broker := NewMessageBroker(10)
	err := broker.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer broker.Stop()

	// Publish various events
	broker.PublishAlarm(time.Now(), "HZ", 2.0)
	broker.PublishReset(time.Now(), "HZ")
	broker.PublishAlarm(time.Now(), "HZ", 3.0)

	// Metrics are updated synchronously
	metrics := broker.GetMetrics()

	if metrics.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got %d", metrics.TotalEvents)
	}

	if metrics.EventsByType[EventAlarm] != 2 {
		t.Errorf("Expected 2 alarm events, got %d", metrics.EventsByType[EventAlarm])
	}

	if metrics.EventsByType[EventReset] != 1 {
		t.Errorf("Expected 1 reset event, got %d", metrics.EventsByType[EventReset])
	}
}
