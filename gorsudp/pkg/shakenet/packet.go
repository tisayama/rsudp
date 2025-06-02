// Package shakenet provides types and utilities for Raspberry Shake UDP packet processing
package shakenet

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UDPPacket represents a single UDP packet from Raspberry Shake
// Format: "{'CHANNEL', TIMESTAMP, DATA1, DATA2, ...}"
// Example: "{'EHZ', 1582315130.292, 14168, 14927, 16112, ...}"
type UDPPacket struct {
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
	Data      []int32   `json:"data"`
	Raw       []byte    `json:"-"` // Original raw packet data
}

// ParseUDPPacket parses a raw UDP packet byte slice into a UDPPacket struct
func ParseUDPPacket(raw []byte) (*UDPPacket, error) {
	packet := &UDPPacket{Raw: raw}

	// Convert to string and clean up
	str := strings.TrimSpace(string(raw))
	if len(str) == 0 {
		return nil, fmt.Errorf("empty packet")
	}

	// Remove outer braces
	if !strings.HasPrefix(str, "{") || !strings.HasSuffix(str, "}") {
		return nil, fmt.Errorf("invalid packet format: missing braces")
	}
	str = str[1 : len(str)-1]

	// Remove quotes around the whole content if present
	if strings.HasPrefix(str, "'") && strings.HasSuffix(str, "'") {
		str = str[1 : len(str)-1]
	}

	// Split by comma
	parts := strings.Split(str, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid packet format: insufficient parts")
	}

	// Parse channel (first part)
	channel := strings.Trim(strings.TrimSpace(parts[0]), "'\"")
	if channel == "" {
		return nil, fmt.Errorf("invalid channel")
	}
	packet.Channel = channel

	// Parse timestamp (second part)
	timestampStr := strings.TrimSpace(parts[1])
	timestampFloat, err := strconv.ParseFloat(timestampStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %v", err)
	}

	// Convert Unix timestamp to time.Time with microsecond precision
	sec := int64(timestampFloat)
	nsec := int64((timestampFloat - float64(sec)) * 1e9)
	packet.Timestamp = time.Unix(sec, nsec).UTC()

	// Parse data values (remaining parts)
	data := make([]int32, 0, len(parts)-2)
	for i := 2; i < len(parts); i++ {
		valueStr := strings.TrimSpace(parts[i])
		if valueStr == "" {
			continue
		}

		value, err := strconv.ParseInt(valueStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid data value at index %d: %v", i-2, err)
		}
		data = append(data, int32(value))
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no data values found")
	}

	packet.Data = data
	return packet, nil
}

// GetChannel returns the channel name
func (p *UDPPacket) GetChannel() string {
	return p.Channel
}

// GetTimestamp returns the timestamp as Unix float64 (for compatibility)
func (p *UDPPacket) GetTimestamp() float64 {
	return float64(p.Timestamp.Unix()) + float64(p.Timestamp.Nanosecond())/1e9
}

// GetData returns the data slice
func (p *UDPPacket) GetData() []int32 {
	return p.Data
}

// String returns a string representation of the packet
func (p *UDPPacket) String() string {
	dataStr := make([]string, len(p.Data))
	for i, d := range p.Data {
		dataStr[i] = fmt.Sprintf("%d", d)
	}

	return fmt.Sprintf("{'%s', %.3f, %s}",
		p.Channel,
		p.GetTimestamp(),
		strings.Join(dataStr, ", "))
}

// ToJSON converts the packet to JSON format
func (p *UDPPacket) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// IsValidChannel checks if the channel name is valid
func (p *UDPPacket) IsValidChannel() bool {
	validChannels := []string{
		"EHZ", "EHN", "EHE", // Geophone channels (velocity)
		"ENZ", "ENN", "ENE", // Accelerometer channels
		"SHZ", "SHN", "SHE", // Geophone channels (alternative naming)
		"HDF", // Pressure transducer
	}

	for _, valid := range validChannels {
		if p.Channel == valid {
			return true
		}
	}
	return false
}

// SampleRate returns the estimated sample rate based on data length
// Assumes standard Raspberry Shake timing
func (p *UDPPacket) SampleRate() float64 {
	switch len(p.Data) {
	case 25:
		return 100.0 // 100 Hz, 25 samples per 250ms packet
	case 50:
		return 50.0 // 50 Hz, 50 samples per 1000ms packet
	default:
		// Estimate based on common rates
		if len(p.Data) <= 30 {
			return 100.0
		}
		return 50.0
	}
}
