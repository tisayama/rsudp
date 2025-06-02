package writer

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewMiniSeedWriter creates a new MiniSEED writer
func NewMiniSeedWriter(config WriterConfig) (*MiniSeedWriter, error) {
	if config.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	if config.RecordLength == 0 {
		config.RecordLength = DefaultRecordLength
	}

	if config.BufferSize == 0 {
		config.BufferSize = DefaultBufferSize
	}

	if config.FlushInterval == 0 {
		config.FlushInterval = DefaultFlushInterval
	}

	if config.FileFormat == "" {
		config.FileFormat = string(FormatDaily)
	}

	// Validate record length (must be power of 2)
	if !isPowerOfTwo(config.RecordLength) || config.RecordLength < 512 {
		return nil, fmt.Errorf("invalid record length: must be power of 2 and >= 512")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	writer := &MiniSeedWriter{
		outputDir:   config.OutputDir,
		compression: config.Compression,
		channels:    config.Channels,
		files:       make(map[string]*os.File),
		buffers:     make(map[string]*MiniSeedBuffer),
		config:      config,
	}

	log.Printf("MiniSEED writer created: dir=%s, compression=%s, record_length=%d",
		config.OutputDir, config.Compression, config.RecordLength)

	return writer, nil
}

// WriteData writes seismic data to MiniSEED format
func (msw *MiniSeedWriter) WriteData(channel, network, station, location string,
	timestamp time.Time, samples []int32, sampleRate float64) error {

	msw.mutex.Lock()
	defer msw.mutex.Unlock()

	// Check if we should process this channel
	if !msw.shouldProcessChannel(channel) {
		return nil
	}

	// Get or create buffer for this channel
	bufferKey := fmt.Sprintf("%s.%s.%s.%s", network, station, location, channel)
	buffer, exists := msw.buffers[bufferKey]

	if !exists {
		buffer = &MiniSeedBuffer{
			data:       make([]int32, 0, msw.config.BufferSize),
			startTime:  timestamp,
			sampleRate: sampleRate,
			maxSamples: msw.config.BufferSize,
			channel:    channel,
			network:    network,
			station:    station,
			location:   location,
		}
		msw.buffers[bufferKey] = buffer
	}

	// Add samples to buffer
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	for _, sample := range samples {
		buffer.data = append(buffer.data, sample)

		// Check if buffer is full
		if len(buffer.data) >= buffer.maxSamples {
			if err := msw.flushBuffer(buffer, bufferKey); err != nil {
				log.Printf("Error flushing buffer for %s: %v", bufferKey, err)
				return err
			}
		}
	}

	return nil
}

// Flush forces all buffers to be written to disk
func (msw *MiniSeedWriter) Flush() error {
	msw.mutex.Lock()
	defer msw.mutex.Unlock()

	for key, buffer := range msw.buffers {
		if len(buffer.data) > 0 {
			if err := msw.flushBuffer(buffer, key); err != nil {
				log.Printf("Error flushing buffer for %s: %v", key, err)
				return err
			}
		}
	}

	return nil
}

// Close closes all open files and flushes remaining data
func (msw *MiniSeedWriter) Close() error {
	log.Println("Closing MiniSEED writer...")

	// Flush all buffers
	if err := msw.Flush(); err != nil {
		log.Printf("Error flushing buffers during close: %v", err)
	}

	msw.mutex.Lock()
	defer msw.mutex.Unlock()

	// Close all open files
	for key, file := range msw.files {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file for %s: %v", key, err)
		}
	}

	msw.files = make(map[string]*os.File)
	msw.buffers = make(map[string]*MiniSeedBuffer)

	log.Println("MiniSEED writer closed")
	return nil
}

// flushBuffer writes buffer contents to MiniSEED file
func (msw *MiniSeedWriter) flushBuffer(buffer *MiniSeedBuffer, bufferKey string) error {
	if len(buffer.data) == 0 {
		return nil
	}

	// Get or create file for this channel
	file, err := msw.getOrCreateFile(buffer, bufferKey)
	if err != nil {
		return fmt.Errorf("failed to get file: %v", err)
	}

	// Create MiniSEED record
	record, err := msw.createMiniSeedRecord(buffer)
	if err != nil {
		return fmt.Errorf("failed to create MiniSEED record: %v", err)
	}

	// Write record to file
	if _, err := file.Write(record); err != nil {
		return fmt.Errorf("failed to write record: %v", err)
	}

	// Clear buffer and update start time
	samplesPerSecond := buffer.sampleRate
	duration := float64(len(buffer.data)) / samplesPerSecond
	buffer.startTime = buffer.startTime.Add(time.Duration(duration * float64(time.Second)))
	buffer.data = buffer.data[:0] // Clear but keep capacity

	return nil
}

// getOrCreateFile gets or creates a file for the given buffer
func (msw *MiniSeedWriter) getOrCreateFile(buffer *MiniSeedBuffer, bufferKey string) (*os.File, error) {
	// Generate filename based on format
	filename := msw.generateFilename(buffer)
	filePath := filepath.Join(msw.outputDir, filename)

	// Check if file is already open
	if file, exists := msw.files[bufferKey]; exists {
		// Check if we need to rotate file (e.g., new day)
		currentFilename := msw.generateFilename(buffer)
		expectedPath := filepath.Join(msw.outputDir, currentFilename)

		if file.Name() != expectedPath {
			// Need to rotate file
			file.Close()
			delete(msw.files, bufferKey)
		} else {
			return file, nil
		}
	}

	// Create directory if needed
	if msw.config.CreateDirs {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	// Open file for append
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", filePath, err)
	}

	msw.files[bufferKey] = file
	log.Printf("Opened MiniSEED file: %s", filePath)

	return file, nil
}

// generateFilename generates a filename based on the configured format
func (msw *MiniSeedWriter) generateFilename(buffer *MiniSeedBuffer) string {
	t := buffer.startTime

	switch FileNamingFormat(msw.config.FileFormat) {
	case FormatDaily:
		// YYYY.DDD.NN.SSS.LL.CCC.D.mseed
		yearDay := fmt.Sprintf("%04d.%03d", t.Year(), t.YearDay())
		return fmt.Sprintf("%s.%s.%s.%s.%s.D.mseed",
			yearDay, buffer.network, buffer.station, buffer.location, buffer.channel)

	case FormatHourly:
		// YYYY.DDD.HH.NN.SSS.LL.CCC.D.mseed
		yearDay := fmt.Sprintf("%04d.%03d.%02d", t.Year(), t.YearDay(), t.Hour())
		return fmt.Sprintf("%s.%s.%s.%s.%s.D.mseed",
			yearDay, buffer.network, buffer.station, buffer.location, buffer.channel)

	case FormatContinuous:
		// NN.SSS.LL.CCC.D.mseed
		return fmt.Sprintf("%s.%s.%s.%s.D.mseed",
			buffer.network, buffer.station, buffer.location, buffer.channel)

	default:
		// Default to daily format
		yearDay := fmt.Sprintf("%04d.%03d", t.Year(), t.YearDay())
		return fmt.Sprintf("%s.%s.%s.%s.%s.D.mseed",
			yearDay, buffer.network, buffer.station, buffer.location, buffer.channel)
	}
}

// createMiniSeedRecord creates a MiniSEED record from buffer data
func (msw *MiniSeedWriter) createMiniSeedRecord(buffer *MiniSeedBuffer) ([]byte, error) {
	numSamples := len(buffer.data)
	if numSamples == 0 {
		return nil, fmt.Errorf("no data to write")
	}

	// Create record buffer
	record := make([]byte, msw.config.RecordLength)

	// Fill header
	header := record[:MiniSeedHeaderSize]

	// Sequence number (6 bytes) - can be zeros for now
	copy(header[0:6], "000000")

	// Data header type and reserved byte
	header[6] = 'D' // Data record
	header[7] = ' ' // Reserved

	// Station code (5 bytes, space-padded)
	stationBytes := padString(buffer.station, 5)
	copy(header[8:13], stationBytes)

	// Location code (2 bytes, space-padded)
	locationBytes := padString(buffer.location, 2)
	copy(header[13:15], locationBytes)

	// Channel code (3 bytes, space-padded)
	channelBytes := padString(buffer.channel, 3)
	copy(header[15:18], channelBytes)

	// Network code (2 bytes, space-padded)
	networkBytes := padString(buffer.network, 2)
	copy(header[18:20], networkBytes)

	// Start time (10 bytes in SEED time format)
	seedTime := msw.timeToSeedTime(buffer.startTime)
	copy(header[20:30], seedTime)

	// Number of samples (2 bytes, big-endian)
	binary.BigEndian.PutUint16(header[30:32], uint16(numSamples))

	// Sample rate factor and multiplier (simplified)
	sampleRateInt := int16(buffer.sampleRate)
	binary.BigEndian.PutUint16(header[32:34], uint16(sampleRateInt))
	binary.BigEndian.PutUint16(header[34:36], 1) // Multiplier = 1

	// Flags
	header[36] = 0                 // Activity flags
	header[37] = 0                 // I/O flags
	header[38] = byte(QualityGood) // Data quality
	header[39] = 0                 // Number of blockettes

	// Time correction (4 bytes) - zero for now
	binary.BigEndian.PutUint32(header[40:44], 0)

	// Data offset (2 bytes) - points to start of data
	binary.BigEndian.PutUint16(header[44:46], MiniSeedHeaderSize)

	// First blockette offset (2 bytes) - zero if no blockettes
	binary.BigEndian.PutUint16(header[46:48], 0)

	// Write data (simple uncompressed format for now)
	dataStart := MiniSeedHeaderSize
	dataSize := msw.config.RecordLength - MiniSeedHeaderSize

	if msw.compression == CompressionNone {
		// Write raw int32 samples
		samplesPerRecord := dataSize / 4 // 4 bytes per int32
		if numSamples > samplesPerRecord {
			numSamples = samplesPerRecord
		}

		for i := 0; i < numSamples; i++ {
			offset := dataStart + i*4
			binary.BigEndian.PutUint32(record[offset:offset+4], uint32(buffer.data[i]))
		}
	}
	// TODO: Implement Steim compression

	return record, nil
}

// timeToSeedTime converts time.Time to SEED time format
func (msw *MiniSeedWriter) timeToSeedTime(t time.Time) []byte {
	seedTime := make([]byte, 10)

	// Year (2 bytes)
	binary.BigEndian.PutUint16(seedTime[0:2], uint16(t.Year()))

	// Day of year (2 bytes)
	binary.BigEndian.PutUint16(seedTime[2:4], uint16(t.YearDay()))

	// Hour (1 byte)
	seedTime[4] = byte(t.Hour())

	// Minute (1 byte)
	seedTime[5] = byte(t.Minute())

	// Second (1 byte)
	seedTime[6] = byte(t.Second())

	// Reserved (1 byte)
	seedTime[7] = 0

	// Fractional seconds (2 bytes, in units of 0.0001 seconds)
	fractional := uint16(t.Nanosecond() / 100000) // Convert to 0.0001 second units
	binary.BigEndian.PutUint16(seedTime[8:10], fractional)

	return seedTime
}

// shouldProcessChannel checks if this channel should be processed
func (msw *MiniSeedWriter) shouldProcessChannel(channel string) bool {
	if len(msw.channels) == 0 {
		return true // Process all channels if none specified
	}

	for _, ch := range msw.channels {
		if ch == "all" || ch == channel {
			return true
		}
		// Support wildcard matching
		if strings.HasSuffix(ch, "*") {
			prefix := strings.TrimSuffix(ch, "*")
			if strings.HasPrefix(channel, prefix) {
				return true
			}
		}
	}

	return false
}

// padString pads a string to the specified length with spaces
func padString(s string, length int) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = ' '
	}

	if len(s) > length {
		s = s[:length]
	}

	copy(result, []byte(s))
	return result
}

// isPowerOfTwo checks if a number is a power of 2
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
