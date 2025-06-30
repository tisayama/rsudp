#!/usr/bin/env python3
"""
Investigation script to understand why Python implementation shows ratio=0.0
while manual_ratio shows the correct calculation.
"""

import json

def investigate_python_ratio_issue():
    """Investigate the discrepancy in Python's ratio vs manual_ratio fields."""
    
    print("=== Python Ratio Field Investigation ===\n")
    
    # Load Python output
    with open('/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests/python-output.json', 'r') as f:
        python_data = json.load(f)
    
    # Examine first 20 samples
    print("Sample analysis (first 20 samples):")
    print("Index | STA Value | LTA Value | ratio | manual_ratio | Expected (STA/LTA)")
    print("-" * 80)
    
    ratio_zero_count = 0
    manual_ratio_correct_count = 0
    
    for i in range(20):
        sample = python_data['results'][i]
        sta = sample['sta_value']
        lta = sample['lta_value']
        ratio = sample['ratio']
        manual_ratio = sample['manual_ratio']
        
        expected = sta / lta if lta != 0 else float('inf')
        
        ratio_is_zero = (ratio == 0.0)
        manual_is_correct = abs(manual_ratio - expected) < 1e-10
        
        if ratio_is_zero:
            ratio_zero_count += 1
        if manual_is_correct:
            manual_ratio_correct_count += 1
            
        print(f"{i:5d} | {sta:9.3e} | {lta:9.3e} | {ratio:5.1f} | {manual_ratio:12.6f} | {expected:12.6f}")
    
    print(f"\nSummary for first 20 samples:")
    print(f"- ratio field = 0.0: {ratio_zero_count}/20")
    print(f"- manual_ratio correct: {manual_ratio_correct_count}/20")
    
    # Check if this pattern continues throughout the dataset
    print(f"\n=== Full Dataset Analysis ===")
    
    total_samples = len(python_data['results'])
    ratio_zero_total = 0
    manual_correct_total = 0
    
    for sample in python_data['results']:
        sta = sample['sta_value']
        lta = sample['lta_value']
        ratio = sample['ratio']
        manual_ratio = sample['manual_ratio']
        
        expected = sta / lta if lta != 0 else float('inf')
        
        if ratio == 0.0:
            ratio_zero_total += 1
        if abs(manual_ratio - expected) < 1e-10:
            manual_correct_total += 1
    
    print(f"Total samples: {total_samples}")
    print(f"ratio field = 0.0: {ratio_zero_total}/{total_samples} ({100*ratio_zero_total/total_samples:.1f}%)")
    print(f"manual_ratio correct: {manual_correct_total}/{total_samples} ({100*manual_correct_total/total_samples:.1f}%)")
    
    # Look for any non-zero ratio values
    non_zero_ratios = []
    for i, sample in enumerate(python_data['results']):
        if sample['ratio'] != 0.0:
            non_zero_ratios.append((i, sample['ratio']))
    
    if non_zero_ratios:
        print(f"\nFound {len(non_zero_ratios)} non-zero ratio values:")
        for idx, value in non_zero_ratios[:10]:  # Show first 10
            print(f"  Index {idx}: ratio = {value}")
    else:
        print(f"\nAll ratio values are 0.0 - this appears to be a systematic issue in the Python implementation")

if __name__ == "__main__":
    investigate_python_ratio_issue()