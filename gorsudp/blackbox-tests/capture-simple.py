#!/usr/bin/env python3
"""
Simple UDP data capture for blackbox testing.
Captures raw UDP packets and converts to the format expected by our tests.
"""

import socket
import json
import time
from datetime import datetime, timezone
import argparse
import signal

class SimpleUDPCapture:
    def __init__(self, port=8889, duration=60):
        self.port = port
        self.duration = duration
        self.packets = []
        self.running = True
        
        signal.signal(signal.SIGINT, self.signal_handler)
        signal.signal(signal.SIGTERM, self.signal_handler)
    
    def signal_handler(self, signum, frame):
        print(f"\nReceived signal {signum}, stopping capture...")
        self.running = False
    
    def capture_data(self):
        """Capture UDP packets from the specified port."""
        
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.settimeout(1.0)
        
        try:
            sock.bind(('0.0.0.0', self.port))
            print(f"Listening on UDP port {self.port} for {self.duration} seconds...")
            print("Press Ctrl+C to stop capture early")
            
            start_time = time.time()
            packet_count = 0
            
            while self.running:
                current_time = time.time()
                
                if current_time - start_time >= self.duration:
                    print(f"\nCapture duration ({self.duration}s) reached")
                    break
                
                try:
                    data, addr = sock.recvfrom(65536)
                    packet_count += 1
                    
                    # Create a simple packet entry
                    packet_info = {
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                        "channel": "EHZ",  # Default channel
                        "sample_rate": 100,  # Default sample rate
                        "samples": [],  # Will be filled with actual data
                        "raw_data": data.hex(),  # Store raw data for analysis
                        "source_ip": addr[0],
                        "data_length": len(data)
                    }
                    
                    # Try to extract samples (basic parsing)
                    if len(data) >= 4:
                        # Assume samples are 32-bit integers starting from some offset
                        # This is a simplified approach - adjust based on actual packet format
                        samples = []
                        # Try different starting positions to find sample data
                        for offset in [64, 32, 16, 0]:  # Common header sizes
                            if offset + 100 <= len(data):  # Need at least 25 samples (100 bytes)
                                try:
                                    sample_data = data[offset:offset+100]  # 25 samples * 4 bytes
                                    import struct
                                    samples = list(struct.unpack('!25i', sample_data))
                                    break
                                except:
                                    continue
                        
                        if samples and any(abs(s) < 1000000 for s in samples):  # Basic sanity check
                            packet_info["samples"] = samples
                            self.packets.append(packet_info)
                    
                    if packet_count % 100 == 0:
                        elapsed = current_time - start_time
                        print(f"Captured {len(self.packets)} valid packets "
                              f"({packet_count} total) in {elapsed:.1f}s")
                    
                except socket.timeout:
                    continue
                except Exception as e:
                    print(f"Error processing packet: {e}")
                    continue
            
        except Exception as e:
            print(f"Error setting up socket: {e}")
            return False
        finally:
            sock.close()
        
        print(f"\nCapture completed: {len(self.packets)} valid packets captured")
        return True
    
    def save_data(self, output_file):
        """Save captured data."""
        if not self.packets:
            print("No packets to save")
            return False
        
        # Filter packets to get consistent data
        valid_packets = [p for p in self.packets if len(p.get('samples', [])) == 25]
        
        if not valid_packets:
            print("No valid packets with sample data found")
            return False
        
        total_samples = sum(len(p['samples']) for p in valid_packets)
        
        output_data = {
            "metadata": {
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "generator": "capture-simple.py",
                "capture_port": self.port,
                "capture_duration_sec": self.duration,
                "total_packets": len(valid_packets),
                "total_samples": total_samples,
                "data_source": "live_capture_simple"
            },
            "packets": valid_packets
        }
        
        try:
            with open(output_file, 'w') as f:
                json.dump(output_data, f, indent=2)
            print(f"Data saved to: {output_file}")
            print(f"Summary: {len(valid_packets)} packets, {total_samples} samples")
            return True
        except Exception as e:
            print(f"Error saving data: {e}")
            return False

def main():
    parser = argparse.ArgumentParser(description='Simple UDP data capture')
    parser.add_argument('--port', type=int, default=8889, help='UDP port (default: 8889)')
    parser.add_argument('--duration', type=int, default=60, help='Duration in seconds (default: 60)')
    parser.add_argument('--output', type=str, default='live-simple-data.json', help='Output file')
    
    args = parser.parse_args()
    
    print("=== Simple UDP Data Capture ===")
    print(f"Port: {args.port}, Duration: {args.duration}s, Output: {args.output}")
    
    capture = SimpleUDPCapture(port=args.port, duration=args.duration)
    
    if capture.capture_data():
        if capture.save_data(args.output):
            print("\nCapture successful!")
        else:
            print("Failed to save data")
    else:
        print("Failed to capture data")

if __name__ == "__main__":
    main()