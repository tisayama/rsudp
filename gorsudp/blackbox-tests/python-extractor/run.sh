#!/bin/bash
set -e

echo "=== Python STA/LTA Extractor ==="

# Check if we're in the correct directory
if [ ! -f "extract_stalta.py" ]; then
    echo "Error: Please run this script from the python-extractor directory"
    exit 1
fi

# Install requirements if needed
if [ ! -d "venv" ]; then
    echo "Creating Python virtual environment..."
    python3 -m venv venv
fi

echo "Activating virtual environment..."
source venv/bin/activate

echo "Installing requirements..."
pip install -q -r requirements.txt

# Run the extractor
echo "Running Python STA/LTA extraction..."
python extract_stalta.py ../test-data/sample-packets.json ../python-output.json

echo "Python extraction complete. Output: ../python-output.json"