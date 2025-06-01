#!/bin/bash

echo "🚀 Starting GoRSUDP Frontend Development Server..."
echo ""
echo "Prerequisites:"
echo "1. Go backend should be running on localhost:8080"
echo "2. Node.js 18+ and npm should be installed"
echo ""

# Check if Node.js is available
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

# Check if npm is available
if ! command -v npm &> /dev/null; then
    echo "❌ npm is not installed. Please install npm."
    exit 1
fi

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing dependencies..."
    npm install
    if [ $? -ne 0 ]; then
        echo "❌ Failed to install dependencies"
        exit 1
    fi
fi

# Check if Go backend is running
echo "🔍 Checking if Go backend is running on localhost:8080..."
if curl -s http://localhost:8080/api/status > /dev/null 2>&1; then
    echo "✅ Go backend is running"
else
    echo "⚠️  Go backend is not responding on localhost:8080"
    echo "   Please start the Go backend first:"
    echo "   cd ../.. && go run cmd/gorsudp/main.go"
    echo ""
    echo "   Continuing anyway (frontend will work but show connection errors)..."
fi

echo ""
echo "🎯 Starting Next.js development server..."
echo "   Frontend will be available at: http://localhost:3000"
echo "   Use Ctrl+C to stop the server"
echo ""

npm run dev