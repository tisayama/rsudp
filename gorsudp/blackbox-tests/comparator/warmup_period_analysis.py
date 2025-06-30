#!/usr/bin/env python3
"""
Analysis of the warmup period behavior in Python vs Go implementations.
"""

import json

def analyze_warmup_transition():
    """Analyze what happens at the warmup period boundary."""
    
    print("=== Warmup Period Transition Analysis ===\n")
    
    # Load both datasets
    with open('/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests/python-output.json', 'r') as f:
        python_data = json.load(f)
    
    with open('/home/tisayama/Development/gorsudp/gorsudp/blackbox-tests/go-output.json', 'r') as f:
        go_data = json.load(f)
    
    # Check the transition around index 3000
    print("Samples around the 3000 mark (warmup period end):")
    print("Index | Python ratio | Python manual | Go ratio | Match?")
    print("-" * 60)
    
    for i in range(2995, 3005):
        py_sample = python_data['results'][i]
        go_sample = go_data['results'][i]
        
        py_ratio = py_sample['ratio']
        py_manual = py_sample['manual_ratio']
        go_ratio = go_sample['ratio']
        
        # Check if Go matches Python manual_ratio
        matches_manual = abs(go_ratio - py_manual) < 1e-10
        
        print(f"{i:5d} | {py_ratio:12.6f} | {py_manual:13.6f} | {go_ratio:8.6f} | {matches_manual}")
    
    print(f"\n=== Key Findings ===")
    
    # Check warmup period length
    warmup_end = 3000
    print(f"Warmup period appears to end at index: {warmup_end}")
    
    # Verify the pattern
    warmup_zeros = sum(1 for i in range(warmup_end) if python_data['results'][i]['ratio'] == 0.0)
    post_warmup_zeros = sum(1 for i in range(warmup_end, len(python_data['results'])) 
                           if python_data['results'][i]['ratio'] == 0.0)
    
    print(f"Python ratio=0.0 in warmup period (0-{warmup_end-1}): {warmup_zeros}/{warmup_end}")
    print(f"Python ratio=0.0 after warmup ({warmup_end}+): {post_warmup_zeros}/{len(python_data['results'])-warmup_end}")
    
    # Check Go consistency
    print(f"\n=== Go Implementation Consistency ===")
    
    # Compare Go ratios with Python manual_ratios throughout
    warmup_matches = 0
    post_warmup_matches = 0
    
    for i in range(len(python_data['results'])):
        py_manual = python_data['results'][i]['manual_ratio']
        go_ratio = go_data['results'][i]['ratio']
        
        if abs(go_ratio - py_manual) < 1e-10:
            if i < warmup_end:
                warmup_matches += 1
            else:
                post_warmup_matches += 1
    
    print(f"Go matches Python manual_ratio in warmup: {warmup_matches}/{warmup_end}")
    print(f"Go matches Python manual_ratio after warmup: {post_warmup_matches}/{len(python_data['results'])-warmup_end}")
    
    # Check if Go matches Python ratio after warmup
    post_warmup_ratio_matches = 0
    for i in range(warmup_end, len(python_data['results'])):
        py_ratio = python_data['results'][i]['ratio']
        go_ratio = go_data['results'][i]['ratio']
        
        if abs(go_ratio - py_ratio) < 1e-10:
            post_warmup_ratio_matches += 1
    
    print(f"Go matches Python ratio after warmup: {post_warmup_ratio_matches}/{len(python_data['results'])-warmup_end}")

if __name__ == "__main__":
    analyze_warmup_transition()