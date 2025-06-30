#!/usr/bin/env python3
"""
Capture live UDP data from port 8889 and save for blackbox testing.
This script captures real production data for STA/LTA comparison tests.
"""

import socket
import json
import struct
import time
from datetime import datetime, timezone
import argparse
import signal
import sys

class UDPDataCapture:
    def __init__(self, port=8889, duration=120):
        self.port = port
        self.duration = duration
        self.packets = []
        self.running = True
        
        # Set up signal handler for graceful shutdown
        signal.signal(signal.SIGINT, self.signal_handler)
        signal.signal(signal.SIGTERM, self.signal_handler)
    
    def signal_handler(self, signum, frame):
        print(f"\nReceived signal {signum}, stopping capture...")
        self.running = False
    
    def parse_raspberry_shake_packet(self, data, source_addr):
        """
        Parse Raspberry Shake UDP packet format.
        Based on the structure used in rsudp and gorsudp.
        """
        try:
            if len(data) < 64:  # Minimum header size
                return None
            
            # Parse header (first 64 bytes)
            # This is a simplified parser - adjust based on actual packet format
            header = struct.unpack('!16s8s8s8sIIII8s', data[:64])
            
            # Extract basic information
            station = header[0].decode('ascii').strip('\x00')
            network = header[1].decode('ascii').strip('\x00') 
            channel = header[2].decode('ascii').strip('\x00')
            location = header[3].decode('ascii').strip('\x00')
            
            sample_rate = header[4]
            timestamp_sec = header[5]
            timestamp_usec = header[6]
            num_samples = header[7]
            
            # Calculate timestamp
            timestamp = datetime.fromtimestamp(
                timestamp_sec + timestamp_usec / 1000000.0,
                tz=timezone.utc
            )
            
            # Parse sample data (remaining bytes as 32-bit integers)
            sample_data_start = 64
            sample_data = data[sample_data_start:]
            
            if len(sample_data) < num_samples * 4:
                print(f"Warning: Incomplete sample data, expected {num_samples} samples")
                num_samples = len(sample_data) // 4
            
            # Unpack samples as signed 32-bit integers
            samples = list(struct.unpack(f'!{num_samples}i', sample_data[:num_samples*4]))
            
            packet_info = {
                "timestamp": timestamp.isoformat(),
                "source_ip": source_addr[0],
                "source_port": source_addr[1],
                "station": station,
                "network": network,
                "channel": channel,
                "location": location,
                "sample_rate": sample_rate,
                "samples": samples,
                "metadata": {
                    "packet_size": len(data),
                    "header_size": 64,
                    "num_samples_declared": header[7],
                    "num_samples_actual": len(samples),
                    "timestamp_sec": timestamp_sec,
                    "timestamp_usec": timestamp_usec
                }
            }
            
            return packet_info
            
        except Exception as e:
            print(f"Error parsing packet: {e}")
            return None
    
    def capture_data(self):
        """Capture UDP packets from the specified port."""
        
        # Create UDP socket
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.settimeout(1.0)  # 1 second timeout for non-blocking operation
        
        try:
            # Bind to the port
            sock.bind(('0.0.0.0', self.port))
            print(f"Listening on UDP port {self.port} for {self.duration} seconds...")
            print("Press Ctrl+C to stop capture early")
            
            start_time = time.time()
            packet_count = 0
            
            while self.running:
                current_time = time.time()
                
                # Check if duration has elapsed
                if current_time - start_time >= self.duration:
                    print(f"\nCapture duration ({self.duration}s) reached")
                    break
                
                try:
                    # Receive packet
                    data, addr = sock.recvfrom(65536)  # Max UDP packet size
                    packet_count += 1
                    
                    # Parse the packet
                    packet_info = self.parse_raspberry_shake_packet(data, addr)
                    
                    if packet_info:
                        self.packets.append(packet_info)
                        
                        # Print progress every 100 packets
                        if packet_count % 100 == 0:
                            elapsed = current_time - start_time
                            print(f"Captured {len(self.packets)} valid packets "
                                  f"({packet_count} total) in {elapsed:.1f}s")
                    
                except socket.timeout:
                    # Check if we should continue
                    continue
                except Exception as e:
                    print(f"Error receiving packet: {e}")
                    continue
            
        except Exception as e:
            print(f"Error setting up socket: {e}")
            return False
        finally:
            sock.close()
        
        print(f"\nCapture completed: {len(self.packets)} valid packets captured")
        return True
    
    def save_data(self, output_file):
        """Save captured data in blackbox test format."""
        
        if not self.packets:
            print("No packets to save")
            return False
        
        # Calculate statistics
        total_samples = sum(len(p['samples']) for p in self.packets)
        channels = list(set(p['channel'] for p in self.packets))
        sample_rates = list(set(p['sample_rate'] for p in self.packets))
        
        # Create output structure compatible with blackbox tests
        output_data = {
            "metadata": {
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "generator": "capture-live-data.py",
                "capture_port": self.port,
                "capture_duration_sec": self.duration,
                "total_packets": len(self.packets),
                "total_samples": total_samples,
                "channels": channels,
                "sample_rates": sample_rates,
                "data_source": "live_capture",
                "first_packet_time": self.packets[0]["timestamp"] if self.packets else None,
                "last_packet_time": self.packets[-1]["timestamp"] if self.packets else None
            },
            "packets": self.packets
        }
        
        # Save to file
        try:
            with open(output_file, 'w') as f:
                json.dump(output_data, f, indent=2)
            print(f"Data saved to: {output_file}")
            
            # Print summary
            print(f"Summary:")
            print(f"  Packets: {len(self.packets)}")
            print(f"  Total samples: {total_samples}")
            print(f"  Channels: {', '.join(channels)}")
            print(f"  Sample rates: {', '.join(map(str, sample_rates))}")
            print(f"  Duration: {self.duration}s")
            
            return True
            
        except Exception as e:
            print(f"Error saving data: {e}")
            return False

def main():
    parser = argparse.ArgumentParser(description='Capture live UDP data for blackbox testing')
    parser.add_argument('--port', type=int, default=8889, help='UDP port to listen on (default: 8889)')
    parser.add_argument('--duration', type=int, default=120, help='Capture duration in seconds (default: 120)')
    parser.add_argument('--output', type=str, default='live-capture-data.json', help='Output file (default: live-capture-data.json)')
    
    args = parser.parse_args()
    
    print("=== Live UDP Data Capture for Blackbox Testing ===")
    print(f"Port: {args.port}")
    print(f"Duration: {args.duration} seconds")
    print(f"Output: {args.output}")
    print()
    
    # Create capture instance
    capture = UDPDataCapture(port=args.port, duration=args.duration)
    
    # Capture data
    if capture.capture_data():
        # Save data
        if capture.save_data(args.output):
            print("\nCapture successful! You can now run blackbox tests with this data.")
            print(f"Example: ./run-test.sh {args.output}")
        else:
            print("Failed to save captured data")
            sys.exit(1)
    else:
        print("Failed to capture data")
        sys.exit(1)

if __name__ == "__main__":
    main()