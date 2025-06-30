#!/usr/bin/env python3
"""
Test ObsPy's recursive_sta_lta behavior during warmup period to understand
when it starts returning non-zero ratios.
"""

import numpy as np
from obspy.signal.trigger import recursive_sta_lta

def test_obspy_warmup_behavior():
    """Test ObsPy's behavior during warmup period."""
    
    # Create simple test data
    data = np.array([1000, 1100, 900, 1050, 950, 1200, 800, 1150, 850, 1300], dtype=np.float64)
    
    # Apply deconvolution like the Python implementation
    sensitivity = 1.6e8
    data = data / sensitivity
    
    # Remove DC offset
    data = data - np.mean(data)
    
    print("Test data after deconvolution and DC removal:")
    for i, val in enumerate(data):
        print(f"  Sample {i}: {val:.6e}")
    
    # STA/LTA parameters
    sta_samples = int(6.0 * 100)  # 6 seconds * 100 Hz = 600 samples
    lta_samples = int(30.0 * 100)  # 30 seconds * 100 Hz = 3000 samples
    
    print(f"\nSTA window: {sta_samples} samples")
    print(f"LTA window: {lta_samples} samples")
    print(f"Data length: {len(data)} samples")
    
    # Calculate ObsPy's recursive STA/LTA
    ratios = recursive_sta_lta(data, sta_samples, lta_samples)
    
    print(f"\nObsPy ratios:")
    for i, ratio in enumerate(ratios):
        print(f"  Sample {i}: {ratio:.16e}")
    
    # Manual implementation matching ObsPy
    print(f"\nManual implementation:")
    
    sta = 0.0
    lta = np.finfo(0.0).tiny
    
    csta = 1.0 / sta_samples
    clta = 1.0 / lta_samples
    icsta = 1.0 - csta
    iclta = 1.0 - clta
    
    print(f"Coefficients: csta={csta:.16e}, clta={clta:.16e}")
    print(f"Initial: sta={sta:.16e}, lta={lta:.16e}")
    
    for i, sample in enumerate(data):
        squared_sample = sample * sample
        
        # Recursive calculation
        sta = csta * squared_sample + icsta * sta
        lta = clta * squared_sample + iclta * lta
        
        # Calculate ratio
        if lta > 0:
            manual_ratio = sta / lta
        else:
            manual_ratio = 0.0
        
        print(f"  Sample {i}:")
        print(f"    squared_sample: {squared_sample:.16e}")
        print(f"    sta: {sta:.16e}")
        print(f"    lta: {lta:.16e}")
        print(f"    manual_ratio: {manual_ratio:.16e}")
        print(f"    obspy_ratio: {ratios[i]:.16e}")
        print(f"    difference: {abs(manual_ratio - ratios[i]):.16e}")
        print()

if __name__ == "__main__":
    test_obspy_warmup_behavior()