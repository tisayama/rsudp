// Package broker provides a channel-based message broker for distributing data
// between producers and consumers
package broker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// EventType represents different types of events in the system
type EventType int

const (
	EventData EventType = iota
	EventAlarm
	EventReset
	EventImagePath
	EventTerm
)

func (e EventType) String() string {
	switch e {
	case EventData:
		return "DATA"
	case EventAlarm:
		return "ALARM"
	case EventReset:
		return "RESET"
	case EventImagePath:
		return "IMGPATH"
	case EventTerm:
		return "TERM"
	default:
		return "UNKNOWN"
	}
}

// Event represents a message in the system
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// DataEvent contains UDP packet data
type DataEvent struct {
	Packet *shakenet.UDPPacket `json:"packet"`
}

// AlarmEvent contains alarm information
type AlarmEvent struct {
	EventTime time.Time `json:"event_time"`
	Channel   string    `json:"channel"`
	Ratio     float64   `json:"ratio"`
}

// ResetEvent contains reset information
type ResetEvent struct {
	ResetTime time.Time `json:"reset_time"`
	Channel   string    `json:"channel"`
}

// ImagePathEvent contains image file information
type ImagePathEvent struct {
	EventTime time.Time `json:"event_time"`
	ImagePath string    `json:"image_path"`
}

// TermEvent signals termination
type TermEvent struct {
	Reason string `json:"reason"`
}

// Consumer represents a consumer that can receive events
type Consumer interface {
	ProcessEvent(event Event) error
	GetID() string
	GetChannelFilters() []string // Empty slice means accept all channels
}

// MessageBroker manages event distribution to consumers
type MessageBroker struct {
	consumers    map[string]ConsumerInfo
	eventQueue   chan Event
	queueSize    int
	shutdown     chan struct{}
	wg           sync.WaitGroup
	mutex        sync.RWMutex
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	eventMetrics EventMetrics
}

// ConsumerInfo holds information about a registered consumer
type ConsumerInfo struct {
	Consumer Consumer
	Channel  chan Event
	Filters  []string
	Active   bool
}

// EventMetrics tracks broker performance
type EventMetrics struct {
	TotalEvents    int64
	EventsByType   map[EventType]int64
	DroppedEvents  int64
	ConsumerErrors int64
	LastEventTime  time.Time
	mutex          sync.RWMutex
}

// NewMessageBroker creates a new message broker with the specified queue size
func NewMessageBroker(queueSize int) *MessageBroker {
	ctx, cancel := context.WithCancel(context.Background())

	return &MessageBroker{
		consumers:  make(map[string]ConsumerInfo),
		eventQueue: make(chan Event, queueSize),
		queueSize:  queueSize,
		shutdown:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
		eventMetrics: EventMetrics{
			EventsByType: make(map[EventType]int64),
		},
	}
}

// Start begins processing events
func (mb *MessageBroker) Start() error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if mb.running {
		return fmt.Errorf("broker is already running")
	}

	mb.running = true
	mb.wg.Add(1)
	go mb.eventLoop()

	log.Printf("Message broker started with queue size %d", mb.queueSize)
	return nil
}

// Stop gracefully shuts down the broker
func (mb *MessageBroker) Stop() error {
	mb.mutex.Lock()
	if !mb.running {
		mb.mutex.Unlock()
		return fmt.Errorf("broker is not running")
	}
	mb.running = false
	mb.mutex.Unlock()

	// Signal shutdown and cancel context
	mb.cancel()
	close(mb.shutdown)

	// Wait for event loop to finish with timeout
	done := make(chan struct{})
	go func() {
		mb.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Normal shutdown completed
	case <-time.After(10 * time.Second):
		log.Println("Warning: Broker shutdown timeout exceeded, some goroutines may not have finished")
	}

	// Close the event queue channel
	close(mb.eventQueue)

	// Close all consumer channels
	mb.mutex.Lock()
	for id, info := range mb.consumers {
		close(info.Channel)
		log.Printf("Closed channel for consumer %s", id)
	}
	mb.mutex.Unlock()

	log.Println("Message broker stopped")
	return nil
}

// RegisterConsumer adds a new consumer to receive events
func (mb *MessageBroker) RegisterConsumer(consumer Consumer) error {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	id := consumer.GetID()
	if _, exists := mb.consumers[id]; exists {
		return fmt.Errorf("consumer %s already registered", id)
	}

	consumerChan := make(chan Event, 100) // Buffer for each consumer

	mb.consumers[id] = ConsumerInfo{
		Consumer: consumer,
		Channel:  consumerChan,
		Filters:  consumer.GetChannelFilters(),
		Active:   true,
	}

	// Start consumer goroutine
	mb.wg.Add(1)
	go mb.consumerLoop(id, consumerChan, consumer)

	log.Printf("Registered consumer: %s", id)
	return nil
}

// UnregisterConsumer removes a consumer
func (mb *MessageBroker) UnregisterConsumer(consumerID string) error {
	mb.mutex.Lock()
	info, exists := mb.consumers[consumerID]
	if !exists {
		mb.mutex.Unlock()
		return fmt.Errorf("consumer %s not found", consumerID)
	}

	info.Active = false
	delete(mb.consumers, consumerID)
	mb.mutex.Unlock()

	// Close channel after removing from map to avoid race conditions
	close(info.Channel)

	log.Printf("Unregistered consumer: %s", consumerID)
	return nil
}

// PublishEvent sends an event to all registered consumers
func (mb *MessageBroker) PublishEvent(event Event) error {
	mb.mutex.RLock()
	running := mb.running
	mb.mutex.RUnlock()

	if !running {
		return fmt.Errorf("broker is not running")
	}

	select {
	case mb.eventQueue <- event:
		mb.updateMetrics(event)
		return nil
	case <-mb.ctx.Done():
		return fmt.Errorf("broker is shutting down")
	default:
		// Queue is full, drop event
		mb.mutex.RLock()
		mb.eventMetrics.DroppedEvents++
		mb.mutex.RUnlock()
		return fmt.Errorf("event queue full, dropping event")
	}
}

// PublishData publishes a data event from a UDP packet
func (mb *MessageBroker) PublishData(packet *shakenet.UDPPacket) error {
	event := Event{
		Type:      EventData,
		Timestamp: time.Now(),
		Data: DataEvent{
			Packet: packet,
		},
	}
	return mb.PublishEvent(event)
}

// PublishAlarm publishes an alarm event
func (mb *MessageBroker) PublishAlarm(eventTime time.Time, channel string, ratio float64) error {
	event := Event{
		Type:      EventAlarm,
		Timestamp: time.Now(),
		Data: AlarmEvent{
			EventTime: eventTime,
			Channel:   channel,
			Ratio:     ratio,
		},
	}
	return mb.PublishEvent(event)
}

// PublishReset publishes a reset event
func (mb *MessageBroker) PublishReset(resetTime time.Time, channel string) error {
	event := Event{
		Type:      EventReset,
		Timestamp: time.Now(),
		Data: ResetEvent{
			ResetTime: resetTime,
			Channel:   channel,
		},
	}
	return mb.PublishEvent(event)
}

// PublishImagePath publishes an image path event
func (mb *MessageBroker) PublishImagePath(eventTime time.Time, imagePath string) error {
	event := Event{
		Type:      EventImagePath,
		Timestamp: time.Now(),
		Data: ImagePathEvent{
			EventTime: eventTime,
			ImagePath: imagePath,
		},
	}
	return mb.PublishEvent(event)
}

// PublishTerm publishes a termination event
func (mb *MessageBroker) PublishTerm(reason string) error {
	event := Event{
		Type:      EventTerm,
		Timestamp: time.Now(),
		Data: TermEvent{
			Reason: reason,
		},
	}
	return mb.PublishEvent(event)
}

// eventLoop is the main event processing loop
func (mb *MessageBroker) eventLoop() {
	defer mb.wg.Done()

	for {
		select {
		case event := <-mb.eventQueue:
			mb.distributeEvent(event)
		case <-mb.shutdown:
			log.Println("Event loop shutting down")
			return
		}
	}
}

// distributeEvent sends an event to all appropriate consumers
func (mb *MessageBroker) distributeEvent(event Event) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	for id, info := range mb.consumers {
		if !info.Active {
			continue
		}

		// Check if consumer should receive this event
		if mb.shouldReceiveEvent(info, event) {
			select {
			case info.Channel <- event:
				// Event sent successfully
			default:
				// Consumer channel is full, log warning
				log.Printf("Warning: Consumer %s channel full, dropping event", id)
			}
		}
	}
}

// shouldReceiveEvent determines if a consumer should receive an event
func (mb *MessageBroker) shouldReceiveEvent(info ConsumerInfo, event Event) bool {
	// Always send non-data events to all consumers
	if event.Type != EventData {
		return true
	}

	// If no filters, accept all
	if len(info.Filters) == 0 {
		return true
	}

	// Check channel filters for data events
	if dataEvent, ok := event.Data.(DataEvent); ok {
		channel := dataEvent.Packet.Channel
		for _, filter := range info.Filters {
			if filter == channel || filter == "all" {
				return true
			}
		}
	}

	return false
}

// consumerLoop handles events for a specific consumer
func (mb *MessageBroker) consumerLoop(consumerID string, eventChan <-chan Event, consumer Consumer) {
	defer mb.wg.Done()

	for event := range eventChan {
		if err := consumer.ProcessEvent(event); err != nil {
			log.Printf("Consumer %s error processing event: %v", consumerID, err)
			mb.mutex.Lock()
			mb.eventMetrics.ConsumerErrors++
			mb.mutex.Unlock()
		}
	}

	log.Printf("Consumer loop ended for %s", consumerID)
}

// updateMetrics updates broker metrics
func (mb *MessageBroker) updateMetrics(event Event) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	mb.eventMetrics.TotalEvents++
	mb.eventMetrics.EventsByType[event.Type]++
	mb.eventMetrics.LastEventTime = event.Timestamp
}

// GetMetrics returns current broker metrics
func (mb *MessageBroker) GetMetrics() EventMetrics {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	// Return copy to avoid race conditions
	metrics := EventMetrics{
		TotalEvents:    mb.eventMetrics.TotalEvents,
		DroppedEvents:  mb.eventMetrics.DroppedEvents,
		ConsumerErrors: mb.eventMetrics.ConsumerErrors,
		LastEventTime:  mb.eventMetrics.LastEventTime,
		EventsByType:   make(map[EventType]int64),
	}

	for k, v := range mb.eventMetrics.EventsByType {
		metrics.EventsByType[k] = v
	}

	return metrics
}

// GetConsumerInfo returns information about registered consumers
func (mb *MessageBroker) GetConsumerInfo() map[string]ConsumerInfo {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	info := make(map[string]ConsumerInfo)
	for k, v := range mb.consumers {
		info[k] = ConsumerInfo{
			Consumer: v.Consumer,
			Channel:  v.Channel,
			Filters:  append([]string(nil), v.Filters...), // Copy slice
			Active:   v.Active,
		}
	}

	return info
}
