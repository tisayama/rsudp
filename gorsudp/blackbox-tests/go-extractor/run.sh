#!/bin/bash
set -e

echo "=== Go STA/LTA Extractor ==="

# Check if we're in the correct directory  
if [ ! -f "main.go" ]; then
    echo "Error: Please run this script from the go-extractor directory"
    exit 1
fi

# Build the extractor
echo "Building Go STA/LTA extractor..."
go mod tidy
go build -o stalta-extractor main.go

# Run the extractor
echo "Running Go STA/LTA extraction..."
./stalta-extractor ../test-data/sample-packets.json ../go-output.json

echo "Go extraction complete. Output: ../go-output.json"