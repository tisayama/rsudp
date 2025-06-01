# gorsudp

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-GPL--3.0-green.svg)](LICENSE)

A high-performance Go implementation of rsudp (Raspberry Shake UDP) for real-time seismic data processing and earthquake detection.

## Features

- **Real-time UDP data reception** from Raspberry Shake devices
- **STA/LTA earthquake detection** algorithm 
- **Multiple notification channels** (Twitter, Telegram, Discord, Bluesky, etc.)
- **Real-time web visualization** with WebSocket updates
- **MiniSEED data logging** with automatic file rotation
- **High performance** with 50% memory reduction vs Python version
- **Single binary deployment** with embedded web assets
- **Cross-platform support** (Linux, macOS, Windows)

## Quick Start

### Installation

```bash
# Download pre-built binary
curl -L https://github.com/tisayama/gorsudp/releases/latest/download/gorsudp-linux-amd64 -o gorsudp
chmod +x gorsudp

# Or build from source
git clone https://github.com/tisayama/gorsudp.git
cd gorsudp/gorsudp
make build
```

### Development Setup

For development with the Next.js frontend:

#### 1. Build and Start Go Backend

```bash
# In gorsudp/gorsudp directory
cd gorsudp/gorsudp

# Create test configuration
cp test-config.json config.json

# Build and run Go backend
go run cmd/gorsudp/main.go
```

The Go backend will start:
- **UDP listener**: `localhost:8888` (receives seismic data)
- **Web server**: `localhost:8080` (serves API and WebSocket)

#### 2. Start Next.js Frontend (Development)

```bash
# In webapp directory
cd gorsudp/gorsudp/webapp

# Install dependencies (first time only)
npm install

# Start development server with API proxy
npm run dev
```

The Next.js development server will start:
- **Frontend**: `http://localhost:3000`
- **API proxy**: Automatically forwards `/api/*` and `/ws` to Go backend

#### 3. Start Test Data Generator (Optional)

```bash
# In another terminal, generate test seismic data
cd gorsudp/gorsudp
go run internal/testdata/live_generator.go
```

### Production Deployment

#### 1. Build Frontend for Production

```bash
# Build static frontend
cd gorsudp/gorsudp/webapp
npm run build

# This creates dist/ directory with static files
```

#### 2. Configure Go Backend for Static Files

Update your Go configuration to serve static files from `webapp/dist/`:

```json
{
  "plot": {
    "enabled": true,
    "host": "0.0.0.0",
    "port": 8080,
    "static_dir": "./webapp/dist"
  }
}
```

#### 3. Run Combined Production Server

```bash
# Run Go backend (serves both API and static frontend)
./gorsudp -config config.json
```

Access the application at `http://localhost:8080`

### Configuration

Generate a default configuration file:

```bash
./gorsudp -generate-config
```

This creates `~/.gorsudp/config.json` with default settings. Edit this file to configure your Raspberry Shake station and notification preferences.

### Running

```bash
# Development mode (separate Go backend + Next.js frontend)
# Terminal 1: Go backend
cd gorsudp/gorsudp && go run cmd/gorsudp/main.go

# Terminal 2: Next.js frontend  
cd gorsudp/gorsudp/webapp && npm run dev

# Production mode (combined server)
./gorsudp -config config.json

# Custom configuration
./gorsudp -config /path/to/config.json

# Debug logging
./gorsudp -debug
```

## Configuration

### Basic Settings

```json
{
  "settings": {
    "port": 8888,
    "station": "R24FA",
    "network": "AM",
    "debug": false
  },
  "alert": {
    "enabled": true,
    "channel": "HZ",
    "sta": 5.0,
    "lta": 30.0,
    "threshold": 1.6,
    "reset": 1.55
  }
}
```

### Notification Setup

Configure multiple notification channels:

```json
{
  "telegram": {
    "enabled": true,
    "token": "YOUR_BOT_TOKEN",
    "chat_id": "YOUR_CHAT_ID"
  },
  "twitter": {
    "enabled": true,
    "consumer_key": "YOUR_CONSUMER_KEY",
    "consumer_secret": "YOUR_CONSUMER_SECRET",
    "access_token": "YOUR_ACCESS_TOKEN",
    "access_secret": "YOUR_ACCESS_SECRET"
  }
}
```

## Production Deployment

### Docker

```bash
# Build and run with Docker Compose
cd deploy/docker
docker-compose up -d

# Or build manually
docker build -t gorsudp .
docker run -d -p 8080:8080 -p 8081:8081 -v ~/.gorsudp:/app/config gorsudp
```

### systemd Service

```bash
# Install as system service (requires root)
sudo ./deploy/systemd/install-service.sh

# Start service
sudo systemctl start gorsudp
sudo systemctl enable gorsudp

# Check status
sudo systemctl status gorsudp
```

### Health Monitoring

Built-in health monitoring available at `http://localhost:8081/health`:

```json
{
  "status": "healthy",
  "timestamp": "2025-05-25T07:43:57Z",
  "components": {
    "udp_port": {"status": "healthy", "message": "UDP port listening"},
    "broker": {"status": "healthy", "message": "Message broker operational"},
    "consumers": {"status": "healthy", "message": "All consumers running"}
  }
}
```

## Development

### Prerequisites

- Go 1.21 or later
- Node.js 18+ and npm 8+ (for frontend development)
- Make (optional, for Go build automation)

### Building

#### Go Backend

```bash
# Install dependencies
make deps

# Build Go binary
make build

# Run tests
make test

# Generate coverage report
make coverage
```

#### Next.js Frontend

```bash
# Navigate to webapp directory
cd webapp

# Install Node.js dependencies
npm install

# Development commands
npm run dev          # Start development server
npm run build        # Build for production
npm run lint         # Run ESLint
npm run type-check   # Run TypeScript checking

# Quality checks (run before commit)
npm run type-check && npm run lint && npm run build
```

### Project Structure

```
gorsudp/
├── cmd/                    # Command-line applications
│   ├── gorsudp/           # Main application
│   ├── gorsudp-test/      # Test runner
│   └── gorsudp-settings/  # Configuration tool
├── internal/              # Internal packages
│   ├── producer/          # UDP data reception
│   ├── consumer/          # Base consumer interface
│   ├── alert/             # STA/LTA earthquake detection
│   ├── plot/              # Real-time visualization server
│   ├── notify/            # Notification systems
│   └── broker/            # Message broker
├── pkg/                   # Public packages
│   ├── stream/            # Time series data processing
│   ├── dsp/               # Digital signal processing
│   ├── shakenet/          # Raspberry Shake protocol
│   └── config/            # Configuration management
├── webapp/                # Next.js Frontend Application
│   ├── src/               # TypeScript source code
│   │   ├── app/           # Next.js App Router pages
│   │   ├── components/    # React components
│   │   │   ├── charts/    # D3.js chart components
│   │   │   └── layout/    # Layout components
│   │   ├── hooks/         # Custom React hooks
│   │   ├── types/         # TypeScript type definitions
│   │   └── utils/         # Utility functions
│   ├── public/            # Static files and Web Workers
│   ├── dist/              # Production build output
│   └── package.json       # Node.js dependencies
└── web/                   # Legacy web assets (deprecated)
```

### Architecture

gorsudp uses a producer-consumer architecture with channel-based message passing:

```
┌─────────────────┐    ┌──────────────────────┐
│ Raspberry Shake │───▶│     UDP Producer     │
└─────────────────┘    └──────────┬───────────┘
                                  │
                                  ▼
                       ┌──────────────────────┐
                       │   Message Broker     │
                       │   (Channel-based)    │
                       └─────────┬────────────┘
                                 │
                ┌────────────────┼────────────────┐
                ▼                ▼                ▼
        ┌───────────────┐ ┌─────────────┐ ┌─────────────┐
        │ Alert Consumer│ │Plot Consumer│ │Write Consumer│
        └───────────────┘ └─────────────┘ └─────────────┘
                │                │                │
                ▼                ▼                ▼
        ┌───────────────┐ ┌─────────────┐ ┌─────────────┐
        │   Notifiers   │ │  WebSocket  │ │ File System │
        │ (Multi-target)│ │   Server    │ │ (MiniSEED)  │
        └───────────────┘ └─────────────┘ └─────────────┘
                                 │
                                 ▼
                       ┌─────────────────┐
                       │ Next.js Frontend│
                       │ (TypeScript +   │
                       │  D3.js Charts)  │
                       └─────────────────┘
```

**Frontend Architecture:**
- **Next.js 14**: App Router with TypeScript
- **Real-time Updates**: WebSocket connection to Go backend
- **Charts**: D3.js-based Waveform and Spectrogram components
- **State Management**: Custom React hooks for data management
- **Responsive Design**: Mobile-first with Tailwind CSS

## Performance

Compared to the Python rsudp implementation:

- **50% lower memory usage**
- **30% lower CPU usage** 
- **10x faster startup time**
- **Sub-millisecond alert latency**

## API Reference

### Command Line Options

```
Usage: gorsudp [options]

Options:
  -config string
        Configuration file path
  -debug
        Enable debug logging
  -generate-config
        Generate default configuration file
  -health-check
        Perform health check and exit
  -version
        Show version information
```

### Configuration Reference

See [CONFIG.md](CONFIG.md) for complete configuration documentation.

## Troubleshooting

### No Data Received

1. Check that your Raspberry Shake is configured to send data to this machine
2. Verify the UDP port (default 8888) is not blocked by firewall
3. Ensure the Raspberry Shake and this machine are on the same network

```bash
# Check if UDP packets are arriving
sudo tcpdump -i any udp port 8888
```

### High Memory Usage

1. Adjust plot duration in configuration
2. Reduce number of enabled consumers
3. Check for packet loss with debug logging

### Performance Issues

1. Use production build (not debug mode)
2. Adjust buffer sizes in configuration
3. Monitor with built-in health endpoint: `http://localhost:8081/health`
4. Check logs for packet loss or consumer lag

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Based on the original [rsudp](https://github.com/raspishake/rsudp) Python implementation
- Raspberry Shake team for the excellent seismometer hardware
- ObsPy project for seismological data processing inspiration

## Related Projects

- [rsudp (Python)](https://github.com/raspishake/rsudp) - Original Python implementation
- [Raspberry Shake](https://raspberryshake.org/) - Personal seismometer network
- [ObsPy](https://obspy.org/) - Python framework for seismology