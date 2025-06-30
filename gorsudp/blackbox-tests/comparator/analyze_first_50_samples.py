#!/usr/bin/env python3
"""
Analysis script to compare the first 50 samples between Python and Go implementations
of STA/LTA calculations to identify why Go shows anomalous maximum values.
"""

import json
import numpy as np
import pandas as pd

def load_output_file(filename):
    """Load JSON output file and return the results."""
    with open(filename, 'r') as f:
        data = json.load(f)
    return data

def extract_first_n_samples(data, n=50):
    """Extract the first n samples from the results."""
    return data['results'][:n]

def analyze_samples(python_data, go_data, n=50):
    """Detailed analysis of the first n samples."""
    
    python_samples = extract_first_n_samples(python_data, n)
    go_samples = extract_first_n_samples(go_data, n)
    
    print(f"=== Analysis of First {n} Samples ===\n")
    
    # Extract values for comparison
    python_ratios = []
    go_ratios = []
    python_stas = []
    go_stas = []
    python_ltas = []
    go_ltas = []
    
    print("Sample-by-sample comparison:")
    print("Index | Python Ratio | Go Ratio | Diff | Python STA | Go STA | Python LTA | Go LTA")
    print("-" * 95)
    
    for i in range(min(n, len(python_samples), len(go_samples))):
        py_sample = python_samples[i]
        go_sample = go_samples[i]
        
        # Python uses manual_ratio, Go uses ratio
        py_ratio = py_sample.get('manual_ratio', py_sample.get('ratio', 0))
        go_ratio = go_sample['ratio']
        
        py_sta = py_sample['sta_value']
        go_sta = go_sample['sta_value']
        py_lta = py_sample['lta_value']
        go_lta = go_sample['lta_value']
        
        python_ratios.append(py_ratio)
        go_ratios.append(go_ratio)
        python_stas.append(py_sta)
        go_stas.append(go_sta)
        python_ltas.append(py_lta)
        go_ltas.append(go_lta)
        
        diff = abs(py_ratio - go_ratio)
        
        print(f"{i:5d} | {py_ratio:12.6f} | {go_ratio:8.6f} | {diff:4.2e} | {py_sta:10.3e} | {go_sta:10.3e} | {py_lta:10.3e} | {go_lta:10.3e}")
    
    # Statistical analysis
    python_ratios = np.array(python_ratios)
    go_ratios = np.array(go_ratios)
    
    print(f"\n=== Statistical Summary ===")
    print(f"Python ratios: min={python_ratios.min():.6f}, max={python_ratios.max():.6f}, mean={python_ratios.mean():.6f}")
    print(f"Go ratios:     min={go_ratios.min():.6f}, max={go_ratios.max():.6f}, mean={go_ratios.mean():.6f}")
    
    diff_ratios = np.abs(python_ratios - go_ratios)
    print(f"Differences:   min={diff_ratios.min():.2e}, max={diff_ratios.max():.2e}, mean={diff_ratios.mean():.2e}")
    
    # Check for identical values
    identical_count = np.sum(diff_ratios < 1e-10)
    print(f"Identical values (diff < 1e-10): {identical_count}/{len(diff_ratios)}")
    
    # Look at the actual ratio calculation issue
    print(f"\n=== Detailed First Sample Analysis ===")
    py_first = python_samples[0]
    go_first = go_samples[0]
    
    print(f"Sample 0:")
    print(f"  Python - STA: {py_first['sta_value']:.15e}, LTA: {py_first['lta_value']:.15e}")
    print(f"  Go     - STA: {go_first['sta_value']:.15e}, LTA: {go_first['lta_value']:.15e}")
    print(f"  Python - ratio: {py_first.get('ratio', 'N/A')}, manual_ratio: {py_first.get('manual_ratio', 'N/A')}")
    print(f"  Go     - ratio: {go_first['ratio']}")
    
    # Manual calculation
    py_manual_calc = py_first['sta_value'] / py_first['lta_value'] if py_first['lta_value'] != 0 else 0
    go_manual_calc = go_first['sta_value'] / go_first['lta_value'] if go_first['lta_value'] != 0 else 0
    
    print(f"  Manual calculation - Python: {py_manual_calc:.15f}")
    print(f"  Manual calculation - Go:     {go_manual_calc:.15f}")
    
    return {
        'python_ratios': python_ratios,
        'go_ratios': go_ratios,
        'differences': diff_ratios,
        'identical_count': identical_count,
        'python_samples': python_samples,
        'go_samples': go_samples
    }

def analyze_warmup_period(python_data, go_data, warmup_samples=3000):
    """Analyze the warmup period to understand initialization behavior."""
    
    print(f"\n=== Warmup Period Analysis (First {warmup_samples} samples) ===")
    
    python_results = python_data['results'][:warmup_samples]
    go_results = go_data['results'][:warmup_samples]
    
    # Extract ratios for warmup period
    python_ratios = []
    go_ratios = []
    
    for i in range(min(warmup_samples, len(python_results), len(go_results))):
        py_ratio = python_results[i].get('manual_ratio', python_results[i].get('ratio', 0))
        go_ratio = go_results[i]['ratio']
        python_ratios.append(py_ratio)
        go_ratios.append(go_ratio)
    
    python_ratios = np.array(python_ratios)
    go_ratios = np.array(go_ratios)
    
    print(f"Warmup period statistics:")
    print(f"Python - min: {python_ratios.min():.6f}, max: {python_ratios.max():.6f}, mean: {python_ratios.mean():.6f}")
    print(f"Go     - min: {go_ratios.min():.6f}, max: {go_ratios.max():.6f}, mean: {go_ratios.mean():.6f}")
    
    # Find where Go shows the maximum value
    go_max_idx = np.argmax(go_ratios)
    print(f"\nGo maximum ratio occurs at index: {go_max_idx}")
    print(f"Go maximum ratio value: {go_ratios[go_max_idx]:.6f}")
    print(f"Python ratio at same index: {python_ratios[go_max_idx]:.6f}")
    
    # Check for any values above threshold
    threshold = 3.95
    python_above_threshold = np.sum(python_ratios > threshold)
    go_above_threshold = np.sum(go_ratios > threshold)
    
    print(f"\nValues above threshold ({threshold}):")
    print(f"Python: {python_above_threshold} samples")
    print(f"Go: {go_above_threshold} samples")
    
    if go_above_threshold > 0:
        go_trigger_indices = np.where(go_ratios > threshold)[0]
        print(f"Go trigger indices: {go_trigger_indices[:10]}...")  # Show first 10
        
    return {
        'python_warmup_ratios': python_ratios,
        'go_warmup_ratios': go_ratios,
        'go_max_index': go_max_idx,
        'go_max_value': go_ratios[go_max_idx],
        'python_above_threshold': python_above_threshold,
        'go_above_threshold': go_above_threshold
    }

def main():
    """Main analysis function."""
    
    # Load the data files
    python_file = '/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests/python-output.json'
    go_file = '/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests/go-output.json'
    
    print("Loading data files...")
    python_data = load_output_file(python_file)
    go_data = load_output_file(go_file)
    
    print(f"Python implementation: {len(python_data['results'])} samples")
    print(f"Go implementation: {len(go_data['results'])} samples")
    
    # Analyze first 50 samples
    first_50_analysis = analyze_samples(python_data, go_data, 50)
    
    # Analyze warmup period
    warmup_analysis = analyze_warmup_period(python_data, go_data, 3000)
    
    # Look for the specific issue mentioned
    print(f"\n=== Issue Investigation: Go Max Value 5.000 ===")
    
    go_ratios_all = [result['ratio'] for result in go_data['results']]
    go_ratios_array = np.array(go_ratios_all)
    
    # Find all occurrences of values close to 5.0
    close_to_5 = np.where(np.abs(go_ratios_array - 5.0) < 0.001)[0]
    print(f"Samples with ratio close to 5.0: {len(close_to_5)}")
    
    if len(close_to_5) > 0:
        print(f"First few indices with ratio ~5.0: {close_to_5[:10]}")
        for idx in close_to_5[:5]:  # Show first 5
            go_sample = go_data['results'][idx]
            python_sample = python_data['results'][idx]
            py_ratio = python_sample.get('manual_ratio', python_sample.get('ratio', 0))
            
            print(f"  Index {idx}: Go={go_sample['ratio']:.6f}, Python={py_ratio:.6f}")
            print(f"    Go STA/LTA: {go_sample['sta_value']:.6e} / {go_sample['lta_value']:.6e}")
            print(f"    Python STA/LTA: {python_sample['sta_value']:.6e} / {python_sample['lta_value']:.6e}")

if __name__ == "__main__":
    main()