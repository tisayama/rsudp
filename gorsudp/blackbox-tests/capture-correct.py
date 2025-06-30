#!/usr/bin/env python3
"""
Correct Raspberry Shake UDP data capture for blackbox testing.
Captures string-format packets: "{'CHANNEL', TIMESTAMP, DATA1, DATA2, ...}"
"""

import socket
import json
import time
from datetime import datetime, timezone
import argparse
import signal
import re

class RaspberryShakeCapture:
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
    
    def parse_packet(self, data_str, source_addr):
        """Parse Raspberry Shake string format packet."""
        try:
            # Remove outer braces and quotes
            cleaned = data_str.strip()
            if not (cleaned.startswith("{") and cleaned.endswith("}")):
                return None
            
            cleaned = cleaned[1:-1]  # Remove { }
            if cleaned.startswith("'") and cleaned.endswith("'"):
                cleaned = cleaned[1:-1]  # Remove outer quotes
            
            # Split by comma
            parts = [p.strip() for p in cleaned.split(",")]
            if len(parts) < 3:
                return None
            
            # Parse channel
            channel = parts[0].strip("'\"")
            if not channel:
                return None
            
            # Parse timestamp
            try:
                timestamp_float = float(parts[1])
                timestamp = datetime.fromtimestamp(timestamp_float, tz=timezone.utc)
            except:
                return None
            
            # Parse data values
            samples = []
            for i in range(2, len(parts)):
                try:
                    value = int(parts[i].strip())
                    samples.append(value)
                except:
                    continue
            
            if not samples:
                return None
            
            packet_info = {
                "channel": channel,
                "timestamp": timestamp.isoformat(),
                "sample_rate": 100,  # Default for Raspberry Shake
                "samples": samples,
                "metadata": {
                    "source_ip": source_addr[0],
                    "source_port": source_addr[1],
                    "packet_size": len(data_str),
                    "num_samples": len(samples)
                }
            }
            
            return packet_info
            
        except Exception as e:
            return None
    
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
                    
                    # Convert to string
                    try:
                        data_str = data.decode('utf-8', errors='ignore')
                    except:
                        continue
                    
                    # Parse packet
                    packet_info = self.parse_packet(data_str, addr)
                    
                    if packet_info:
                        self.packets.append(packet_info)
                        
                        # Print first few packets for debugging
                        if len(self.packets) <= 3:
                            print(f"Sample packet {len(self.packets)}: {packet_info['channel']}, "
                                  f"{len(packet_info['samples'])} samples")
                    
                    if packet_count % 100 == 0:
                        elapsed = current_time - start_time
                        print(f"Captured {len(self.packets)} valid packets "
                              f"({packet_count} total) in {elapsed:.1f}s")
                    
                except socket.timeout:
                    continue
                except Exception as e:
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
        
        total_samples = sum(len(p['samples']) for p in self.packets)
        channels = list(set(p['channel'] for p in self.packets))
        
        output_data = {
            "metadata": {
                "generated_at": datetime.now(timezone.utc).isoformat(),
                "generator": "capture-correct.py",
                "capture_port": self.port,
                "capture_duration_sec": self.duration,
                "total_packets": len(self.packets),
                "total_samples": total_samples,
                "channels": channels,
                "data_source": "live_capture_raspberry_shake",
                "first_packet_time": self.packets[0]["timestamp"] if self.packets else None,
                "last_packet_time": self.packets[-1]["timestamp"] if self.packets else None
            },
            "packets": self.packets
        }
        
        try:
            with open(output_file, 'w') as f:
                json.dump(output_data, f, indent=2)
            print(f"Data saved to: {output_file}")
            print(f"Summary:")
            print(f"  Packets: {len(self.packets)}")
            print(f"  Total samples: {total_samples}")
            print(f"  Channels: {', '.join(channels)}")
            print(f"  Average samples per packet: {total_samples / len(self.packets):.1f}")
            return True
        except Exception as e:
            print(f"Error saving data: {e}")
            return False

def main():
    parser = argparse.ArgumentParser(description='Raspberry Shake UDP data capture')
    parser.add_argument('--port', type=int, default=8889, help='UDP port (default: 8889)')
    parser.add_argument('--duration', type=int, default=60, help='Duration in seconds (default: 60)')
    parser.add_argument('--output', type=str, default='live-production-data.json', help='Output file')
    
    args = parser.parse_args()
    
    print("=== Raspberry Shake UDP Data Capture ===")
    print(f"Port: {args.port}, Duration: {args.duration}s, Output: {args.output}")
    print("Expected format: {'CHANNEL', TIMESTAMP, DATA1, DATA2, ...}")
    print()
    
    capture = RaspberryShakeCapture(port=args.port, duration=args.duration)
    
    if capture.capture_data():
        if capture.save_data(args.output):
            print("\nCapture successful! Ready for blackbox testing.")
        else:
            print("Failed to save data")
    else:
        print("Failed to capture data")

if __name__ == "__main__":
    main()