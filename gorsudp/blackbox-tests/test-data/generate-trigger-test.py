#!/usr/bin/env python3
"""
Generate test data specifically designed to trigger STA/LTA alerts.
Creates realistic scenarios that should exceed threshold values.
"""

import json
import numpy as np
from datetime import datetime, timedelta

def create_trigger_scenario():
    """Create data designed to trigger STA/LTA alerts reliably."""
    
    sample_rate = 100
    duration_sec = 120
    total_samples = duration_sec * sample_rate
    
    # Low background noise for better signal-to-noise ratio
    background_noise = 300
    signal = np.random.normal(0, background_noise, total_samples)
    
    # Multiple trigger events with different characteristics
    events = [
        # Event 1: Moderate event that should just trigger
        {
            "start": 25,
            "duration": 12,
            "amplitude": 8000,
            "freq": 5.0,
            "description": "Moderate trigger event"
        },
        
        # Event 2: Strong event that should clearly trigger  
        {
            "start": 55,
            "duration": 15,
            "amplitude": 15000,
            "freq": 6.0,
            "description": "Strong trigger event"
        },
        
        # Event 3: Very strong event with multiple frequencies
        {
            "start": 85,
            "duration": 20,
            "amplitude": 25000,
            "freq": 4.0,
            "description": "Very strong multi-frequency event"
        }
    ]
    
    for event in events:
        start_sample = int(event["start"] * sample_rate)
        end_sample = int((event["start"] + event["duration"]) * sample_rate)
        
        if end_sample > total_samples:
            end_sample = total_samples
            
        event_length = end_sample - start_sample
        t = np.linspace(0, event["duration"], event_length)
        
        # Create realistic earthquake signal with:
        # - Sudden onset (P-wave)
        # - Stronger S-wave arrival
        # - Gradual decay
        
        # P-wave: smaller amplitude, higher frequency
        p_wave_duration = min(3.0, event["duration"] * 0.3)
        p_samples = int(p_wave_duration * sample_rate)
        
        if p_samples > 0:
            p_t = t[:p_samples]
            p_wave = (event["amplitude"] * 0.3 * 
                     np.exp(-p_t / (p_wave_duration * 0.8)) *
                     np.sin(2 * np.pi * (event["freq"] * 2) * p_t))
            signal[start_sample:start_sample + p_samples] += p_wave
        
        # S-wave: larger amplitude, lower frequency, longer duration
        s_start = start_sample + p_samples
        s_length = event_length - p_samples
        
        if s_length > 0:
            s_t = t[p_samples:]
            # Envelope that rises quickly then decays
            envelope = np.exp(-s_t / (event["duration"] * 0.4))
            # Add quick onset
            quick_onset = np.minimum(s_t * 10, 1.0)
            envelope *= quick_onset
            
            # Multi-frequency S-wave
            s_wave = (event["amplitude"] * envelope * 
                     (np.sin(2 * np.pi * event["freq"] * s_t) +
                      0.4 * np.sin(2 * np.pi * (event["freq"] * 0.7) * s_t) +
                      0.2 * np.sin(2 * np.pi * (event["freq"] * 1.5) * s_t)))
            
            signal[s_start:end_sample] += s_wave[:s_length]
    
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
    print("Generating trigger-optimized test data...")
    
    samples, events = create_trigger_scenario()
    
    print(f"Generated {len(samples)} samples")
    print(f"Sample range: {samples.min()} to {samples.max()}")
    
    packets = create_udp_packets(samples)
    
    output_data = {
        "metadata": {
            "generated_at": datetime.now().isoformat() + "Z",
            "generator": "generate-trigger-test.py",
            "duration_sec": 120,
            "sample_rate": 100,
            "total_samples": len(samples),
            "total_packets": len(packets),
            "description": "Optimized for reliable STA/LTA triggering",
            "events": events
        },
        "packets": packets,
        "expected_behavior": {
            "description": "Multiple trigger events designed to exceed threshold 3.95",
            "threshold": 3.95,
            "reset": 0.9,
            "expected_triggers": 3,
            "events": [
                {"time": "25-37s", "expected_stalta": "4-8", "should_trigger": True},
                {"time": "55-70s", "expected_stalta": "8-15", "should_trigger": True},
                {"time": "85-105s", "expected_stalta": "15-30", "should_trigger": True}
            ]
        }
    }
    
    with open('trigger-test-data.json', 'w') as f:
        json.dump(output_data, f, indent=2)
    
    print("Trigger test data saved to: trigger-test-data.json")
    print("Expected triggers:")
    for event in events:
        print(f"  {event['start']}-{event['start']+event['duration']}s: {event['description']}")

if __name__ == "__main__":
    main()