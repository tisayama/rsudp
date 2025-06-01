// WebSocket message types
export interface WebSocketMessage {
  type: 'plot_data' | 'alert' | 'system_status' | 'config' | 'ping' | 'pong'
  data: any
}

export interface PlotDataMessage {
  type: 'plot_data'
  data: {
    channel: string
    timestamp: string
    samples: number[]
    sample_rate: number
    units: string
    sample_timestamps?: string[]  // Optional: individual timestamps for each sample
  }
}

export interface AlertMessage {
  type: 'alert'
  data: {
    channel: string
    timestamp: string
    stalta_ratio: number
    message: string
  }
}

export interface SystemStatusMessage {
  type: 'system_status'
  data: {
    timestamp: string
    active_channels: string[]
    client_count: number
    packets_received: number
    alerts_triggered: number
    uptime: string
  }
}

export interface ConfigMessage {
  type: 'config'
  data: {
    spectrogram_freq_range: boolean
    lower_limit: number
    upper_limit: number
  }
}

// Chart data types
export interface WaveformData {
  timestamp: Date
  value: number
}

export interface ChannelData {
  data: WaveformData[]
  units: string
  sampleRate: number
}

export interface SpectrogramColumn {
  timestamp: number
  frequencies: number[]
  powers: number[]
}

export interface SpectrogramData {
  time: Date
  data: Array<{
    frequency: number
    power: number
    normalizedPower: number
  }>
}

// Connection status
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected'

// Chart configuration
export interface ChartConfig {
  width: number
  height: number
  margin: {
    top: number
    right: number
    bottom: number
    left: number
  }
}

// Color scale types for spectrogram
export type ColorScale = 'viridis' | 'plasma' | 'inferno' | 'magma'