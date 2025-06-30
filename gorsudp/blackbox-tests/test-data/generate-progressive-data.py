#!/usr/bin/env python3
"""
Generate progressive seismic data with gradual amplitude increases
to test sensitivity differences between Python and Go implementations.
"""

import json
import numpy as np
from datetime import datetime, timedelta
import argparse

def create_progressive_signal(duration_sec=120, sample_rate=100, base_noise=500):
    """
    Create seismic data with progressively increasing amplitude events.
    """
    total_samples = int(duration_sec * sample_rate)
    
    # Base background noise
    signal = np.random.normal(0, base_noise, total_samples)
    
    # Add progressive events with increasing amplitude
    events = [
        {"start": 30, "duration": 8, "amplitude": 2000, "freq": 4},    # Low level
        {"start": 50, "duration": 10, "amplitude": 4000, "freq": 5},   # Medium level  
        {"start": 75, "duration": 12, "amplitude": 8000, "freq": 6},   # High level
        {"start": 95, "duration": 15, "amplitude": 15000, "freq": 7},  # Very high level
    ]
    
    for event in events:
        start_sample = int(event["start"] * sample_rate)
        end_sample = int((event["start"] + event["duration"]) * sample_rate)
        
        if end_sample > total_samples:
            end_sample = total_samples
            
        duration = (end_sample - start_sample) / sample_rate
        t = np.linspace(0, duration, end_sample - start_sample)
        
        # Tapered sinusoid with gradual onset and decay
        taper_length = min(len(t) // 4, int(2 * sample_rate))  # 2 second taper or 1/4 length
        
        # Create taper windows
        onset_taper = np.ones(len(t))
        if len(t) > 2 * taper_length:
            onset_taper[:taper_length] = 0.5 * (1 - np.cos(np.pi * np.arange(taper_length) / taper_length))
            onset_taper[-taper_length:] = 0.5 * (1 + np.cos(np.pi * np.arange(taper_length) / taper_length))
        
        # Generate event signal with multiple frequency components
        event_signal = (event["amplitude"] * onset_taper * 
                       (np.sin(2 * np.pi * event["freq"] * t) + 
                        0.3 * np.sin(2 * np.pi * (event["freq"] * 2.5) * t) +
                        0.1 * np.sin(2 * np.pi * (event["freq"] * 4) * t)))
        
        signal[start_sample:end_sample] += event_signal
    
    return signal.astype(np.int32), events

def create_udp_packets(samples, sample_rate=100, packet_size=25):
    """Convert samples into UDP packet format."""
    packets = []
    num_packets = len(samples) // packet_size
    start_time = datetime.now()
    
    for i in range(num_packets):
        packet_start = i * packet_size
        packet_end = packet_start + packet_size
        packet_samples = samples[packet_start:packet_end].tolist()
        
        packet_time = start_time + timedelta(seconds=i * packet_size / sample_rate)
        
        packet = {
            "channel": "EHZ",
            "timestamp": packet_time.isoformat() + "Z", 
            "sample_rate": sample_rate,
            "samples": packet_samples
        }
        packets.append(packet)
    
    return packets

def main():
    parser = argparse.ArgumentParser(description='Generate progressive test data')
    parser.add_argument('--duration', type=int, default=120, help='Duration in seconds')
    parser.add_argument('--sample-rate', type=int, default=100, help='Sample rate')
    parser.add_argument('--base-noise', type=int, default=500, help='Base noise level')
    parser.add_argument('--output', type=str, default='progressive-test-data.json', help='Output file')
    
    args = parser.parse_args()
    
    print("Generating progressive seismic test data...")
    
    # Generate signal
    samples, events = create_progressive_signal(
        duration_sec=args.duration,
        sample_rate=args.sample_rate, 
        base_noise=args.base_noise
    )
    
    print(f"Generated {len(samples)} samples")
    print(f"Sample range: {samples.min()} to {samples.max()}")
    
    # Create packets
    packets = create_udp_packets(samples, args.sample_rate)
    
    # Create output
    output_data = {
        "metadata": {
            "generated_at": datetime.now().isoformat() + "Z",
            "generator": "generate-progressive-data.py",
            "duration_sec": args.duration,
            "sample_rate": args.sample_rate,
            "total_samples": len(samples),
            "total_packets": len(packets),
            "base_noise": args.base_noise,
            "events": events
        },
        "packets": packets,
        "expected_behavior": {
            "description": "Progressive events with increasing STA/LTA levels",
            "events": [
                {"time": "30-38s", "expected_stalta": "2-4", "should_trigger": False},
                {"time": "50-60s", "expected_stalta": "4-6", "should_trigger": True},
                {"time": "75-87s", "expected_stalta": "6-10", "should_trigger": True},
                {"time": "95-110s", "expected_stalta": "10-20", "should_trigger": True}
            ]
        }
    }
    
    with open(args.output, 'w') as f:
        json.dump(output_data, f, indent=2)
    
    print(f"Progressive test data saved to: {args.output}")
    print("Events scheduled:")
    for i, event in enumerate(events):
        print(f"  Event {i+1}: {event['start']}-{event['start']+event['duration']}s, amplitude {event['amplitude']}")

if __name__ == "__main__":
    main()