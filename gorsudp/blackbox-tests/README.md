# STA/LTA Blackbox Comparison Tests

This directory contains blackbox tests to compare STA/LTA calculations between Python rsudp and Go gorsudp implementations.

## Directory Structure

- `test-data/` - Common test data (UDP packets, expected results)
- `python-extractor/` - Python implementation STA/LTA extractor
- `go-extractor/` - Go implementation STA/LTA extractor  
- `comparator/` - Result comparison and validation tools

## Test Flow

1. **Generate Test Data**: Create synthetic UDP packets with known characteristics
2. **Python Extraction**: Run Python rsudp STA/LTA calculation on test data
3. **Go Extraction**: Run Go gorsudp STA/LTA calculation on same test data
4. **Comparison**: Compare outputs and generate detailed report

## Usage

```bash
# Run complete test suite
./run-test.sh

# Run individual extractors
cd python-extractor && ./run.sh
cd go-extractor && ./run.sh

# Compare results
cd comparator && python compare.py
```

## Output Format

Both extractors output JSON with this structure:

```json
{
  "metadata": {
    "timestamp": "ISO8601",
    "implementation": "python|go", 
    "config": { "sta_duration": 6.0, "lta_duration": 30.0, ... }
  },
  "results": [
    {
      "sample_index": 0,
      "timestamp": "ISO8601",
      "sta_value": 1234.56,
      "lta_value": 987.65, 
      "ratio": 1.25,
      "triggered": false
    }
  ]
}
```

## Test Data

Test data includes:
- Quiet background noise
- Synthetic earthquake signals
- Mixed noise + signal scenarios
- Edge cases (very small/large values)

## Acceptance Criteria

- STA/LTA ratios match within 1% tolerance
- Trigger points match exactly
- Processing time within reasonable limits