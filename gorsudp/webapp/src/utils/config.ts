/**
 * Configuration utilities for GoRSUDP WebApp
 * Handles environment variables and default values
 */

export interface AppConfig {
  websocketUrl: string
  apiBaseUrl: string
  defaultTimeWindow: number
  defaultFFTSize: number
  defaultSampleRate: number
  enableDebugLogs: boolean
}

/**
 * Get WebSocket URL from environment variables with fallback logic
 */
export const getWebSocketUrl = (): string => {
  // First, try environment variable
  if (process.env.NEXT_PUBLIC_WEBSOCKET_URL) {
    return process.env.NEXT_PUBLIC_WEBSOCKET_URL
  }

  // Fallback to dynamic URL construction
  if (typeof window !== 'undefined') {
    if (process.env.NODE_ENV === 'development') {
      return 'ws://localhost:8080/ws'
    } else {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      return `${protocol}//${window.location.host}/ws`
    }
  }

  return ''
}

/**
 * Get API base URL from environment variables with fallback logic
 */
export const getApiBaseUrl = (): string => {
  // First, try environment variable
  if (process.env.NEXT_PUBLIC_API_BASE_URL) {
    return process.env.NEXT_PUBLIC_API_BASE_URL
  }

  // Fallback to dynamic URL construction
  if (process.env.NODE_ENV === 'development') {
    return 'http://localhost:8080'
  } else {
    return '' // Same origin in production
  }
}

/**
 * Get complete application configuration
 */
export const getAppConfig = (): AppConfig => {
  return {
    websocketUrl: getWebSocketUrl(),
    apiBaseUrl: getApiBaseUrl(),
    defaultTimeWindow: parseInt(process.env.NEXT_PUBLIC_DEFAULT_TIME_WINDOW || '120'),
    defaultFFTSize: parseInt(process.env.NEXT_PUBLIC_DEFAULT_FFT_SIZE || '256'),
    defaultSampleRate: parseInt(process.env.NEXT_PUBLIC_DEFAULT_SAMPLE_RATE || '100'),
    enableDebugLogs: process.env.NEXT_PUBLIC_ENABLE_DEBUG_LOGS === 'true'
  }
}

/**
 * Debug logging utility that respects configuration
 */
export const debugLog = (...args: any[]) => {
  if (getAppConfig().enableDebugLogs) {
    console.log('[GoRSUDP Debug]', ...args)
  }
}

/**
 * Error logging utility
 */
export const errorLog = (...args: any[]) => {
  console.error('[GoRSUDP Error]', ...args)
}