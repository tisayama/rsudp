package shakenet

import (
	"testing"
	"time"
)

func TestParseUDPPacket(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    *UDPPacket
	}{
		{
			name:  "valid EHZ packet",
			input: "{'EHZ', 1582315130.292, 14168, 14927, 16112, 15012, 14598}",
			expected: &UDPPacket{
				Channel: "EHZ",
				Data:    []int32{14168, 14927, 16112, 15012, 14598},
			},
		},
		{
			name:  "valid ENZ packet",
			input: "{'ENZ', 1582315130.292, -1024, 0, 1024, 2048}",
			expected: &UDPPacket{
				Channel: "ENZ",
				Data:    []int32{-1024, 0, 1024, 2048},
			},
		},
		{
			name:        "empty packet",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid format - no braces",
			input:       "EHZ, 1582315130.292, 14168",
			expectError: true,
		},
		{
			name:        "invalid format - insufficient parts",
			input:       "{'EHZ', 1582315130.292}",
			expectError: true,
		},
		{
			name:        "invalid timestamp",
			input:       "{'EHZ', invalid, 14168, 14927}",
			expectError: true,
		},
		{
			name:        "invalid data value",
			input:       "{'EHZ', 1582315130.292, invalid, 14927}",
			expectError: true,
		},
		{
			name:        "no data values",
			input:       "{'EHZ', 1582315130.292}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet, err := ParseUDPPacket([]byte(tt.input))
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if packet.Channel != tt.expected.Channel {
				t.Errorf("expected channel %s, got %s", tt.expected.Channel, packet.Channel)
			}
			
			if len(packet.Data) != len(tt.expected.Data) {
				t.Errorf("expected %d data points, got %d", len(tt.expected.Data), len(packet.Data))
				return
			}
			
			for i, expected := range tt.expected.Data {
				if packet.Data[i] != expected {
					t.Errorf("data[%d]: expected %d, got %d", i, expected, packet.Data[i])
				}
			}
			
			// Verify timestamp is parsed correctly (allow small precision differences)
			expectedTime := time.Unix(1582315130, 292000000).UTC()
			timeDiff := packet.Timestamp.Sub(expectedTime)
			if timeDiff < -time.Microsecond || timeDiff > time.Microsecond {
				t.Errorf("expected timestamp %v, got %v (diff: %v)", expectedTime, packet.Timestamp, timeDiff)
			}
		})
	}
}

func TestUDPPacketGetTimestamp(t *testing.T) {
	packet := &UDPPacket{
		Timestamp: time.Unix(1582315130, 292000000).UTC(),
	}
	
	expected := 1582315130.292
	actual := packet.GetTimestamp()
	
	if actual != expected {
		t.Errorf("expected timestamp %f, got %f", expected, actual)
	}
}

func TestUDPPacketIsValidChannel(t *testing.T) {
	tests := []struct {
		channel string
		valid   bool
	}{
		{"EHZ", true},
		{"EHN", true},
		{"EHE", true},
		{"ENZ", true},
		{"ENN", true},
		{"ENE", true},
		{"SHZ", true},
		{"SHN", true},
		{"SHE", true},
		{"HDF", true},
		{"INVALID", false},
		{"", false},
	}
	
	for _, tt := range tests {
		packet := &UDPPacket{Channel: tt.channel}
		if packet.IsValidChannel() != tt.valid {
			t.Errorf("channel %s: expected valid=%t, got %t", tt.channel, tt.valid, packet.IsValidChannel())
		}
	}
}

func TestUDPPacketSampleRate(t *testing.T) {
	tests := []struct {
		dataLength int
		expected   float64
	}{
		{25, 100.0},
		{50, 50.0},
		{10, 100.0}, // Less than 30, defaults to 100 Hz
		{100, 50.0}, // More than 30, defaults to 50 Hz
	}
	
	for _, tt := range tests {
		data := make([]int32, tt.dataLength)
		packet := &UDPPacket{Data: data}
		
		if packet.SampleRate() != tt.expected {
			t.Errorf("data length %d: expected sample rate %f, got %f", 
				tt.dataLength, tt.expected, packet.SampleRate())
		}
	}
}

func TestUDPPacketString(t *testing.T) {
	packet := &UDPPacket{
		Channel:   "EHZ",
		Timestamp: time.Unix(1582315130, 292000000).UTC(),
		Data:      []int32{14168, 14927, 16112},
	}
	
	expected := "{'EHZ', 1582315130.292, 14168, 14927, 16112}"
	actual := packet.String()
	
	if actual != expected {
		t.Errorf("expected string %s, got %s", expected, actual)
	}
}

func BenchmarkParseUDPPacket(b *testing.B) {
	input := []byte("{'EHZ', 1582315130.292, 14168, 14927, 16112, 15012, 14598, 13201, 12987, 13456, 14123, 15234, 16789, 15432, 14876, 13654, 12345, 11987, 12876, 13987, 15123, 16234, 17345, 16876, 15987}")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseUDPPacket(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}