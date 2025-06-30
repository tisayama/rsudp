#!/usr/bin/env python3
"""
Compare STA/LTA results between Python and Go implementations.
Generates detailed comparison report with statistics and visualizations.
"""

import json
import sys
import numpy as np
import matplotlib.pyplot as plt
from datetime import datetime
import argparse

class STALTAComparator:
    def __init__(self, python_file, go_file):
        """Initialize comparator with result files."""
        self.python_data = self._load_json(python_file)
        self.go_data = self._load_json(go_file)
        
        # Extract results
        self.python_results = self.python_data['results']
        self.go_results = self.go_data['results']
        
        # Verify same number of samples
        if len(self.python_results) != len(self.go_results):
            print(f"WARNING: Sample count mismatch - Python: {len(self.python_results)}, Go: {len(self.go_results)}")
            
    def _load_json(self, filename):
        """Load JSON data from file."""
        with open(filename, 'r') as f:
            return json.load(f)
            
    def compare_values(self):
        """Compare STA, LTA, and ratio values between implementations."""
        n_samples = min(len(self.python_results), len(self.go_results))
        
        # Arrays for differences
        sta_diffs = []
        lta_diffs = []
        ratio_diffs = []
        trigger_mismatches = 0
        
        # Compare each sample
        for i in range(n_samples):
            py_result = self.python_results[i]
            go_result = self.go_results[i]
            
            # Skip warmup period where values are 0
            if py_result['ratio'] == 0 or go_result['ratio'] == 0:
                continue
                
            # Calculate differences
            sta_diff = abs(py_result['sta_value'] - go_result['sta_value'])
            lta_diff = abs(py_result['lta_value'] - go_result['lta_value'])
            ratio_diff = abs(py_result['ratio'] - go_result['ratio'])
            
            sta_diffs.append(sta_diff)
            lta_diffs.append(lta_diff)
            ratio_diffs.append(ratio_diff)
            
            # Check trigger mismatch
            if py_result['triggered'] != go_result['triggered']:
                trigger_mismatches += 1
                
        return {
            'sta_diffs': sta_diffs,
            'lta_diffs': lta_diffs,
            'ratio_diffs': ratio_diffs,
            'trigger_mismatches': trigger_mismatches,
            'n_compared': len(sta_diffs)
        }
        
    def generate_statistics(self, diffs):
        """Generate statistics from difference arrays."""
        stats = {}
        
        for key, values in diffs.items():
            if isinstance(values, list) and len(values) > 0:
                stats[f'{key}_min'] = float(np.min(values))
                stats[f'{key}_max'] = float(np.max(values))
                stats[f'{key}_mean'] = float(np.mean(values))
                stats[f'{key}_std'] = float(np.std(values))
                stats[f'{key}_median'] = float(np.median(values))
                
        return stats
        
    def plot_comparison(self, output_prefix='comparison'):
        """Create comparison plots."""
        n_samples = min(len(self.python_results), len(self.go_results))
        
        # Extract time series data
        indices = list(range(n_samples))
        py_ratios = [r['ratio'] for r in self.python_results[:n_samples]]
        go_ratios = [r['ratio'] for r in self.go_results[:n_samples]]
        
        # Create figure with subplots
        fig, axes = plt.subplots(3, 1, figsize=(12, 10))
        
        # Plot 1: STA/LTA Ratios
        ax1 = axes[0]
        ax1.plot(indices, py_ratios, 'b-', label='Python', alpha=0.7)
        ax1.plot(indices, go_ratios, 'r--', label='Go', alpha=0.7)
        ax1.axhline(y=self.python_data['metadata']['config']['threshold'], 
                   color='g', linestyle=':', label='Threshold')
        ax1.axhline(y=self.python_data['metadata']['config']['reset'], 
                   color='orange', linestyle=':', label='Reset')
        ax1.set_ylabel('STA/LTA Ratio')
        ax1.set_title('STA/LTA Ratio Comparison')
        ax1.legend()
        ax1.grid(True, alpha=0.3)
        
        # Plot 2: Difference in Ratios
        ax2 = axes[1]
        ratio_diff = [abs(py - go) for py, go in zip(py_ratios, go_ratios)]
        ax2.plot(indices, ratio_diff, 'k-', alpha=0.7)
        ax2.set_ylabel('|Python - Go|')
        ax2.set_title('Absolute Difference in STA/LTA Ratios')
        ax2.grid(True, alpha=0.3)
        
        # Plot 3: Trigger States
        ax3 = axes[2]
        py_triggers = [1 if r['triggered'] else 0 for r in self.python_results[:n_samples]]
        go_triggers = [1 if r['triggered'] else 0 for r in self.go_results[:n_samples]]
        ax3.fill_between(indices, py_triggers, alpha=0.5, label='Python', step='post')
        ax3.fill_between(indices, go_triggers, alpha=0.5, label='Go', step='post')
        ax3.set_ylabel('Triggered')
        ax3.set_xlabel('Sample Index')
        ax3.set_title('Trigger State Comparison')
        ax3.set_ylim(-0.1, 1.1)
        ax3.legend()
        ax3.grid(True, alpha=0.3)
        
        plt.tight_layout()
        plt.savefig(f'{output_prefix}_plots.png', dpi=150)
        plt.close()
        
        # Create histogram of differences
        fig, ax = plt.subplots(1, 1, figsize=(8, 6))
        
        # Filter out zeros from ratio differences
        nonzero_diffs = [d for d in ratio_diff if d > 0]
        
        if nonzero_diffs:
            ax.hist(nonzero_diffs, bins=50, alpha=0.7, color='purple', edgecolor='black')
            ax.set_xlabel('Absolute Difference in STA/LTA Ratio')
            ax.set_ylabel('Frequency')
            ax.set_title('Distribution of STA/LTA Ratio Differences')
            ax.axvline(x=np.mean(nonzero_diffs), color='red', linestyle='--', 
                      label=f'Mean: {np.mean(nonzero_diffs):.6f}')
            ax.legend()
            ax.grid(True, alpha=0.3)
            
        plt.tight_layout()
        plt.savefig(f'{output_prefix}_histogram.png', dpi=150)
        plt.close()
        
    def generate_report(self):
        """Generate comprehensive comparison report."""
        # Compare values
        diffs = self.compare_values()
        
        # Generate statistics
        stats = self.generate_statistics(diffs)
        
        # Determine pass/fail based on tolerances
        tolerance_ratio = 0.01  # 1% tolerance for ratios
        tolerance_trigger = 0   # No tolerance for trigger mismatches
        
        max_ratio_diff = diffs['ratio_diffs'][np.argmax(diffs['ratio_diffs'])] if diffs['ratio_diffs'] else 0
        mean_ratio_diff = np.mean(diffs['ratio_diffs']) if diffs['ratio_diffs'] else 0
        
        overall_pass = bool(
            mean_ratio_diff < tolerance_ratio and
            diffs['trigger_mismatches'] <= tolerance_trigger
        )
        
        # Build report structure
        report = {
            'metadata': {
                'comparison_timestamp': datetime.now().isoformat() + 'Z',
                'python_implementation': self.python_data['metadata'],
                'go_implementation': self.go_data['metadata']
            },
            'summary': {
                'total_samples': diffs['n_compared'],
                'max_sta_diff': stats.get('sta_diffs_max', 0),
                'max_lta_diff': stats.get('lta_diffs_max', 0),
                'max_ratio_diff': stats.get('ratio_diffs_max', 0),
                'mean_ratio_diff': stats.get('ratio_diffs_mean', 0),
                'trigger_mismatches': diffs['trigger_mismatches'],
                'overall_pass': overall_pass
            },
            'detailed_statistics': stats,
            'tolerances': {
                'ratio_tolerance': tolerance_ratio,
                'trigger_tolerance': tolerance_trigger
            }
        }
        
        return report

def main():
    parser = argparse.ArgumentParser(description='Compare STA/LTA results between implementations')
    parser.add_argument('python_file', help='Python results JSON file')
    parser.add_argument('go_file', help='Go results JSON file')
    parser.add_argument('output_file', help='Output report JSON file')
    parser.add_argument('--plot-prefix', default='comparison', help='Prefix for plot files')
    
    args = parser.parse_args()
    
    print(f"Comparing results...")
    print(f"  Python: {args.python_file}")
    print(f"  Go: {args.go_file}")
    
    # Create comparator
    comparator = STALTAComparator(args.python_file, args.go_file)
    
    # Generate plots
    print("Generating comparison plots...")
    comparator.plot_comparison(args.plot_prefix)
    
    # Generate report
    print("Generating comparison report...")
    report = comparator.generate_report()
    
    # Save report
    with open(args.output_file, 'w') as f:
        json.dump(report, f, indent=2)
    
    print(f"Report saved to: {args.output_file}")
    
    # Print summary
    summary = report['summary']
    print("\n=== Comparison Summary ===")
    print(f"Total samples compared: {summary['total_samples']}")
    print(f"Max STA difference: {summary['max_sta_diff']:.6f}")
    print(f"Max LTA difference: {summary['max_lta_diff']:.6f}")
    print(f"Max ratio difference: {summary['max_ratio_diff']:.6f}")
    print(f"Mean ratio difference: {summary['mean_ratio_diff']:.6f}")
    print(f"Trigger mismatches: {summary['trigger_mismatches']}")
    print(f"Overall result: {'PASS' if summary['overall_pass'] else 'FAIL'}")
    
    # Exit with appropriate code
    sys.exit(0 if summary['overall_pass'] else 1)

if __name__ == "__main__":
    main()