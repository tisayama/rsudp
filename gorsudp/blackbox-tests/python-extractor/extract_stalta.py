#!/usr/bin/env python3
"""
Extract STA/LTA values using Python rsudp implementation logic.
This matches the behavior of rsudp c_alert.py as closely as possible.
"""

import json
import sys
import numpy as np
from datetime import datetime
from obspy.signal.trigger import recursive_sta_lta
from scipy.signal import butter, sosfiltfilt
import argparse

class STALTAExtractor:
    def __init__(self, config):
        """Initialize with configuration matching rsudp settings."""
        self.config = config
        self.sta_duration = config.get('sta_duration', 6.0)
        self.lta_duration = config.get('lta_duration', 30.0)
        self.threshold = config.get('threshold', 3.95)
        self.reset = config.get('reset', 0.9)
        self.sample_rate = config.get('sample_rate', 100.0)
        self.deconv = config.get('deconv', True)
        
        # Filter settings
        self.filter_enabled = config.get('filter_enabled', True)
        self.highpass = config.get('highpass', 0.8)
        self.lowpass = config.get('lowpass', 9.0)
        self.filter_corners = config.get('filter_corners', 2)
        
        # Sensitivity for deconvolution (matching Go implementation)
        self.sensitivity = 1.6e8  # counts/(m/s) for geophone channels
        
        # Calculate window sizes
        self.sta_samples = int(self.sta_duration * self.sample_rate)
        self.lta_samples = int(self.lta_duration * self.sample_rate)
        
        # Track trigger state
        self.triggered = False
        
    def deconvolve(self, data):
        """Apply deconvolution if enabled."""
        if self.deconv:
            # Convert counts to m/s (matching Python rsudp behavior)
            return data / self.sensitivity
        return data
        
    def apply_filter(self, data):
        """Apply Butterworth bandpass filter if enabled."""
        if not self.filter_enabled:
            return data
            
        # Create filter coefficients
        nyquist = self.sample_rate / 2.0
        
        if self.highpass > 0 and self.lowpass > 0:
            # Bandpass filter
            sos = butter(self.filter_corners, 
                        [self.highpass/nyquist, self.lowpass/nyquist], 
                        btype='band', 
                        output='sos')
        elif self.highpass > 0:
            # Highpass filter
            sos = butter(self.filter_corners, 
                        self.highpass/nyquist, 
                        btype='high', 
                        output='sos')
        elif self.lowpass > 0:
            # Lowpass filter
            sos = butter(self.filter_corners, 
                        self.lowpass/nyquist, 
                        btype='low', 
                        output='sos')
        else:
            return data
            
        # Apply zero-phase filter (forward-backward)
        return sosfiltfilt(sos, data)
        
    def process_samples(self, samples):
        """Process samples and extract STA/LTA values using recursive implementation matching ObsPy."""
        # Convert to numpy array
        data = np.array(samples, dtype=np.float64)
        
        # Apply deconvolution if enabled (before STA/LTA)
        data = self.deconvolve(data)
        
        # Remove DC offset
        data = data - np.mean(data)
        
        # Apply filter if enabled
        data = self.apply_filter(data)
        
        # Calculate STA/LTA using ObsPy's recursive implementation
        stalta = recursive_sta_lta(data, self.sta_samples, self.lta_samples)
        
        # Manual recursive implementation to get individual STA/LTA values (matching ObsPy)
        sta = 0.0
        lta = np.finfo(0.0).tiny  # ObsPy initialization: avoid zero division
        
        # Calculate coefficients
        csta = 1.0 / self.sta_samples   # 1/nsta
        clta = 1.0 / self.lta_samples   # 1/nlta
        icsta = 1.0 - csta              # 1 - csta
        iclta = 1.0 - clta              # 1 - clta
        
        # Extract results for each sample
        results = []
        
        for i in range(len(data)):
            # Square the sample (energy-based, matching ObsPy)
            squared_sample = data[i] * data[i]
            
            # Recursive calculation (matching ObsPy formula)
            sta = csta * squared_sample + icsta * sta
            lta = clta * squared_sample + iclta * lta
            
            # Calculate ratio
            if lta > 0:
                manual_ratio = sta / lta
            else:
                manual_ratio = 0.0
            
            # Use ObsPy's ratio (should match our manual calculation)
            ratio = stalta[i]
            
            # Check trigger state
            if ratio > self.threshold and not self.triggered:
                self.triggered = True
            elif ratio < self.reset and self.triggered:
                self.triggered = False
                
            results.append({
                'sample_index': i,
                'sta_value': float(sta),
                'lta_value': float(lta),
                'ratio': float(ratio),
                'manual_ratio': float(manual_ratio),  # For debugging
                'triggered': self.triggered
            })
            
        return results

def main():
    parser = argparse.ArgumentParser(description='Extract STA/LTA values using Python implementation')
    parser.add_argument('input_file', help='Input JSON file with UDP packets')
    parser.add_argument('output_file', help='Output JSON file for results')
    parser.add_argument('--config', type=str, help='Optional config JSON file')
    
    args = parser.parse_args()
    
    # Default configuration matching rsudp
    config = {
        'sta_duration': 6.0,
        'lta_duration': 30.0,
        'threshold': 3.95,
        'reset': 0.9,
        'sample_rate': 100.0,
        'deconv': True,  # Python default
        'filter_enabled': False,  # Python default is no filtering
        'highpass': 0.8,
        'lowpass': 9.0,
        'filter_corners': 2
    }
    
    # Load custom config if provided
    if args.config:
        with open(args.config, 'r') as f:
            custom_config = json.load(f)
            config.update(custom_config)
    
    print(f"Loading test data from: {args.input_file}")
    
    # Load input data
    with open(args.input_file, 'r') as f:
        input_data = json.load(f)
    
    packets = input_data['packets']
    print(f"Loaded {len(packets)} packets")
    
    # Combine all samples
    all_samples = []
    timestamps = []
    
    for packet in packets:
        all_samples.extend(packet['samples'])
        # Generate timestamps for each sample
        packet_time = datetime.fromisoformat(packet['timestamp'].rstrip('Z'))
        sample_rate = packet.get('sample_rate', 100)
        for i in range(len(packet['samples'])):
            sample_time = packet_time.timestamp() + (i / sample_rate)
            timestamps.append(sample_time)
    
    print(f"Total samples: {len(all_samples)}")
    
    # Create extractor
    extractor = STALTAExtractor(config)
    
    # Process samples
    print("Processing STA/LTA...")
    results = extractor.process_samples(all_samples)
    
    # Add timestamps to results
    for i, result in enumerate(results):
        if i < len(timestamps):
            result['timestamp'] = datetime.fromtimestamp(timestamps[i]).isoformat() + 'Z'
    
    # Count triggers
    trigger_count = sum(1 for j in range(len(results)) if results[j]['triggered'] and (j == 0 or not results[j-1]['triggered']))
    print(f"Found {trigger_count} trigger(s)")
    
    # Create output structure
    output_data = {
        'metadata': {
            'timestamp': datetime.now().isoformat() + 'Z',
            'implementation': 'python',
            'config': config,
            'total_samples': len(all_samples),
            'trigger_count': trigger_count
        },
        'results': results
    }
    
    # Save results
    with open(args.output_file, 'w') as f:
        json.dump(output_data, f, indent=2)
    
    print(f"Results saved to: {args.output_file}")
    
    # Print summary statistics
    ratios = [r['ratio'] for r in results if r['ratio'] > 0]
    if ratios:
        print(f"STA/LTA ratio statistics:")
        print(f"  Min: {min(ratios):.3f}")
        print(f"  Max: {max(ratios):.3f}")
        print(f"  Mean: {np.mean(ratios):.3f}")

if __name__ == "__main__":
    main()