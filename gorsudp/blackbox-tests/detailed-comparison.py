#!/usr/bin/env python3
"""
Detailed numerical comparison between Python and Go STA/LTA implementations.
This script checks the exact initialization values and compares the first few samples
at the numerical level.
"""

import json
import sys
import numpy as np
from datetime import datetime
import subprocess
import os

def check_numpy_tiny():
    """Check numpy's tiny value for float64."""
    tiny_value = np.finfo(np.float64).tiny
    print(f"np.finfo(np.float64).tiny = {tiny_value}")
    print(f"np.finfo(np.float64).tiny (scientific) = {tiny_value:.16e}")
    print(f"np.finfo(np.float64).tiny (repr) = {repr(tiny_value)}")
    
    # Also check with np.finfo(0.0) as used in the original code
    tiny_value_0 = np.finfo(0.0).tiny
    print(f"np.finfo(0.0).tiny = {tiny_value_0}")
    print(f"np.finfo(0.0).tiny (scientific) = {tiny_value_0:.16e}")
    print(f"np.finfo(0.0).tiny (repr) = {repr(tiny_value_0)}")
    
    # Check if they're equal
    print(f"np.finfo(np.float64).tiny == np.finfo(0.0).tiny: {tiny_value == tiny_value_0}")
    
    # Check against Go's hardcoded value
    go_value = 2.2250738585072014e-308
    print(f"Go hardcoded value: {go_value}")
    print(f"Go value (repr): {repr(go_value)}")
    print(f"Values match: {tiny_value == go_value}")
    
    return tiny_value

def create_test_data():
    """Create simple test data for detailed comparison."""
    # Create simple test data with known values
    test_data = {
        "metadata": {
            "generated_at": datetime.now().isoformat() + "Z",
            "duration_sec": 1,
            "sample_rate": 100,
            "total_samples": 100
        },
        "packets": [
            {
                "channel": "EHZ",
                "timestamp": "2023-01-01T00:00:00Z",
                "sample_rate": 100,
                "samples": [
                    1000, 1100, 900, 1050, 950, 1200, 800, 1150, 850, 1300,
                    700, 1400, 600, 1500, 500, 1600, 400, 1700, 300, 1800,
                    200, 1900, 100, 2000, 0, 2100, -100, 2200, -200, 2300,
                    -300, 2400, -400, 2500, -500, 2600, -600, 2700, -700, 2800,
                    -800, 2900, -900, 3000, -1000, 3100, -1100, 3200, -1200, 3300,
                    -1300, 3400, -1400, 3500, -1500, 3600, -1600, 3700, -1700, 3800,
                    -1800, 3900, -1900, 4000, -2000, 4100, -2100, 4200, -2200, 4300,
                    -2300, 4400, -2400, 4500, -2500, 4600, -2600, 4700, -2700, 4800,
                    -2800, 4900, -2900, 5000, -3000, 5100, -3100, 5200, -3200, 5300,
                    -3300, 5400, -3400, 5500, -3500, 5600, -3600, 5700, -3700, 5800
                ]
            }
        ]
    }
    
    # Save test data
    with open('detailed-test-data.json', 'w') as f:
        json.dump(test_data, f, indent=2)
    
    return test_data

def run_python_extraction():
    """Run Python STA/LTA extraction."""
    cmd = [
        sys.executable, 
        'python-extractor/extract_stalta.py',
        'detailed-test-data.json',
        'python-detailed-output.json'
    ]
    
    print("Running Python extraction...")
    result = subprocess.run(cmd, capture_output=True, text=True)
    
    if result.returncode != 0:
        print(f"Python extraction failed: {result.stderr}")
        return None
    
    print(result.stdout)
    
    # Load and return results
    with open('python-detailed-output.json', 'r') as f:
        return json.load(f)

def run_go_extraction():
    """Run Go STA/LTA extraction."""
    # Build Go extractor if needed
    if not os.path.exists('go-extractor/stalta-extractor'):
        print("Building Go extractor...")
        build_cmd = ['go', 'build', '-o', 'stalta-extractor', 'main.go']
        result = subprocess.run(build_cmd, cwd='go-extractor', capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"Go build failed: {result.stderr}")
            return None
    
    cmd = [
        './go-extractor/stalta-extractor',
        'detailed-test-data.json',
        'go-detailed-output.json'
    ]
    
    print("Running Go extraction...")
    result = subprocess.run(cmd, capture_output=True, text=True)
    
    if result.returncode != 0:
        print(f"Go extraction failed: {result.stderr}")
        return None
    
    print(result.stdout)
    
    # Load and return results
    with open('go-detailed-output.json', 'r') as f:
        return json.load(f)

def compare_detailed_results(python_results, go_results):
    """Compare Python and Go results in detail."""
    print("\n" + "="*80)
    print("DETAILED NUMERICAL COMPARISON")
    print("="*80)
    
    py_results = python_results['results']
    go_results_data = go_results['results']
    
    # Compare first 50 samples in detail
    print(f"\nComparing first 50 samples:")
    print(f"{'Index':<5} {'Py_STA':<15} {'Go_STA':<15} {'STA_Diff':<12} {'Py_LTA':<15} {'Go_LTA':<15} {'LTA_Diff':<12} {'Py_Ratio':<10} {'Go_Ratio':<10} {'Ratio_Diff':<12}")
    print("-" * 130)
    
    max_diff_sta = 0.0
    max_diff_lta = 0.0
    max_diff_ratio = 0.0
    
    for i in range(min(50, len(py_results), len(go_results_data))):
        py_r = py_results[i]
        go_r = go_results_data[i]
        
        py_sta = py_r['sta_value']
        go_sta = go_r['sta_value']
        py_lta = py_r['lta_value']
        go_lta = go_r['lta_value']
        py_ratio = py_r['ratio']
        go_ratio = go_r['ratio']
        
        sta_diff = abs(py_sta - go_sta)
        lta_diff = abs(py_lta - go_lta)
        ratio_diff = abs(py_ratio - go_ratio)
        
        max_diff_sta = max(max_diff_sta, sta_diff)
        max_diff_lta = max(max_diff_lta, lta_diff)
        max_diff_ratio = max(max_diff_ratio, ratio_diff)
        
        print(f"{i:<5} {py_sta:<15.6e} {go_sta:<15.6e} {sta_diff:<12.6e} {py_lta:<15.6e} {go_lta:<15.6e} {lta_diff:<12.6e} {py_ratio:<10.6f} {go_ratio:<10.6f} {ratio_diff:<12.6e}")
        
        # Show detailed comparison for first few samples
        if i < 10:
            print(f"  Sample {i} details:")
            print(f"    Python: STA={py_sta:.16e}, LTA={py_lta:.16e}, Ratio={py_ratio:.16e}")
            print(f"    Go:     STA={go_sta:.16e}, LTA={go_lta:.16e}, Ratio={go_ratio:.16e}")
            print(f"    Diffs:  STA={sta_diff:.16e}, LTA={lta_diff:.16e}, Ratio={ratio_diff:.16e}")
            
            # Check if using manual ratio from Python
            if 'manual_ratio' in py_r:
                manual_ratio = py_r['manual_ratio']
                manual_diff = abs(manual_ratio - go_ratio)
                print(f"    Python manual ratio: {manual_ratio:.16e}, diff from Go: {manual_diff:.16e}")
            print()
    
    print(f"\nMaximum differences:")
    print(f"  STA: {max_diff_sta:.16e}")
    print(f"  LTA: {max_diff_lta:.16e}")
    print(f"  Ratio: {max_diff_ratio:.16e}")
    
    # Check initialization values
    print(f"\nInitialization values check:")
    print(f"  Python LTA init (from numpy): {np.finfo(0.0).tiny:.16e}")
    print(f"  Go LTA init (hardcoded): {2.2250738585072014e-308:.16e}")
    print(f"  Values match: {np.finfo(0.0).tiny == 2.2250738585072014e-308}")
    
    # Check if first LTA values match the initialization
    if len(py_results) > 0 and len(go_results_data) > 0:
        print(f"  First Python LTA: {py_results[0]['lta_value']:.16e}")
        print(f"  First Go LTA: {go_results_data[0]['lta_value']:.16e}")
    
    return {
        'max_diff_sta': max_diff_sta,
        'max_diff_lta': max_diff_lta,
        'max_diff_ratio': max_diff_ratio,
        'initialization_match': np.finfo(0.0).tiny == 2.2250738585072014e-308
    }

def main():
    """Main function."""
    print("Detailed STA/LTA Comparison")
    print("="*50)
    
    # Check numpy tiny value
    print("\n1. Checking numpy tiny value...")
    tiny_value = check_numpy_tiny()
    
    # Create test data
    print("\n2. Creating test data...")
    test_data = create_test_data()
    print(f"Created test data with {test_data['metadata']['total_samples']} samples")
    
    # Run Python extraction
    print("\n3. Running Python extraction...")
    python_results = run_python_extraction()
    if python_results is None:
        print("Python extraction failed")
        return 1
    
    # Run Go extraction
    print("\n4. Running Go extraction...")
    go_results = run_go_extraction()
    if go_results is None:
        print("Go extraction failed")
        return 1
    
    # Compare results
    print("\n5. Comparing results...")
    comparison = compare_detailed_results(python_results, go_results)
    
    # Summary
    print(f"\n" + "="*80)
    print("SUMMARY")
    print("="*80)
    print(f"Initialization values match: {comparison['initialization_match']}")
    print(f"Maximum STA difference: {comparison['max_diff_sta']:.16e}")
    print(f"Maximum LTA difference: {comparison['max_diff_lta']:.16e}")
    print(f"Maximum Ratio difference: {comparison['max_diff_ratio']:.16e}")
    
    # Check for significant differences
    tolerance = 1e-10
    if comparison['max_diff_sta'] > tolerance:
        print(f"WARNING: STA differences exceed tolerance ({tolerance})")
    if comparison['max_diff_lta'] > tolerance:
        print(f"WARNING: LTA differences exceed tolerance ({tolerance})")
    if comparison['max_diff_ratio'] > tolerance:
        print(f"WARNING: Ratio differences exceed tolerance ({tolerance})")
    
    if not comparison['initialization_match']:
        print("ERROR: Initialization values do not match!")
        return 1
    
    print("Detailed comparison completed successfully!")
    return 0

if __name__ == "__main__":
    sys.exit(main())