// Package producer provides UDP data reception and distribution functionality
package producer

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/pkg/shakenet"
)

// UDPProducer receives UDP packets from Raspberry Shake devices and distributes them via broker
type UDPProducer struct {
	port         int
	conn         *net.UDPConn
	broker       *broker.MessageBroker
	firstSender  *net.UDPAddr
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mutex        sync.RWMutex
	
	// Configuration
	bufferSize   int
	timeout      time.Duration
	
	// Metrics
	packetsReceived int64
	packetsDropped  int64
	bytesReceived   int64
	lastPacketTime  time.Time
	errors          int64
}

// Config holds configuration options for UDPProducer
type Config struct {
	Port       int           `json:"port"`
	BufferSize int           `json:"buffer_size"`
	Timeout    time.Duration `json:"timeout"`
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Port:       8888,
		BufferSize: 1024,
		Timeout:    10 * time.Second,
	}
}

// NewUDPProducer creates a new UDP producer
func NewUDPProducer(config Config, messageBroker *broker.MessageBroker) *UDPProducer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &UDPProducer{
		port:        config.Port,
		broker:      messageBroker,
		bufferSize:  config.BufferSize,
		timeout:     config.Timeout,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins listening for UDP packets
func (p *UDPProducer) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if p.running {
		return fmt.Errorf("producer is already running")
	}
	
	// Create UDP address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", p.port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}
	
	// Listen on UDP port
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port %d: %v", p.port, err)
	}
	
	p.conn = conn
	p.running = true
	
	// Set read timeout if specified
	if p.timeout > 0 {
		p.conn.SetReadDeadline(time.Now().Add(p.timeout))
	}
	
	// Start packet reception loop
	p.wg.Add(1)
	go p.receiveLoop()
	
	log.Printf("UDP Producer started listening on port %d", p.port)
	return nil
}

// Stop gracefully shuts down the producer
func (p *UDPProducer) Stop() error {
	p.mutex.Lock()
	if !p.running {
		p.mutex.Unlock()
		return fmt.Errorf("producer is not running")
	}
	p.running = false
	p.mutex.Unlock()
	
	// Cancel context and close connection
	p.cancel()
	if p.conn != nil {
		p.conn.Close()
	}
	
	// Wait for receive loop to finish
	p.wg.Wait()
	
	log.Println("UDP Producer stopped")
	return nil
}

// receiveLoop is the main packet reception loop
func (p *UDPProducer) receiveLoop() {
	defer p.wg.Done()
	
	buffer := make([]byte, p.bufferSize)
	
	for {
		select {
		case <-p.ctx.Done():
			log.Println("Receive loop shutting down")
			return
		default:
			// Continue with packet reception
		}
		
		// Set read deadline for timeout handling
		if p.timeout > 0 {
			p.conn.SetReadDeadline(time.Now().Add(p.timeout))
		}
		
		// Read UDP packet
		n, addr, err := p.conn.ReadFromUDP(buffer)
		if err != nil {
			// Check if this is a timeout or context cancellation
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout occurred, check if we should continue
				select {
				case <-p.ctx.Done():
					return
				default:
					// Reset deadline and continue
					if p.timeout > 0 {
						p.conn.SetReadDeadline(time.Now().Add(p.timeout))
					}
					continue
				}
			}
			
			// Check if connection was closed during shutdown
			select {
			case <-p.ctx.Done():
				return
			default:
				log.Printf("Error reading UDP packet: %v", err)
				p.incrementErrors()
				continue
			}
		}
		
		// Process the received packet
		if err := p.processPacket(buffer[:n], addr); err != nil {
			log.Printf("Error processing packet from %s: %v", addr, err)
			p.incrementErrors()
		}
	}
}

// processPacket processes a received UDP packet
func (p *UDPProducer) processPacket(data []byte, addr *net.UDPAddr) error {
	// Update metrics
	p.mutex.Lock()
	p.packetsReceived++
	p.bytesReceived += int64(len(data))
	p.lastPacketTime = time.Now()
	p.mutex.Unlock()
	
	// Check if this is the first sender or matches the first sender
	if p.firstSender == nil {
		p.mutex.Lock()
		if p.firstSender == nil {
			p.firstSender = addr
			log.Printf("First UDP sender registered: %s", addr)
		}
		p.mutex.Unlock()
	} else if !p.firstSender.IP.Equal(addr.IP) {
		// Ignore packets from different senders (security feature)
		log.Printf("Ignoring packet from unauthorized sender: %s (expected: %s)", 
			addr, p.firstSender)
		return nil
	}
	
	// Parse UDP packet
	packet, err := shakenet.ParseUDPPacket(data)
	if err != nil {
		return fmt.Errorf("failed to parse UDP packet: %v", err)
	}
	
	// Validate packet
	if !packet.IsValidChannel() {
		return fmt.Errorf("invalid channel: %s", packet.Channel)
	}
	
	// Publish to broker
	if err := p.broker.PublishData(packet); err != nil {
		p.mutex.Lock()
		p.packetsDropped++
		p.mutex.Unlock()
		return fmt.Errorf("failed to publish packet: %v", err)
	}
	
	return nil
}

// incrementErrors safely increments the error counter
func (p *UDPProducer) incrementErrors() {
	p.mutex.Lock()
	p.errors++
	p.mutex.Unlock()
}

// GetMetrics returns current producer metrics
func (p *UDPProducer) GetMetrics() ProducerMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	return ProducerMetrics{
		PacketsReceived: p.packetsReceived,
		PacketsDropped:  p.packetsDropped,
		BytesReceived:   p.bytesReceived,
		LastPacketTime:  p.lastPacketTime,
		Errors:          p.errors,
		FirstSender:     p.firstSender,
		Running:         p.running,
	}
}

// ProducerMetrics holds metrics about producer performance
type ProducerMetrics struct {
	PacketsReceived int64         `json:"packets_received"`
	PacketsDropped  int64         `json:"packets_dropped"`
	BytesReceived   int64         `json:"bytes_received"`
	LastPacketTime  time.Time     `json:"last_packet_time"`
	Errors          int64         `json:"errors"`
	FirstSender     *net.UDPAddr  `json:"first_sender"`
	Running         bool          `json:"running"`
}

// GetStatus returns the current status of the producer
func (p *UDPProducer) GetStatus() ProducerStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	status := ProducerStatus{
		Running:        p.running,
		Port:           p.port,
		FirstSender:    p.firstSender,
		LastPacketTime: p.lastPacketTime,
	}
	
	// Determine health based on recent packet activity
	if p.running && !p.lastPacketTime.IsZero() {
		timeSinceLastPacket := time.Since(p.lastPacketTime)
		if timeSinceLastPacket < 5*time.Second {
			status.Health = "healthy"
		} else if timeSinceLastPacket < 30*time.Second {
			status.Health = "warning"
		} else {
			status.Health = "unhealthy"
		}
	} else if p.running {
		status.Health = "waiting"
	} else {
		status.Health = "stopped"
	}
	
	return status
}

// ProducerStatus represents the current status of the producer
type ProducerStatus struct {
	Running        bool          `json:"running"`
	Port           int           `json:"port"`
	FirstSender    *net.UDPAddr  `json:"first_sender"`
	LastPacketTime time.Time     `json:"last_packet_time"`
	Health         string        `json:"health"` // healthy, warning, unhealthy, waiting, stopped
}

// IsHealthy returns true if the producer is operating normally
func (p *UDPProducer) IsHealthy() bool {
	status := p.GetStatus()
	return status.Health == "healthy"
}

// WaitForData waits for the first data packet to arrive or timeout
func (p *UDPProducer) WaitForData(timeout time.Duration) error {
	start := time.Now()
	
	for time.Since(start) < timeout {
		p.mutex.RLock()
		hasData := p.packetsReceived > 0
		p.mutex.RUnlock()
		
		if hasData {
			return nil
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("no data received within %v", timeout)
}