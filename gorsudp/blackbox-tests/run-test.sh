#!/bin/bash
set -e

# STA/LTA Blackbox Comparison Test Runner
echo "=== STA/LTA Blackbox Comparison Tests ==="

# Check if we're in the correct directory
if [ ! -f "README.md" ]; then
    echo "Error: Please run this script from the blackbox-tests directory"
    exit 1
fi

# Configuration
TEST_DATA_FILE="test-data/sample-packets.json"
PYTHON_OUTPUT="python-output.json"
GO_OUTPUT="go-output.json"
REPORT_OUTPUT="comparison-report.json"

echo "1. Checking test data..."
if [ ! -f "$TEST_DATA_FILE" ]; then
    echo "Generating test data..."
    # TODO: Implement test data generator
    echo "Test data generation not yet implemented"
    exit 1
fi

echo "2. Running Python extractor..."
cd python-extractor
if [ ! -f "extract_stalta.py" ]; then
    echo "Error: Python extractor not found"
    exit 1
fi
./run.sh
cd ..

echo "3. Running Go extractor..."
cd go-extractor  
if [ ! -f "main.go" ]; then
    echo "Error: Go extractor not found"
    exit 1
fi
./run.sh
cd ..

echo "4. Comparing results..."
cd comparator
python compare.py ../$PYTHON_OUTPUT ../$GO_OUTPUT ../$REPORT_OUTPUT
cd ..

echo "5. Test complete. Results in: $REPORT_OUTPUT"

# Display summary
if [ -f "$REPORT_OUTPUT" ]; then
    echo "=== Test Summary ==="
    python -c "
import json
with open('$REPORT_OUTPUT', 'r') as f:
    report = json.load(f)
    summary = report.get('summary', {})
    print(f\"Total samples: {summary.get('total_samples', 0)}\")
    print(f\"Max STA difference: {summary.get('max_sta_diff', 0):.6f}\")
    print(f\"Max LTA difference: {summary.get('max_lta_diff', 0):.6f}\")
    print(f\"Max ratio difference: {summary.get('max_ratio_diff', 0):.6f}\")
    print(f\"Trigger mismatches: {summary.get('trigger_mismatches', 0)}\")
    print(f\"Test result: {'PASS' if summary.get('overall_pass', False) else 'FAIL'}\")
"
fi