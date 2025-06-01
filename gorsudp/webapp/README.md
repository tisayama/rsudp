# GoRSUDP WebApp

Next.js frontend application for GoRSUDP real-time seismic monitoring system.

## Quick Start

### Prerequisites

- Node.js 18+
- npm 8+
- Go backend running on `localhost:8080`

### Development Setup

```bash
# Install dependencies
npm install

# Copy environment configuration (optional)
cp .env.example .env.local

# Edit configuration as needed
# nano .env.local

# Start development server
npm run dev

# Open browser
open http://localhost:3000
```

### Production Build

```bash
# Build static files for production
npm run build

# Output will be in dist/ directory
ls dist/
```

## Technology Stack

- **Framework**: Next.js 14+ (App Router)
- **Language**: TypeScript 5+
- **Styling**: Tailwind CSS 3+
- **Visualization**: D3.js 7+
- **State Management**: Custom hooks with React state
- **Build Tool**: Next.js built-in (Webpack/Turbopack)

## Project Structure

```
src/
├── app/                    # Next.js App Router pages
│   ├── globals.css        # Global styles with Tailwind
│   ├── layout.tsx         # Root layout component
│   └── page.tsx           # Main dashboard page
├── components/            # React components
│   ├── charts/           # D3.js chart components
│   │   ├── Waveform.tsx      # Seismic waveform display
│   │   └── Spectrogram.tsx   # Frequency spectrogram
│   └── layout/           # Layout components
│       ├── Header.tsx        # Application header
│       ├── StatusBar.tsx     # System status display
│       ├── ChannelTabs.tsx   # Channel selection
│       └── AlertsPanel.tsx   # Alert notifications
├── hooks/                # Custom React hooks
│   ├── useWebSocket.ts   # WebSocket connection management
│   └── useRealtimeData.ts # Real-time data management
├── types/                # TypeScript type definitions
│   └── index.ts          # Common types and interfaces
└── utils/                # Utility functions
    └── d3-utils.ts       # D3.js helper functions
```

## Features

### Real-time Data Visualization

- **Waveform Display**: Real-time seismic waveform with auto-scaling
- **Spectrogram**: Frequency domain analysis with customizable FFT
- **Multi-channel Support**: Switch between different seismic channels
- **Time Window Control**: Configurable data display window

### WebSocket Integration

- **Auto-reconnection**: Exponential backoff reconnection strategy
- **Message Handling**: Structured message parsing for different data types
- **Connection Status**: Visual connection status indicator

### Chart Components

- **Modular Design**: Separate Waveform and Spectrogram components
- **D3.js Integration**: High-performance SVG-based charts
- **Web Workers**: FFT processing in background threads
- **Responsive Layout**: Adaptive chart sizing

### Alert System

- **Real-time Alerts**: STA/LTA earthquake detection alerts
- **Severity Levels**: Color-coded alert severity based on trigger ratio
- **Alert History**: Display of recent earthquake events

## Development

### Prerequisites

- Node.js 18+
- npm 8+

### Setup

1. Install dependencies:
```bash
npm install
```

2. Development server:
```bash
npm run dev
```

3. Type checking:
```bash
npm run type-check
```

4. Linting:
```bash
npm run lint
```

5. Production build:
```bash
npm run build
```

### Development Rules

**Pre-commit Requirements**:
```bash
npm run type-check    # TypeScript validation
npm run lint          # ESLint validation
npm run build         # Production build test
```

**Important Guidelines**:
- Zero TypeScript errors allowed
- All ESLint warnings must be resolved
- Production build must succeed
- No direct `package-lock.json` modifications

## Configuration

### Environment Variables

The application supports configuration via environment variables:

```bash
# Copy example configuration
cp .env.example .env.local
```

**Available Configuration Options:**

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `NEXT_PUBLIC_WEBSOCKET_URL` | WebSocket endpoint URL | `ws://localhost:8080/ws` | `wss://your-domain.com/ws` |
| `NEXT_PUBLIC_API_BASE_URL` | API base URL | `http://localhost:8080` | `https://your-domain.com` |
| `NEXT_PUBLIC_DEFAULT_TIME_WINDOW` | Chart time window (seconds) | `120` | `60` |
| `NEXT_PUBLIC_DEFAULT_FFT_SIZE` | FFT size for spectrogram | `256` | `512` |
| `NEXT_PUBLIC_DEFAULT_SAMPLE_RATE` | Sample rate (Hz) | `100` | `125` |
| `NEXT_PUBLIC_ENABLE_DEBUG_LOGS` | Enable debug logging | `true` | `false` |

**Production Configuration:**
```bash
# Production .env.local example
NEXT_PUBLIC_WEBSOCKET_URL=wss://your-domain.com/ws
NEXT_PUBLIC_API_BASE_URL=https://your-domain.com
NEXT_PUBLIC_ENABLE_DEBUG_LOGS=false
```

### API Integration

The application connects to the Go backend via:
- **WebSocket**: Configured via `NEXT_PUBLIC_WEBSOCKET_URL`
- **REST API**: Configured via `NEXT_PUBLIC_API_BASE_URL`

### Chart Configuration

Chart settings are configurable via environment variables:
- Time window: `NEXT_PUBLIC_DEFAULT_TIME_WINDOW` (default: 120 seconds)
- FFT size: `NEXT_PUBLIC_DEFAULT_FFT_SIZE` (default: 256 samples)  
- Sample rate: `NEXT_PUBLIC_DEFAULT_SAMPLE_RATE` (default: 100 Hz)
- Frequency range: Configured via Go backend API

## Deployment

### Static Export

The app is configured for static export to integrate with Go backend:

```bash
npm run build
```

Output directory: `dist/` (configured in `next.config.js`)

### Docker Integration

The build output can be served by the Go backend or as static files.

## WebSocket Message Format

### Plot Data
```typescript
{
  type: 'plot_data',
  data: {
    channel: string,
    timestamp: string,
    samples: number[],
    sample_rate: number,
    units: string
  }
}
```

### Alert Data
```typescript
{
  type: 'alert',
  data: {
    channel: string,
    timestamp: string,
    stalta_ratio: number,
    message: string
  }
}
```

### System Status
```typescript
{
  type: 'system_status',
  data: {
    active_channels: string[],
    client_count: number,
    packets_received: number,
    alerts_triggered: number,
    uptime: string
  }
}
```

## Performance Optimizations

- **Web Workers**: FFT calculations run in background threads
- **Data Throttling**: Configurable data point limits to prevent memory overflow
- **Efficient Rendering**: D3.js optimizations for smooth real-time updates
- **Component Memoization**: React optimizations for chart re-rendering

## Browser Support

- Chrome 90+
- Firefox 88+
- Safari 14+
- Edge 90+

Requires WebSocket and Web Workers support.