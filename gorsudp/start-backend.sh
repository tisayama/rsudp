#!/bin/bash

echo "🚀 Starting GoRSUDP Backend Server..."
echo ""

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

# Check if we're in the correct directory
if [ ! -f "cmd/gorsudp/main.go" ]; then
    echo "❌ Please run this script from the gorsudp/gorsudp directory"
    echo "   Current directory: $(pwd)"
    exit 1
fi

# Create config if it doesn't exist
if [ ! -f "config.json" ]; then
    if [ -f "test-config.json" ]; then
        echo "📋 Creating config.json from test-config.json..."
        cp test-config.json config.json
    else
        echo "❌ No configuration file found. Please create config.json or test-config.json"
        exit 1
    fi
fi

echo "⚙️  Configuration: $(pwd)/config.json"
echo "🌐 Backend will start on:"
echo "   - UDP listener: localhost:8888 (for seismic data)"
echo "   - Web server: http://localhost:8080 (API & WebSocket)"
echo ""
echo "🔄 Use Ctrl+C to stop the server"
echo ""

# Start the Go backend
go run cmd/gorsudp/main.go