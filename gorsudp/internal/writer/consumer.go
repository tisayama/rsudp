package writer

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/config"
	"github.com/tisayama/gorsudp/pkg/stream"
)

// WriterConsumer processes data events for continuous recording
type WriterConsumer struct {
	id             string
	config         config.Write
	miniSeedWriter *MiniSeedWriter
	stream         *stream.Stream
	channelFilter  []string
	stats          *WriteStats
	flushTicker    *time.Ticker
	stopChan       chan struct{}
}

// NewWriterConsumer creates a new writer consumer
func NewWriterConsumer(cfg config.Write) (*WriterConsumer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("writer consumer is disabled")
	}

	if cfg.OutDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	// Create writer configuration
	writerConfig := WriterConfig{
		OutputDir:     cfg.OutDir,
		Channels:      cfg.Channels,
		Compression:   CompressionNone, // Start with no compression
		RecordLength:  DefaultRecordLength,
		BufferSize:    DefaultBufferSize,
		FlushInterval: DefaultFlushInterval,
		FileFormat:    string(FormatDaily),
		CreateDirs:    true,
	}

	// Create MiniSEED writer
	miniSeedWriter, err := NewMiniSeedWriter(writerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create MiniSEED writer: %v", err)
	}

	consumer := &WriterConsumer{
		id:             "writer",
		config:         cfg,
		miniSeedWriter: miniSeedWriter,
		stream:         stream.NewStream(),
		channelFilter:  cfg.Channels,
		stats: &WriteStats{
			StartTime: time.Now(),
		},
		stopChan: make(chan struct{}),
	}

	// Start flush ticker
	consumer.flushTicker = time.NewTicker(writerConfig.FlushInterval)
	go consumer.flushLoop()

	return consumer, nil
}

// Start starts the writer consumer
func (wc *WriterConsumer) Start() error {
	log.Println("Writer consumer started")
	return nil
}

// Stop stops the writer consumer
func (wc *WriterConsumer) Stop() error {
	log.Println("Stopping writer consumer...")

	// Stop flush loop
	close(wc.stopChan)
	if wc.flushTicker != nil {
		wc.flushTicker.Stop()
	}

	// Close MiniSEED writer
	if wc.miniSeedWriter != nil {
		if err := wc.miniSeedWriter.Close(); err != nil {
			log.Printf("Error closing MiniSEED writer: %v", err)
			return err
		}
	}

	log.Println("Writer consumer stopped")
	return nil
}

// GetID returns the consumer ID
func (wc *WriterConsumer) GetID() string {
	return wc.id
}

// GetChannelFilters returns the channels this consumer wants to receive
func (wc *WriterConsumer) GetChannelFilters() []string {
	return wc.channelFilter
}

// ProcessEvent processes events from the message broker
func (wc *WriterConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventData:
		return wc.processData(event)
	case broker.EventTerm:
		return wc.processTerm(event)
	default:
		// Ignore other event types
		return nil
	}
}

// processData processes data events
func (wc *WriterConsumer) processData(event broker.Event) error {
	dataEvent, ok := event.Data.(broker.DataEvent)
	if !ok {
		return fmt.Errorf("invalid data event format")
	}

	packet := dataEvent.Packet

	// Check if we should process this channel
	if !wc.shouldProcessChannel(packet.Channel) {
		return nil
	}

	// Update stream with packet data
	err := wc.stream.UpdateFromPacket(packet, "Z0000", "AM")
	if err != nil {
		return fmt.Errorf("failed to update stream: %v", err)
	}

	// Convert timestamp from float64 seconds since epoch to time.Time
	timestamp := time.Unix(0, int64(packet.GetTimestamp()*1e9))

	// Write data to MiniSEED
	err = wc.miniSeedWriter.WriteData(
		packet.Channel,
		"AM",    // Network - TODO: Get from config
		"Z0000", // Station - TODO: Get from config
		"00",    // Location - TODO: Get from config
		timestamp,
		packet.Data,
		100.0, // Sample rate - TODO: Get from packet or config
	)

	if err != nil {
		wc.updateErrorStats()
		return fmt.Errorf("failed to write MiniSEED data: %v", err)
	}

	// Update statistics
	wc.updateWriteStats(len(packet.Data))

	// Log occasionally to avoid spam
	if wc.stats.RecordsWritten%100 == 0 {
		log.Printf("Writer: processed %d records, %d samples for channel %s",
			wc.stats.RecordsWritten, wc.stats.SamplesWritten, packet.Channel)
	}

	return nil
}

// processTerm processes termination events
func (wc *WriterConsumer) processTerm(event broker.Event) error {
	log.Println("Writer consumer received termination signal")

	// Flush all remaining data
	if err := wc.miniSeedWriter.Flush(); err != nil {
		log.Printf("Error flushing data during termination: %v", err)
	}

	// Print final statistics
	wc.printStats()

	return nil
}

// shouldProcessChannel checks if this channel should be processed
func (wc *WriterConsumer) shouldProcessChannel(channel string) bool {
	// If no filters specified, process all channels
	if len(wc.channelFilter) == 0 {
		return true
	}

	// Check if "all" is specified
	for _, filter := range wc.channelFilter {
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

// flushLoop periodically flushes buffered data
func (wc *WriterConsumer) flushLoop() {
	for {
		select {
		case <-wc.flushTicker.C:
			if err := wc.miniSeedWriter.Flush(); err != nil {
				log.Printf("Error during periodic flush: %v", err)
			}
		case <-wc.stopChan:
			return
		}
	}
}

// updateWriteStats updates writing statistics
func (wc *WriterConsumer) updateWriteStats(sampleCount int) {
	wc.stats.mutex.Lock()
	defer wc.stats.mutex.Unlock()

	wc.stats.RecordsWritten++
	wc.stats.SamplesWritten += int64(sampleCount)
	wc.stats.LastWrite = time.Now()
}

// updateErrorStats updates error statistics
func (wc *WriterConsumer) updateErrorStats() {
	wc.stats.mutex.Lock()
	defer wc.stats.mutex.Unlock()

	wc.stats.ErrorCount++
}

// GetStats returns current writing statistics
func (wc *WriterConsumer) GetStats() WriteStats {
	wc.stats.mutex.RLock()
	defer wc.stats.mutex.RUnlock()

	return WriteStats{
		RecordsWritten: wc.stats.RecordsWritten,
		SamplesWritten: wc.stats.SamplesWritten,
		BytesWritten:   wc.stats.BytesWritten,
		FilesCreated:   wc.stats.FilesCreated,
		LastWrite:      wc.stats.LastWrite,
		ErrorCount:     wc.stats.ErrorCount,
		StartTime:      wc.stats.StartTime,
	}
}

// printStats prints current statistics
func (wc *WriterConsumer) printStats() {
	wc.stats.mutex.RLock()
	defer wc.stats.mutex.RUnlock()

	duration := time.Since(wc.stats.StartTime)
	log.Printf("Writer Statistics:")
	log.Printf("  Records Written: %d", wc.stats.RecordsWritten)
	log.Printf("  Samples Written: %d", wc.stats.SamplesWritten)
	log.Printf("  Bytes Written: %d", wc.stats.BytesWritten)
	log.Printf("  Files Created: %d", wc.stats.FilesCreated)
	log.Printf("  Error Count: %d", wc.stats.ErrorCount)
	log.Printf("  Duration: %v", duration)
	if duration > 0 {
		samplesPerSec := float64(wc.stats.SamplesWritten) / duration.Seconds()
		log.Printf("  Avg Samples/sec: %.2f", samplesPerSec)
	}
}
