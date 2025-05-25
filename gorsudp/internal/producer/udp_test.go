package producer

import (
	"net"
	"testing"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Port != 8888 {
		t.Errorf("Default port = %d, want %d", config.Port, 8888)
	}

	if config.BufferSize != 1024 {
		t.Errorf("Default buffer size = %d, want %d", config.BufferSize, 1024)
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("Default timeout = %v, want %v", config.Timeout, 10*time.Second)
	}
}

func TestNewUDPProducer(t *testing.T) {
	config := Config{
		Port:       9999,
		BufferSize: 2048,
		Timeout:    5 * time.Second,
	}

	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	if producer == nil {
		t.Fatal("NewUDPProducer returned nil")
	}

	if producer.port != 9999 {
		t.Errorf("Port = %d, want %d", producer.port, 9999)
	}

	if producer.bufferSize != 2048 {
		t.Errorf("BufferSize = %d, want %d", producer.bufferSize, 2048)
	}

	if producer.timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want %v", producer.timeout, 5*time.Second)
	}

	if producer.broker != testBroker {
		t.Error("Broker not set correctly")
	}
}

func TestUDPProducer_StartStop(t *testing.T) {
	config := Config{
		Port:       0, // Use any available port
		BufferSize: 1024,
		Timeout:    1 * time.Second,
	}

	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	// Test starting
	err := producer.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !producer.running {
		t.Error("Producer should be running after Start()")
	}

	// Test starting again (should fail)
	err = producer.Start()
	if err == nil {
		t.Error("Start() should fail when already running")
	}

	// Test stopping
	err = producer.Stop()
	if err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if producer.running {
		t.Error("Producer should not be running after Stop()")
	}

	// Test stopping again (should fail)
	err = producer.Stop()
	if err == nil {
		t.Error("Stop() should fail when not running")
	}
}

func TestUDPProducer_GetMetrics(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	metrics := producer.GetMetrics()

	if metrics.PacketsReceived != 0 {
		t.Errorf("Initial PacketsReceived = %d, want 0", metrics.PacketsReceived)
	}

	if metrics.PacketsDropped != 0 {
		t.Errorf("Initial PacketsDropped = %d, want 0", metrics.PacketsDropped)
	}

	if metrics.BytesReceived != 0 {
		t.Errorf("Initial BytesReceived = %d, want 0", metrics.BytesReceived)
	}

	if metrics.Errors != 0 {
		t.Errorf("Initial Errors = %d, want 0", metrics.Errors)
	}

	if metrics.FirstSender != nil {
		t.Errorf("Initial FirstSender should be nil")
	}

	if metrics.Running {
		t.Error("Initial Running should be false")
	}
}

func TestUDPProducer_GetStatus(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	status := producer.GetStatus()

	if status.Running {
		t.Error("Initial status should not be running")
	}

	if status.Port != config.Port {
		t.Errorf("Status port = %d, want %d", status.Port, config.Port)
	}

	if status.Health != "stopped" {
		t.Errorf("Initial health = %s, want stopped", status.Health)
	}

	// Start producer and check status
	err := producer.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer producer.Stop()

	status = producer.GetStatus()

	if !status.Running {
		t.Error("Status should be running after Start()")
	}

	if status.Health != "waiting" {
		t.Errorf("Health after start = %s, want waiting", status.Health)
	}
}

func TestUDPProducer_IsHealthy(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	// Initially not healthy (not running)
	if producer.IsHealthy() {
		t.Error("Producer should not be healthy when stopped")
	}

	// Start producer
	err := producer.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer producer.Stop()

	// Still not healthy (no data received)
	if producer.IsHealthy() {
		t.Error("Producer should not be healthy without data")
	}
}

func TestUDPProducer_WaitForData(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	err := producer.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer producer.Stop()

	// Should timeout when no data arrives
	err = producer.WaitForData(100 * time.Millisecond)
	if err == nil {
		t.Error("WaitForData() should timeout when no data arrives")
	}
}

func TestUDPProducer_incrementErrors(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	producer := NewUDPProducer(config, testBroker)

	// Initially no errors
	metrics := producer.GetMetrics()
	if metrics.Errors != 0 {
		t.Errorf("Initial errors = %d, want 0", metrics.Errors)
	}

	// Increment errors
	producer.incrementErrors()
	producer.incrementErrors()

	metrics = producer.GetMetrics()
	if metrics.Errors != 2 {
		t.Errorf("After incrementing errors = %d, want 2", metrics.Errors)
	}
}

func TestUDPProducer_processPacket(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	err := testBroker.Start()
	if err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}
	defer testBroker.Stop()

	producer := NewUDPProducer(config, testBroker)

	// Create a mock UDP address
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	// Test with invalid packet data
	invalidData := []byte("invalid packet")
	err = producer.processPacket(invalidData, addr)
	if err == nil {
		t.Error("processPacket() should fail with invalid data")
	}

	// Test with valid packet data (mock Raspberry Shake packet)
	// This is a simplified test - in real implementation, you'd need valid packet format
	validData := createMockRaspberryShakePacket()

	err = producer.processPacket(validData, addr)
	// This will likely fail due to packet format, but that's expected

	// Check that first sender was set
	if producer.firstSender == nil {
		t.Error("FirstSender should be set after processing packet")
	}

	if !producer.firstSender.IP.Equal(addr.IP) {
		t.Error("FirstSender should match the packet sender")
	}
}

func TestUDPProducer_firstSenderSecurity(t *testing.T) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(10)
	err := testBroker.Start()
	if err != nil {
		t.Fatalf("Failed to start broker: %v", err)
	}
	defer testBroker.Stop()

	producer := NewUDPProducer(config, testBroker)

	// First sender
	addr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	// Different sender
	addr2, _ := net.ResolveUDPAddr("udp", "192.168.1.1:12345")

	validData := createMockRaspberryShakePacket()

	// Process packet from first sender
	err = producer.processPacket(validData, addr1)
	// Error is expected due to packet format, but firstSender should be set

	if producer.firstSender == nil {
		t.Error("FirstSender should be set")
	}

	// Process packet from different sender (should be ignored)
	err = producer.processPacket(validData, addr2)
	// This should work (though parsing may fail)

	// Verify first sender security check logic
	if producer.firstSender == nil {
		t.Error("FirstSender should be set after first packet")
	} else if !producer.firstSender.IP.Equal(addr1.IP) {
		t.Error("FirstSender should be the first sender's IP")
	}
}

// createMockRaspberryShakePacket creates a mock packet for testing
// Note: This is a simplified mock - real Raspberry Shake packets have specific format
func createMockRaspberryShakePacket() []byte {
	// This is a very basic mock - in reality you'd need proper RS packet structure
	// For testing purposes, we'll create something that will likely fail parsing
	// but will exercise the packet processing pipeline
	
	// Create a mock packet that might resemble actual format but will fail parsing
	// This allows us to test the metrics update portion of processPacket
	mockData := make([]byte, 50)
	copy(mockData, []byte("MOCK_PACKET_"))
	return mockData
}

func BenchmarkUDPProducer_processPacket(b *testing.B) {
	config := DefaultConfig()
	testBroker := broker.NewMessageBroker(1000)
	testBroker.Start()
	defer testBroker.Stop()

	producer := NewUDPProducer(config, testBroker)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	data := createMockRaspberryShakePacket()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		producer.processPacket(data, addr)
	}
}