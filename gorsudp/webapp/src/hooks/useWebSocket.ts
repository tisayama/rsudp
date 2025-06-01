import { useState, useEffect, useCallback, useRef } from 'react'
import { debugLog, errorLog } from '@/utils/config'
import type { ConnectionStatus, WebSocketMessage } from '@/types'

interface UseWebSocketReturn {
  connectionStatus: ConnectionStatus
  lastMessage: WebSocketMessage | null
  sendMessage: (message: any) => void
  reconnect: () => void
}

export const useWebSocket = (url: string): UseWebSocketReturn => {
  const [socket, setSocket] = useState<WebSocket | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected')
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>()
  const pingIntervalRef = useRef<NodeJS.Timeout>()
  const reconnectAttempts = useRef(0)
  const maxReconnectAttempts = 50

  const connect = useCallback(() => {
    if (!url || typeof window === 'undefined') return

    try {
      console.log('🔌 WebSocket Connection Attempt:', {
        url: url,
        timestamp: new Date().toISOString(),
        userAgent: navigator.userAgent,
        location: window.location.href
      })
      debugLog('Attempting WebSocket connection to:', url)
      setConnectionStatus('connecting')
      const ws = new WebSocket(url)

      ws.onopen = () => {
        console.log('✅ WebSocket Connected Successfully:', {
          url: url,
          protocol: ws.protocol,
          extensions: ws.extensions,
          timestamp: new Date().toISOString()
        })
        debugLog('WebSocket connected successfully to:', url)
        setConnectionStatus('connected')
        setSocket(ws)
        reconnectAttempts.current = 0
        
        // Start ping interval to keep connection alive (match server's 54-second interval)
        pingIntervalRef.current = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            debugLog('Sending ping to keep connection alive')
            ws.send(JSON.stringify({ type: 'ping', data: Date.now() }))
          }
        }, 50000) // Send ping every 50 seconds (slightly before server's 54-second timeout)
      }

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as WebSocketMessage
          
          // Handle pong messages (don't propagate to application)
          if (message.type === 'pong') {
            debugLog('Received pong from server')
            return
          }
          
          setLastMessage(message)
        } catch (error) {
          errorLog('Failed to parse WebSocket message:', error)
        }
      }

      ws.onclose = (event) => {
        const closeInfo = {
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
          url: url,
          timestamp: new Date().toISOString()
        }
        
        console.error('❌ WebSocket Connection Closed:', closeInfo)
        
        // Decode close codes for better debugging
        const closeCodeMeaning = {
          1000: 'Normal Closure',
          1001: 'Going Away',
          1002: 'Protocol Error', 
          1003: 'Unsupported Data',
          1005: 'No Status Received',
          1006: 'Abnormal Closure',
          1007: 'Invalid Data',
          1008: 'Policy Violation',
          1009: 'Message Too Big',
          1010: 'Mandatory Extension',
          1011: 'Internal Server Error',
          1015: 'TLS Handshake Error'
        }
        
        console.error(`📋 Close Code ${event.code}: ${closeCodeMeaning[event.code as keyof typeof closeCodeMeaning] || 'Unknown'}`)
        
        debugLog('WebSocket connection closed:', closeInfo)
        setConnectionStatus('disconnected')
        setSocket(null)
        
        // Clear ping interval
        if (pingIntervalRef.current) {
          clearInterval(pingIntervalRef.current)
          pingIntervalRef.current = undefined
        }

        // Auto-reconnect with exponential backoff
        if (reconnectAttempts.current < maxReconnectAttempts) {
          const delay = Math.min(1000 + (reconnectAttempts.current * 500), 5000) // Max 5 second delay
          debugLog(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts.current + 1}/${maxReconnectAttempts})`)
          
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttempts.current++
            connect()
          }, delay)
        } else {
          errorLog('Max reconnection attempts reached')
        }
      }

      ws.onerror = (error) => {
        const readyStateLabels = {
          0: 'CONNECTING',
          1: 'OPEN', 
          2: 'CLOSING',
          3: 'CLOSED'
        }
        
        const errorInfo = {
          error: error,
          url: url,
          readyState: ws.readyState,
          readyStateLabel: readyStateLabels[ws.readyState as keyof typeof readyStateLabels],
          timestamp: new Date().toISOString()
        }
        
        console.error('🔥 WebSocket Error:', errorInfo)
        errorLog('WebSocket error:', errorInfo)
        setConnectionStatus('disconnected')
        
        // Clear ping interval
        if (pingIntervalRef.current) {
          clearInterval(pingIntervalRef.current)
          pingIntervalRef.current = undefined
        }
      }

      return ws
    } catch (error) {
      errorLog('Failed to create WebSocket connection:', error)
      setConnectionStatus('disconnected')
      
      // Retry connection after a delay
      if (reconnectAttempts.current < maxReconnectAttempts) {
        const delay = Math.min(1000 + (reconnectAttempts.current * 500), 5000) // Max 5 second delay
        reconnectTimeoutRef.current = setTimeout(() => {
          reconnectAttempts.current++
          connect()
        }, delay)
      }
    }
  }, [url])

  const sendMessage = useCallback((message: any) => {
    if (socket?.readyState === WebSocket.OPEN) {
      try {
        socket.send(JSON.stringify(message))
      } catch (error) {
        console.error('Failed to send WebSocket message:', error)
      }
    } else {
      console.warn('WebSocket is not connected')
    }
  }, [socket])

  const reconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }
    
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current)
    }
    
    if (socket) {
      socket.close()
    }
    
    reconnectAttempts.current = 0
    connect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connect]) // Remove 'socket' dependency to prevent reconnection loops

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current)
      }
      if (socket) {
        socket.close()
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connect]) // Remove 'socket' from dependencies to prevent reconnection loops

  return {
    connectionStatus,
    lastMessage,
    sendMessage,
    reconnect
  }
}