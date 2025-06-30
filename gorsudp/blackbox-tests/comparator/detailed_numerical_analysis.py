#!/usr/bin/env python3
"""
Detailed numerical analysis script to identify the root cause of micro-differences
between Python and Go implementations of STA/LTA calculations.

This script focuses on:
1. First 50 samples numerical comparison
2. DC offset removal implementation differences
3. Floating-point precision differences
4. Initialization value discrepancies
5. Data processing order differences
6. Filter implementation differences
"""

import json
import numpy as np
import pandas as pd
from decimal import Decimal, getcontext

# Set high precision for decimal calculations
getcontext().prec = 50

def load_json_data(filepath):
    """Load JSON data from file."""
    with open(filepath, 'r') as f:
        return json.load(f)

def extract_raw_data(data_file):
    """Extract raw amplitude data from the input JSON file."""
    with open(data_file, 'r') as f:
        data = json.load(f)
    
    amplitudes = []
    # Extract from packets array
    if 'packets' in data:
        for packet in data['packets']:
            if 'samples' in packet:
                amplitudes.extend(packet['samples'])
    
    return np.array(amplitudes)

def analyze_dc_offset(raw_data, window_size=3000):
    """Analyze DC offset calculation differences."""
    print(f"\n=== DC Offset Analysis ===")
    print(f"Raw data statistics:")
    print(f"  Mean: {np.mean(raw_data):.15e}")
    print(f"  Median: {np.median(raw_data):.15e}")
    print(f"  Min: {np.min(raw_data):.15e}")
    print(f"  Max: {np.max(raw_data):.15e}")
    print(f"  Std: {np.std(raw_data):.15e}")
    
    # Calculate DC offset using different methods
    dc_mean = np.mean(raw_data[:window_size])
    dc_median = np.median(raw_data[:window_size])
    
    print(f"\nDC offset calculations (first {window_size} samples):")
    print(f"  Mean-based DC offset: {dc_mean:.15e}")
    print(f"  Median-based DC offset: {dc_median:.15e}")
    
    # Calculate detrended data using both methods
    detrended_mean = raw_data - dc_mean
    detrended_median = raw_data - dc_median
    
    print(f"\nDetrended data statistics (mean method):")
    print(f"  Mean: {np.mean(detrended_mean):.15e}")
    print(f"  Std: {np.std(detrended_mean):.15e}")
    
    print(f"\nDetrended data statistics (median method):")
    print(f"  Mean: {np.mean(detrended_median):.15e}")
    print(f"  Std: {np.std(detrended_median):.15e}")
    
    return {
        'dc_mean': dc_mean,
        'dc_median': dc_median,
        'detrended_mean': detrended_mean,
        'detrended_median': detrended_median
    }

def manual_sta_lta_calculation(data, sta_samples=600, lta_samples=3000):
    """Manual STA/LTA calculation for verification."""
    results = []
    
    # Initialize buffers
    sta_buffer = np.zeros(sta_samples)
    lta_buffer = np.zeros(lta_samples)
    
    print(f"\n=== Manual STA/LTA Calculation ===")
    print(f"STA samples: {sta_samples}, LTA samples: {lta_samples}")
    print(f"Data length: {len(data)}")
    
    for i in range(len(data)):
        sample = data[i]
        
        # Update STA buffer (circular)
        sta_buffer[i % sta_samples] = sample * sample
        
        # Update LTA buffer (circular) 
        lta_buffer[i % lta_samples] = sample * sample
        
        # Calculate STA and LTA
        if i >= sta_samples - 1:
            sta_value = np.mean(sta_buffer)
        else:
            sta_value = np.mean(sta_buffer[:i+1])
            
        if i >= lta_samples - 1:
            lta_value = np.mean(lta_buffer)
        else:
            lta_value = np.mean(lta_buffer[:i+1])
        
        # Calculate ratio
        ratio = sta_value / lta_value if lta_value > 0 else 0
        
        results.append({
            'index': i,
            'sample': sample,
            'sta_value': sta_value,
            'lta_value': lta_value,
            'ratio': ratio
        })
        
        # Show first few calculations in detail
        if i < 10:
            print(f"Sample {i:2d}: value={sample:12.6e}, STA={sta_value:12.6e}, LTA={lta_value:12.6e}, ratio={ratio:8.6f}")
    
    return results

def compare_first_n_samples(python_data, go_data, raw_data_file, n=50):
    """Detailed comparison of first n samples."""
    print(f"\n=== Detailed First {n} Samples Analysis ===")
    
    # Load raw data for manual calculation
    raw_data = extract_raw_data(raw_data_file)
    
    # Perform DC offset analysis
    dc_analysis = analyze_dc_offset(raw_data)
    
    # Manual calculation using detrended data
    manual_results = manual_sta_lta_calculation(dc_analysis['detrended_mean'][:5000])
    
    python_results = python_data['results'][:n]
    go_results = go_data['results'][:n]
    
    print(f"\n=== Sample-by-Sample Comparison ===")
    print("Idx | Python Ratio | Go Ratio     | Difference  | Python STA   | Go STA       | Python LTA   | Go LTA       | Manual Ratio")
    print("-" * 140)
    
    max_diff = 0
    max_diff_idx = 0
    
    for i in range(min(n, len(python_results), len(go_results), len(manual_results))):
        py_sample = python_results[i]
        go_sample = go_results[i]
        manual_sample = manual_results[i]
        
        # Extract ratios
        py_ratio = py_sample.get('manual_ratio', py_sample.get('ratio', 0))
        go_ratio = go_sample['ratio']
        manual_ratio = manual_sample['ratio']
        
        # Extract STA/LTA values
        py_sta = py_sample['sta_value']
        go_sta = go_sample['sta_value']
        py_lta = py_sample['lta_value'] 
        go_lta = go_sample['lta_value']
        
        # Calculate differences
        ratio_diff = abs(py_ratio - go_ratio)
        sta_diff = abs(py_sta - go_sta)
        lta_diff = abs(py_lta - go_lta)
        
        if ratio_diff > max_diff:
            max_diff = ratio_diff
            max_diff_idx = i
        
        print(f"{i:3d} | {py_ratio:12.8f} | {go_ratio:12.8f} | {ratio_diff:11.2e} | {py_sta:12.6e} | {go_sta:12.6e} | {py_lta:12.6e} | {go_lta:12.6e} | {manual_ratio:12.8f}")
        
        # Show detailed analysis for first few samples
        if i < 5:
            print(f"     Detailed analysis for sample {i}:")
            print(f"       STA difference: {sta_diff:.2e}")
            print(f"       LTA difference: {lta_diff:.2e}")
            print(f"       Manual vs Python ratio diff: {abs(manual_ratio - py_ratio):.2e}")
            print(f"       Manual vs Go ratio diff: {abs(manual_ratio - go_ratio):.2e}")
            
            # Check manual calculation
            manual_calc_py = py_sta / py_lta if py_lta > 0 else 0
            manual_calc_go = go_sta / go_lta if go_lta > 0 else 0
            print(f"       Manual calc from Python STA/LTA: {manual_calc_py:.8f}")
            print(f"       Manual calc from Go STA/LTA: {manual_calc_go:.8f}")
            print()
    
    print(f"\nMaximum ratio difference: {max_diff:.2e} at index {max_diff_idx}")
    
    return {
        'max_diff': max_diff,
        'max_diff_idx': max_diff_idx,
        'dc_analysis': dc_analysis,
        'manual_results': manual_results
    }

def analyze_floating_point_precision(python_data, go_data, n=50):
    """Analyze floating-point precision differences."""
    print(f"\n=== Floating-Point Precision Analysis ===")
    
    python_results = python_data['results'][:n]
    go_results = go_data['results'][:n]
    
    # Convert to high-precision Decimal for analysis
    python_ratios_decimal = []
    go_ratios_decimal = []
    
    print("High-precision decimal comparison (first 10 samples):")
    for i in range(min(10, len(python_results), len(go_results))):
        py_sample = python_results[i]
        go_sample = go_results[i]
        
        py_ratio = py_sample.get('manual_ratio', py_sample.get('ratio', 0))
        go_ratio = go_sample['ratio']
        
        py_decimal = Decimal(str(py_ratio))
        go_decimal = Decimal(str(go_ratio))
        
        python_ratios_decimal.append(py_decimal)
        go_ratios_decimal.append(go_decimal)
        
        diff_decimal = abs(py_decimal - go_decimal)
        
        print(f"Sample {i:2d}:")
        print(f"  Python: {py_decimal}")
        print(f"  Go:     {go_decimal}")
        print(f"  Diff:   {diff_decimal}")
        print()

def analyze_initialization_differences(python_data, go_data):
    """Analyze initialization value differences."""
    print(f"\n=== Initialization Analysis ===")
    
    # Check first valid STA/LTA calculations
    for i in range(min(20, len(python_data['results']), len(go_data['results']))):
        py_sample = python_data['results'][i]
        go_sample = go_data['results'][i]
        
        py_sta = py_sample['sta_value']
        go_sta = go_sample['sta_value']
        py_lta = py_sample['lta_value']
        go_lta = go_sample['lta_value']
        
        if py_sta > 0 and go_sta > 0 and py_lta > 0 and go_lta > 0:
            print(f"First valid calculation at index {i}:")
            print(f"  Python STA: {py_sta:.15e}")
            print(f"  Go STA:     {go_sta:.15e}")
            print(f"  STA diff:   {abs(py_sta - go_sta):.2e}")
            print(f"  Python LTA: {py_lta:.15e}")
            print(f"  Go LTA:     {go_lta:.15e}")
            print(f"  LTA diff:   {abs(py_lta - go_lta):.2e}")
            break

def analyze_processing_order(python_data, go_data, raw_data_file):
    """Analyze data processing order differences."""
    print(f"\n=== Data Processing Order Analysis ===")
    
    # Load raw data
    raw_data = extract_raw_data(raw_data_file)
    
    # Check if the first few raw samples match the processed data
    print("Raw data vs processed data (first 10 samples):")
    for i in range(min(10, len(raw_data))):
        raw_sample = raw_data[i]
        print(f"Sample {i}: Raw={raw_sample:.6e}")
    
    # Check Python implementation assumptions
    print(f"\nPython implementation check:")
    py_config = python_data['metadata']['config']
    print(f"  STA duration: {py_config['sta_duration']} seconds")
    print(f"  LTA duration: {py_config['lta_duration']} seconds")
    print(f"  Sample rate: {py_config['sample_rate']} Hz")
    print(f"  STA samples: {int(py_config['sta_duration'] * py_config['sample_rate'])}")
    print(f"  LTA samples: {int(py_config['lta_duration'] * py_config['sample_rate'])}")
    
    # Check Go implementation assumptions
    print(f"\nGo implementation check:")
    go_config = go_data['metadata']['config']
    print(f"  STA duration: {go_config['sta_duration']} seconds")
    print(f"  LTA duration: {go_config['lta_duration']} seconds")
    print(f"  Sample rate: {go_config['sample_rate']} Hz")
    print(f"  STA samples: {int(go_config['sta_duration'] * go_config['sample_rate'])}")
    print(f"  LTA samples: {int(go_config['lta_duration'] * go_config['sample_rate'])}")

def main():
    """Main analysis function."""
    print("=== Detailed Numerical Analysis for STA/LTA Implementation Differences ===")
    
    # File paths
    base_path = "/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests"
    python_file = f"{base_path}/python-4min-no-filter.json"
    go_file = f"{base_path}/go-4min-no-filter-fixed.json"  # Use fixed version
    raw_data_file = f"{base_path}/live-production-4min.json"
    
    # Load data
    print("Loading data files...")
    python_data = load_json_data(python_file)
    go_data = load_json_data(go_file)
    
    print(f"Python results: {len(python_data['results'])} samples")
    print(f"Go results: {len(go_data['results'])} samples")
    
    # Perform analyses
    result = compare_first_n_samples(python_data, go_data, raw_data_file, 50)
    
    analyze_floating_point_precision(python_data, go_data, 50)
    
    analyze_initialization_differences(python_data, go_data)
    
    analyze_processing_order(python_data, go_data, raw_data_file)
    
    # Summary
    print(f"\n=== Analysis Summary ===")
    print(f"Maximum ratio difference in first 50 samples: {result['max_diff']:.2e}")
    print(f"This occurs at sample index: {result['max_diff_idx']}")
    
    # Check if differences are within expected floating-point precision
    if result['max_diff'] < 1e-10:
        print("Differences are within expected floating-point precision limits.")
    elif result['max_diff'] < 1e-6:
        print("Differences are small but may indicate algorithmic differences.")
    else:
        print("Differences are significant and indicate implementation discrepancies.")

if __name__ == "__main__":
    main()