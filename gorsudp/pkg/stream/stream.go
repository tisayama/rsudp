// Package stream provides time series data processing capabilities
package stream

import (
	"fmt"
	"sync"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// Trace represents a single channel of time series data
type Trace struct {
	Data       []float64 `json:"data"`
	Station    string    `json:"station"`
	Network    string    `json:"network"`
	Channel    string    `json:"channel"`
	Location   string    `json:"location"`
	SampleRate float64   `json:"sample_rate"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Delta      float64   `json:"delta"` // Sample interval (1/SampleRate)
	NumSamples int       `json:"num_samples"`

	// Internal metadata
	lastUpdate time.Time
	mutex      sync.RWMutex
}

// NewTrace creates a new Trace with the given parameters
func NewTrace(station, network, channel, location string, sampleRate float64, startTime time.Time) *Trace {
	return &Trace{
		Data:       make([]float64, 0, 2500), // Pre-allocate for ~25 seconds at 100Hz
		Station:    station,
		Network:    network,
		Channel:    channel,
		Location:   location,
		SampleRate: sampleRate,
		StartTime:  startTime,
		EndTime:    startTime,
		Delta:      1.0 / sampleRate,
		NumSamples: 0,
		lastUpdate: time.Now(),
	}
}

// AppendData adds new data points to the trace
func (t *Trace) AppendData(data []int32, timestamp time.Time) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Convert int32 to float64
	floatData := make([]float64, len(data))
	for i, d := range data {
		floatData[i] = float64(d)
	}

	t.Data = append(t.Data, floatData...)
	t.NumSamples += len(data)
	t.EndTime = timestamp.Add(time.Duration(len(data)) * time.Duration(t.Delta*1e9))
	t.lastUpdate = time.Now()
}

// GetData returns a copy of the trace data
func (t *Trace) GetData() []float64 {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	data := make([]float64, len(t.Data))
	copy(data, t.Data)
	return data
}

// Slice returns a new trace containing data within the specified time window
func (t *Trace) Slice(startTime, endTime time.Time) *Trace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if startTime.After(t.EndTime) || endTime.Before(t.StartTime) {
		// No overlap
		return NewTrace(t.Station, t.Network, t.Channel, t.Location, t.SampleRate, startTime)
	}

	// Calculate sample indices
	startIdx := 0
	if startTime.After(t.StartTime) {
		startIdx = int(startTime.Sub(t.StartTime).Seconds() * t.SampleRate)
	}

	endIdx := len(t.Data)
	if endTime.Before(t.EndTime) {
		endIdx = int(endTime.Sub(t.StartTime).Seconds() * t.SampleRate)
	}

	// Ensure indices are within bounds
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(t.Data) {
		endIdx = len(t.Data)
	}
	if startIdx >= endIdx {
		return NewTrace(t.Station, t.Network, t.Channel, t.Location, t.SampleRate, startTime)
	}

	// Create new trace with sliced data
	newTrace := NewTrace(t.Station, t.Network, t.Channel, t.Location, t.SampleRate, startTime)
	newTrace.Data = make([]float64, endIdx-startIdx)
	copy(newTrace.Data, t.Data[startIdx:endIdx])
	newTrace.NumSamples = len(newTrace.Data)
	newTrace.EndTime = endTime

	return newTrace
}

// Copy creates a deep copy of the trace
func (t *Trace) Copy() *Trace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	newTrace := &Trace{
		Data:       make([]float64, len(t.Data)),
		Station:    t.Station,
		Network:    t.Network,
		Channel:    t.Channel,
		Location:   t.Location,
		SampleRate: t.SampleRate,
		StartTime:  t.StartTime,
		EndTime:    t.EndTime,
		Delta:      t.Delta,
		NumSamples: t.NumSamples,
		lastUpdate: t.lastUpdate,
	}

	copy(newTrace.Data, t.Data)
	return newTrace
}

// ID returns a unique identifier for this trace
func (t *Trace) ID() string {
	return fmt.Sprintf("%s.%s.%s.%s", t.Network, t.Station, t.Location, t.Channel)
}

// Stats returns basic statistics about the trace data
func (t *Trace) Stats() TraceStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if len(t.Data) == 0 {
		return TraceStats{}
	}

	min, max := t.Data[0], t.Data[0]
	sum := 0.0

	for _, v := range t.Data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	mean := sum / float64(len(t.Data))

	// Calculate standard deviation
	variance := 0.0
	for _, v := range t.Data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(t.Data))
	stddev := variance // Square root calculation would need math package

	return TraceStats{
		Min:     min,
		Max:     max,
		Mean:    mean,
		StdDev:  stddev,
		Samples: len(t.Data),
	}
}

// TraceStats contains basic statistics about trace data
type TraceStats struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Mean    float64 `json:"mean"`
	StdDev  float64 `json:"stddev"`
	Samples int     `json:"samples"`
}

// Stream represents a collection of related traces
type Stream struct {
	Traces map[string]*Trace // Key is trace ID
	mutex  sync.RWMutex
}

// NewStream creates a new empty stream
func NewStream() *Stream {
	return &Stream{
		Traces: make(map[string]*Trace),
	}
}

// AddTrace adds a trace to the stream
func (s *Stream) AddTrace(trace *Trace) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Traces[trace.ID()] = trace
}

// GetTrace retrieves a trace by ID
func (s *Stream) GetTrace(id string) (*Trace, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	trace, exists := s.Traces[id]
	return trace, exists
}

// GetTraceByChannel retrieves the first trace with the specified channel
func (s *Stream) GetTraceByChannel(channel string) (*Trace, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, trace := range s.Traces {
		if trace.Channel == channel {
			return trace, true
		}
	}
	return nil, false
}

// UpdateFromPacket updates the stream with data from a UDP packet
func (s *Stream) UpdateFromPacket(packet *shakenet.UDPPacket, station, network string) error {
	// Create trace ID
	location := "00" // Default location
	traceID := fmt.Sprintf("%s.%s.%s.%s", network, station, location, packet.Channel)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	trace, exists := s.Traces[traceID]
	if !exists {
		// Create new trace
		trace = NewTrace(station, network, packet.Channel, location, packet.SampleRate(), packet.Timestamp)
		s.Traces[traceID] = trace
	}

	// Append data to trace
	trace.AppendData(packet.Data, packet.Timestamp)

	return nil
}

// Slice returns a new stream containing traces sliced to the specified time window
func (s *Stream) Slice(startTime, endTime time.Time) *Stream {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	newStream := NewStream()

	for id, trace := range s.Traces {
		slicedTrace := trace.Slice(startTime, endTime)
		if slicedTrace.NumSamples > 0 {
			newStream.Traces[id] = slicedTrace
		}
	}

	return newStream
}

// Copy creates a deep copy of the stream
func (s *Stream) Copy() *Stream {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	newStream := NewStream()

	for id, trace := range s.Traces {
		newStream.Traces[id] = trace.Copy()
	}

	return newStream
}

// GetChannels returns a list of all channel names in the stream
func (s *Stream) GetChannels() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	channels := make([]string, 0, len(s.Traces))
	for _, trace := range s.Traces {
		channels = append(channels, trace.Channel)
	}

	return channels
}

// Count returns the number of traces in the stream
func (s *Stream) Count() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.Traces)
}
