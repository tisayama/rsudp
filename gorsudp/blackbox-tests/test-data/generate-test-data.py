#!/usr/bin/env python3
"""
Generate synthetic test data for STA/LTA comparison tests.
Creates realistic seismic data with known characteristics.
"""

import json
import numpy as np
from datetime import datetime, timedelta
import argparse
import sys

def create_seismic_signal(duration_sec=120, sample_rate=100, noise_level=1000, 
                         earthquake_start=60, earthquake_duration=10, earthquake_amplitude=5000):
    """
    Create synthetic seismic data with background noise and earthquake signal.
    
    Args:
        duration_sec: Total duration in seconds
        sample_rate: Samples per second  
        noise_level: Background noise amplitude (counts)
        earthquake_start: When earthquake starts (seconds)
        earthquake_duration: Duration of earthquake (seconds)
        earthquake_amplitude: Peak amplitude of earthquake (counts)
    
    Returns:
        numpy array of samples
    """
    total_samples = int(duration_sec * sample_rate)
    
    # Generate background noise (Gaussian)
    background = np.random.normal(0, noise_level, total_samples)
    
    # Add earthquake signal (decaying sinusoid)
    eq_start_sample = int(earthquake_start * sample_rate)
    eq_end_sample = int((earthquake_start + earthquake_duration) * sample_rate)
    
    if eq_end_sample > total_samples:
        eq_end_sample = total_samples
    
    eq_samples = eq_end_sample - eq_start_sample
    
    if eq_samples > 0:
        # Create earthquake signal: decaying sinusoid with multiple frequencies
        t = np.linspace(0, earthquake_duration, eq_samples)
        
        # Main frequency around 5 Hz (typical for local earthquakes)
        main_freq = 5.0
        # Add some higher frequency content  
        high_freq = 15.0
        
        # Exponential decay envelope
        envelope = np.exp(-t / (earthquake_duration * 0.3))
        
        # Multi-frequency earthquake signal
        earthquake = (earthquake_amplitude * envelope * 
                     (np.sin(2 * np.pi * main_freq * t) + 
                      0.3 * np.sin(2 * np.pi * high_freq * t)))
        
        # Add earthquake to background noise
        background[eq_start_sample:eq_end_sample] += earthquake
    
    return background.astype(np.int32)

def create_udp_packets(samples, sample_rate=100, packet_size=25):
    """
    Convert samples into UDP packet format (matching Raspberry Shake format).
    
    Args:
        samples: Array of sample values
        sample_rate: Samples per second
        packet_size: Samples per packet
    
    Returns:
        List of UDP packet dictionaries
    """
    packets = []
    num_packets = len(samples) // packet_size
    
    # Start time for first packet
    start_time = datetime.now()
    
    for i in range(num_packets):
        packet_start = i * packet_size
        packet_end = packet_start + packet_size
        packet_samples = samples[packet_start:packet_end].tolist()
        
        # Calculate timestamp for this packet
        packet_time = start_time + timedelta(seconds=i * packet_size / sample_rate)
        
        packet = {
            "channel": "EHZ",
            "timestamp": packet_time.isoformat() + "Z", 
            "sample_rate": sample_rate,
            "samples": packet_samples,
            "metadata": {
                "packet_index": i,
                "total_packets": num_packets,
                "samples_per_packet": packet_size
            }
        }
        packets.append(packet)
    
    return packets

def main():
    parser = argparse.ArgumentParser(description='Generate synthetic test data for STA/LTA comparison')
    parser.add_argument('--duration', type=int, default=120, help='Duration in seconds (default: 120)')
    parser.add_argument('--sample-rate', type=int, default=100, help='Sample rate (default: 100)')
    parser.add_argument('--noise-level', type=int, default=1000, help='Background noise level (default: 1000)')
    parser.add_argument('--earthquake-start', type=int, default=60, help='Earthquake start time (default: 60s)')
    parser.add_argument('--earthquake-duration', type=int, default=10, help='Earthquake duration (default: 10s)')
    parser.add_argument('--earthquake-amplitude', type=int, default=5000, help='Earthquake amplitude (default: 5000)')
    parser.add_argument('--output', type=str, default='sample-packets.json', help='Output file (default: sample-packets.json)')
    
    args = parser.parse_args()
    
    print("Generating synthetic seismic data...")
    print(f"Duration: {args.duration}s, Sample rate: {args.sample_rate} Hz")
    print(f"Earthquake: {args.earthquake_start}s-{args.earthquake_start + args.earthquake_duration}s")
    
    # Generate seismic signal
    samples = create_seismic_signal(
        duration_sec=args.duration,
        sample_rate=args.sample_rate,
        noise_level=args.noise_level,
        earthquake_start=args.earthquake_start,
        earthquake_duration=args.earthquake_duration,
        earthquake_amplitude=args.earthquake_amplitude
    )
    
    print(f"Generated {len(samples)} samples")
    print(f"Sample range: {samples.min()} to {samples.max()}")
    
    # Create UDP packets
    packets = create_udp_packets(samples, args.sample_rate)
    
    print(f"Created {len(packets)} UDP packets")
    
    # Create output data structure
    output_data = {
        "metadata": {
            "generated_at": datetime.now().isoformat() + "Z",
            "generator": "generate-test-data.py",
            "duration_sec": args.duration,
            "sample_rate": args.sample_rate,
            "total_samples": len(samples),
            "total_packets": len(packets),
            "earthquake": {
                "start_sec": args.earthquake_start,
                "duration_sec": args.earthquake_duration,
                "amplitude": args.earthquake_amplitude
            },
            "noise_level": args.noise_level
        },
        "packets": packets,
        "expected_behavior": {
            "pre_earthquake": {
                "description": "Background noise only, STA/LTA should be low (~1.0)",
                "time_range": [0, args.earthquake_start]
            },
            "earthquake": {
                "description": "STA/LTA should exceed threshold (~3.95), trigger expected",
                "time_range": [args.earthquake_start, args.earthquake_start + args.earthquake_duration]
            },
            "post_earthquake": {
                "description": "STA/LTA should decay below reset threshold (~0.9)",
                "time_range": [args.earthquake_start + args.earthquake_duration, args.duration]
            }
        }
    }
    
    # Save to file
    with open(args.output, 'w') as f:
        json.dump(output_data, f, indent=2)
    
    print(f"Test data saved to: {args.output}")
    print(f"File size: {len(json.dumps(output_data)) / 1024:.1f} KB")

if __name__ == "__main__":
    main()