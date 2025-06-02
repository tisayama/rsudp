package stream

import (
	"testing"
	"time"

	"github.com/tisayama/gorsudp/pkg/shakenet"
)

func TestNewTrace(t *testing.T) {
	station := "TEST"
	network := "AM"
	channel := "HZ"
	location := "00"
	sampleRate := 100.0
	startTime := time.Now()

	trace := NewTrace(station, network, channel, location, sampleRate, startTime)

	if trace == nil {
		t.Fatal("NewTrace returned nil")
	}

	if trace.Station != station {
		t.Errorf("Station = %s, want %s", trace.Station, station)
	}

	if trace.Network != network {
		t.Errorf("Network = %s, want %s", trace.Network, network)
	}

	if trace.Channel != channel {
		t.Errorf("Channel = %s, want %s", trace.Channel, channel)
	}

	if trace.Location != location {
		t.Errorf("Location = %s, want %s", trace.Location, location)
	}

	if trace.SampleRate != sampleRate {
		t.Errorf("SampleRate = %f, want %f", trace.SampleRate, sampleRate)
	}

	if trace.Delta != 0.01 {
		t.Errorf("Delta = %f, want 0.01", trace.Delta)
	}

	if !trace.StartTime.Equal(startTime) {
		t.Errorf("StartTime = %v, want %v", trace.StartTime, startTime)
	}

	if !trace.EndTime.Equal(startTime) {
		t.Errorf("EndTime = %v, want %v (should equal StartTime initially)", trace.EndTime, startTime)
	}

	if trace.NumSamples != 0 {
		t.Errorf("NumSamples = %d, want 0", trace.NumSamples)
	}

	if len(trace.Data) != 0 {
		t.Errorf("Data length = %d, want 0", len(trace.Data))
	}

	if cap(trace.Data) != 2500 {
		t.Errorf("Data capacity = %d, want 2500", cap(trace.Data))
	}
}

func TestTrace_AppendData(t *testing.T) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())

	// Test appending data
	data := []int32{100, 200, 300, 400, 500}
	timestamp := time.Now()

	trace.AppendData(data, timestamp)

	if trace.NumSamples != len(data) {
		t.Errorf("NumSamples = %d, want %d", trace.NumSamples, len(data))
	}

	if len(trace.Data) != len(data) {
		t.Errorf("Data length = %d, want %d", len(trace.Data), len(data))
	}

	// Check data values
	for i, expected := range []float64{100, 200, 300, 400, 500} {
		if trace.Data[i] != expected {
			t.Errorf("Data[%d] = %f, want %f", i, trace.Data[i], expected)
		}
	}

	// EndTime should be updated
	expectedEndTime := timestamp.Add(time.Duration(len(data)) * time.Duration(trace.Delta*float64(time.Second)))
	if !trace.EndTime.Equal(expectedEndTime) {
		t.Errorf("EndTime not updated correctly: got %v, want %v", trace.EndTime, expectedEndTime)
	}

	// Append more data
	moreData := []int32{600, 700}
	trace.AppendData(moreData, timestamp.Add(5*time.Duration(trace.Delta*float64(time.Second))))

	if trace.NumSamples != len(data)+len(moreData) {
		t.Errorf("NumSamples after second append = %d, want %d", trace.NumSamples, len(data)+len(moreData))
	}
}

func TestTrace_GetData(t *testing.T) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())

	// Add some data
	data := []int32{100, 200, 300, 400, 500}
	trace.AppendData(data, time.Now())

	// Test getting all data
	result := trace.GetData()
	if len(result) != len(data) {
		t.Errorf("GetData length = %d, want %d", len(result), len(data))
	}

	for i, expected := range []float64{100, 200, 300, 400, 500} {
		if result[i] != expected {
			t.Errorf("GetData[%d] = %f, want %f", i, result[i], expected)
		}
	}

	// Modifying returned slice should not affect original
	result[0] = 999
	if trace.Data[0] == 999 {
		t.Error("GetData should return a copy, not the original slice")
	}
}

func TestTrace_Slice(t *testing.T) {
	startTime := time.Now()
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, startTime)

	// Add data spanning 10 samples (0.1 seconds at 100Hz)
	data := []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	trace.AppendData(data, startTime)

	// Test slicing middle portion
	sliceStart := startTime.Add(20 * time.Millisecond) // 2 samples in
	sliceEnd := startTime.Add(70 * time.Millisecond)   // 7 samples in

	sliced := trace.Slice(sliceStart, sliceEnd)

	if sliced == nil {
		t.Fatal("Slice returned nil")
	}

	// Should get samples from index 2 to 7: [2, 3, 4, 5, 6]
	expectedLen := 5
	if sliced.NumSamples != expectedLen {
		t.Errorf("Sliced trace length = %d, want %d", sliced.NumSamples, expectedLen)
	}

	// Test slicing outside range
	futureStart := startTime.Add(1 * time.Hour)
	futureEnd := startTime.Add(2 * time.Hour)

	emptySlice := trace.Slice(futureStart, futureEnd)
	if emptySlice.NumSamples != 0 {
		t.Errorf("Slice outside range should be empty, got %d samples", emptySlice.NumSamples)
	}
}

func TestTrace_Copy(t *testing.T) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())

	// Add some data
	data := []int32{100, 200, 300, 400, 500}
	trace.AppendData(data, time.Now())

	// Test copying
	copied := trace.Copy()

	if copied == nil {
		t.Fatal("Copy returned nil")
	}

	// Check that basic properties are copied
	if copied.Station != trace.Station {
		t.Errorf("Copied station = %s, want %s", copied.Station, trace.Station)
	}

	if copied.Channel != trace.Channel {
		t.Errorf("Copied channel = %s, want %s", copied.Channel, trace.Channel)
	}

	if copied.NumSamples != trace.NumSamples {
		t.Errorf("Copied NumSamples = %d, want %d", copied.NumSamples, trace.NumSamples)
	}

	// Check that data is copied
	if len(copied.Data) != len(trace.Data) {
		t.Errorf("Copied data length = %d, want %d", len(copied.Data), len(trace.Data))
	}

	for i := range trace.Data {
		if copied.Data[i] != trace.Data[i] {
			t.Errorf("Copied data[%d] = %f, want %f", i, copied.Data[i], trace.Data[i])
		}
	}

	// Check that modifying copy doesn't affect original
	copied.Data[0] = 999
	if trace.Data[0] == 999 {
		t.Error("Copy should be independent of original")
	}
}

func TestTrace_Stats(t *testing.T) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())

	// Test stats with no data
	stats := trace.Stats()
	if stats.Samples != 0 {
		t.Errorf("Stats Samples with no data = %d, want 0", stats.Samples)
	}

	// Add some test data
	data := []int32{-100, 0, 100, 200, 300}
	trace.AppendData(data, time.Now())

	stats = trace.Stats()

	if stats.Samples != 5 {
		t.Errorf("Stats Samples = %d, want 5", stats.Samples)
	}

	if stats.Min != -100 {
		t.Errorf("Stats Min = %f, want -100", stats.Min)
	}

	if stats.Max != 300 {
		t.Errorf("Stats Max = %f, want 300", stats.Max)
	}

	expectedMean := (float64(-100 + 0 + 100 + 200 + 300)) / 5
	if stats.Mean != expectedMean {
		t.Errorf("Stats Mean = %f, want %f", stats.Mean, expectedMean)
	}
}

func TestNewStream(t *testing.T) {
	stream := NewStream()

	if stream == nil {
		t.Fatal("NewStream returned nil")
	}

	if stream.Traces == nil {
		t.Error("Stream Traces map is nil")
	}

	if len(stream.Traces) != 0 {
		t.Errorf("Stream should start with 0 traces, got %d", len(stream.Traces))
	}
}

func TestStream_UpdateFromPacket(t *testing.T) {
	stream := NewStream()

	// Create a test packet
	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	station := "TEST"
	network := "AM"

	// Test first update
	err := stream.UpdateFromPacket(packet, station, network)
	if err != nil {
		t.Fatalf("UpdateFromPacket failed: %v", err)
	}

	// Check that trace was created
	traceCount := stream.Count()
	if traceCount != 1 {
		t.Errorf("Expected 1 trace, got %d", traceCount)
	}

	traceID := "AM.TEST.00.EHZ"
	trace, exists := stream.GetTrace(traceID)
	if !exists {
		t.Errorf("Trace %s not found", traceID)
	}

	if trace.NumSamples != 3 {
		t.Errorf("Trace NumSamples = %d, want 3", trace.NumSamples)
	}

	// Test second update to same trace
	packet2 := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: packet.Timestamp.Add(30 * time.Millisecond), // 3 samples later at 100Hz
		Data:      []int32{400, 500},
	}

	err = stream.UpdateFromPacket(packet2, station, network)
	if err != nil {
		t.Fatalf("Second UpdateFromPacket failed: %v", err)
	}

	// Should still be 1 trace, but with more data
	traceCount = stream.Count()
	if traceCount != 1 {
		t.Errorf("Expected 1 trace after second update, got %d", traceCount)
	}

	trace, _ = stream.GetTrace(traceID)
	if trace.NumSamples != 5 {
		t.Errorf("Trace NumSamples after second update = %d, want 5", trace.NumSamples)
	}

	// Test adding different channel
	packet3 := &shakenet.UDPPacket{
		Channel:   "EHN",
		Timestamp: time.Now(),
		Data:      []int32{1000, 2000},
	}

	err = stream.UpdateFromPacket(packet3, station, network)
	if err != nil {
		t.Fatalf("Third UpdateFromPacket failed: %v", err)
	}

	// Should now have 2 traces
	traceCount = stream.Count()
	if traceCount != 2 {
		t.Errorf("Expected 2 traces after adding different channel, got %d", traceCount)
	}

	// Check new trace
	traceID2 := "AM.TEST.00.EHN"
	trace2, exists := stream.GetTrace(traceID2)
	if !exists {
		t.Errorf("Trace %s not found", traceID2)
	}

	if trace2.NumSamples != 2 {
		t.Errorf("New trace NumSamples = %d, want 2", trace2.NumSamples)
	}
}

func TestStream_GetTrace(t *testing.T) {
	stream := NewStream()

	// Test getting non-existent trace
	trace, exists := stream.GetTrace("nonexistent")
	if exists || trace != nil {
		t.Error("GetTrace should return false and nil for non-existent trace")
	}

	// Add a trace via packet
	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	stream.UpdateFromPacket(packet, "TEST", "AM")

	// Test getting existing trace
	traceID := "AM.TEST.00.EHZ"
	trace, exists = stream.GetTrace(traceID)
	if !exists {
		t.Error("GetTrace should return trace for existing ID")
	}

	if trace.Channel != "EHZ" {
		t.Errorf("Trace channel = %s, want EHZ", trace.Channel)
	}
}

func TestStream_GetChannels(t *testing.T) {
	stream := NewStream()

	// Test with no traces
	channels := stream.GetChannels()
	if len(channels) != 0 {
		t.Errorf("Expected 0 channels, got %d", len(channels))
	}

	// Add some traces
	packet1 := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100},
	}

	packet2 := &shakenet.UDPPacket{
		Channel:   "EHN",
		Timestamp: time.Now(),
		Data:      []int32{200},
	}

	stream.UpdateFromPacket(packet1, "TEST", "AM")
	stream.UpdateFromPacket(packet2, "TEST", "AM")

	channels = stream.GetChannels()
	if len(channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(channels))
	}

	// Check that we got the expected channels
	channelSet := make(map[string]bool)
	for _, ch := range channels {
		channelSet[ch] = true
	}

	if !channelSet["EHZ"] {
		t.Error("Expected EHZ channel")
	}

	if !channelSet["EHN"] {
		t.Error("Expected EHN channel")
	}
}

func TestTrace_ID(t *testing.T) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())

	expectedID := "AM.TEST.00.HZ"
	actualID := trace.ID()

	if actualID != expectedID {
		t.Errorf("Trace ID = %s, want %s", actualID, expectedID)
	}
}

func TestStream_Copy(t *testing.T) {
	stream := NewStream()

	// Add some traces
	packet1 := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200},
	}

	packet2 := &shakenet.UDPPacket{
		Channel:   "EHN",
		Timestamp: time.Now(),
		Data:      []int32{300, 400},
	}

	stream.UpdateFromPacket(packet1, "TEST", "AM")
	stream.UpdateFromPacket(packet2, "TEST", "AM")

	// Test copying
	copied := stream.Copy()

	if copied == nil {
		t.Fatal("Copy returned nil")
	}

	if copied.Count() != stream.Count() {
		t.Errorf("Copied stream count = %d, want %d", copied.Count(), stream.Count())
	}

	// Check that traces are independently copied
	originalChannels := stream.GetChannels()
	copiedChannels := copied.GetChannels()

	if len(copiedChannels) != len(originalChannels) {
		t.Errorf("Copied channels count = %d, want %d", len(copiedChannels), len(originalChannels))
	}

	// Verify independence - modify copy shouldn't affect original
	if trace, exists := copied.GetTrace("AM.TEST.00.EHZ"); exists {
		trace.Data[0] = 999
	}

	if trace, exists := stream.GetTrace("AM.TEST.00.EHZ"); exists {
		if trace.Data[0] == 999 {
			t.Error("Original stream should not be affected by copy modification")
		}
	}
}

func BenchmarkTrace_AppendData(b *testing.B) {
	trace := NewTrace("TEST", "AM", "HZ", "00", 100.0, time.Now())
	data := []int32{100, 200, 300, 400, 500}
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trace.AppendData(data, timestamp)
	}
}

func BenchmarkStream_UpdateFromPacket(b *testing.B) {
	stream := NewStream()
	packet := &shakenet.UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Now(),
		Data:      []int32{100, 200, 300},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream.UpdateFromPacket(packet, "TEST", "AM")
	}
}
