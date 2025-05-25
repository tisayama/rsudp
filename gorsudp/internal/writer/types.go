// Package writer provides data storage functionality for seismic data
package writer

import (
	"os"
	"sync"
	"time"
)

// CompressionType represents the compression type for data storage
type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionSteim1
	CompressionSteim2
)

func (ct CompressionType) String() string {
	switch ct {
	case CompressionNone:
		return "none"
	case CompressionSteim1:
		return "steim1"
	case CompressionSteim2:
		return "steim2"
	default:
		return "unknown"
	}
}

// MiniSeedWriter manages MiniSEED data writing
type MiniSeedWriter struct {
	outputDir    string
	compression  CompressionType
	channels     []string
	files        map[string]*os.File
	buffers      map[string]*MiniSeedBuffer
	config       WriterConfig
	mutex        sync.RWMutex
}

// MiniSeedBuffer manages buffered data for MiniSEED records
type MiniSeedBuffer struct {
	data       []int32
	startTime  time.Time
	sampleRate float64
	maxSamples int
	channel    string
	network    string
	station    string
	location   string
	mutex      sync.Mutex
}

// WriterConfig contains configuration for the writer
type WriterConfig struct {
	OutputDir     string            `json:"output_dir"`
	Channels      []string          `json:"channels"`
	Compression   CompressionType   `json:"compression"`
	RecordLength  int               `json:"record_length"`  // MiniSEED record length (512, 1024, 2048, 4096)
	BufferSize    int               `json:"buffer_size"`    // Number of samples to buffer before writing
	FlushInterval time.Duration     `json:"flush_interval"` // Time interval to force buffer flush
	FileFormat    string            `json:"file_format"`    // File naming format
	CreateDirs    bool              `json:"create_dirs"`    // Create date-based directory structure
}

// MiniSeedRecord represents a MiniSEED record
type MiniSeedRecord struct {
	Header MiniSeedHeader `json:"header"`
	Data   []byte         `json:"data"`
}

// MiniSeedHeader represents the fixed header of a MiniSEED record
type MiniSeedHeader struct {
	SequenceNumber    [6]byte  `json:"sequence_number"`
	DataHeaderType    byte     `json:"data_header_type"`
	ReservedByte      byte     `json:"reserved_byte"`
	StationCode       [5]byte  `json:"station_code"`
	LocationCode      [2]byte  `json:"location_code"`
	ChannelCode       [3]byte  `json:"channel_code"`
	NetworkCode       [2]byte  `json:"network_code"`
	StartTime         [10]byte `json:"start_time"`
	NumberOfSamples   [2]byte  `json:"number_of_samples"`
	SampleRateFactor  [2]byte  `json:"sample_rate_factor"`
	SampleRateMultiplier [2]byte `json:"sample_rate_multiplier"`
	ActivityFlags     byte     `json:"activity_flags"`
	IOFlags           byte     `json:"io_flags"`
	DataQualityFlags  byte     `json:"data_quality_flags"`
	NumberOfBlockettes byte    `json:"number_of_blockettes"`
	TimeCorrection    [4]byte  `json:"time_correction"`
	DataOffset        [2]byte  `json:"data_offset"`
	FirstBlockette    [2]byte  `json:"first_blockette"`
}

// DataQuality represents MiniSEED data quality indicators
type DataQuality byte

const (
	QualityGood DataQuality = 'D' // Good data
	QualityRaw  DataQuality = 'R' // Raw data
	QualityTest DataQuality = 'T' // Test data
	QualityMisc DataQuality = 'M' // Miscellaneous
)

// Constants for MiniSEED format
const (
	MiniSeedHeaderSize    = 48  // Fixed header size
	MiniSeedRecordSize512 = 512 // Standard record sizes
	MiniSeedRecordSize1024 = 1024
	MiniSeedRecordSize2048 = 2048
	MiniSeedRecordSize4096 = 4096
	
	DefaultRecordLength = MiniSeedRecordSize512
	DefaultBufferSize   = 100 // samples
	DefaultFlushInterval = 60 * time.Second
)

// FileNamingFormat represents different file naming conventions
type FileNamingFormat string

const (
	FormatDaily    FileNamingFormat = "daily"    // YYYY.DDD.NN.SSS.LL.CCC.D.mseed
	FormatHourly   FileNamingFormat = "hourly"   // YYYY.DDD.HH.NN.SSS.LL.CCC.D.mseed
	FormatContinuous FileNamingFormat = "continuous" // NN.SSS.LL.CCC.D.mseed
)

// WriteStats tracks writing statistics
type WriteStats struct {
	RecordsWritten   int64     `json:"records_written"`
	SamplesWritten   int64     `json:"samples_written"`
	BytesWritten     int64     `json:"bytes_written"`
	FilesCreated     int       `json:"files_created"`
	LastWrite        time.Time `json:"last_write"`
	ErrorCount       int64     `json:"error_count"`
	StartTime        time.Time `json:"start_time"`
	mutex            sync.RWMutex
}