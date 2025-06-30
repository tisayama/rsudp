#!/bin/bash

# Run STA/LTA comparison tests with alert-level data

echo "=== STA/LTA Alert-Level Comparison Tests ==="

# Test datasets
TEST_DATASETS=(
    "alert-test-data.json"
    "strong-earthquake-data.json" 
    "progressive-test-data.json"
)

for dataset in "${TEST_DATASETS[@]}"; do
    echo ""
    echo "=== Testing with $dataset ==="
    
    # Check if test data exists
    if [ ! -f "test-data/$dataset" ]; then
        echo "ERROR: Test data $dataset not found"
        continue
    fi
    
    echo "1. Running Python extractor..."
    cd python-extractor
    source venv/bin/activate
    python extract_stalta.py "../test-data/$dataset" "../python-output-${dataset%.json}.json"
    if [ $? -ne 0 ]; then
        echo "ERROR: Python extraction failed for $dataset"
        cd ..
        continue
    fi
    cd ..
    
    echo "2. Running Go extractor..."
    cd go-extractor
    ./stalta-extractor "../test-data/$dataset" "../go-output-${dataset%.json}.json"
    if [ $? -ne 0 ]; then
        echo "ERROR: Go extraction failed for $dataset"
        cd ..
        continue
    fi
    cd ..
    
    echo "3. Comparing results..."
    cd comparator
    source venv/bin/activate
    python compare.py "../python-output-${dataset%.json}.json" "../go-output-${dataset%.json}.json" "../comparison-report-${dataset%.json}.json"
    if [ $? -ne 0 ]; then
        echo "ERROR: Comparison failed for $dataset"
        cd ..
        continue
    fi
    cd ..
    
    echo "✓ Completed test for $dataset"
done

echo ""
echo "=== Alert-Level Tests Complete ==="
echo "Check comparison-report-*.json files for detailed results"