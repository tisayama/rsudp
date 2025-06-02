'use client'

import type { ConnectionStatus } from '@/types'

interface HeaderProps {
  connectionStatus: ConnectionStatus
  isTriggered?: boolean
}

export const Header: React.FC<HeaderProps> = ({ connectionStatus, isTriggered = false }) => {
  const getStatusColor = (status: ConnectionStatus) => {
    switch (status) {
      case 'connected':
        return 'bg-green-100 text-green-800 border-green-200'
      case 'connecting':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200'
      case 'disconnected':
        return 'bg-red-100 text-red-800 border-red-200'
      default:
        return 'bg-gray-100 text-gray-800 border-gray-200'
    }
  }

  const getStatusText = (status: ConnectionStatus) => {
    switch (status) {
      case 'connected':
        return 'Connected to server'
      case 'connecting':
        return 'Connecting to server...'
      case 'disconnected':
        return 'Disconnected from server'
      default:
        return 'Unknown status'
    }
  }

  return (
    <header className={`border-b border-gray-200 shadow-sm transition-colors duration-300 ${
      isTriggered ? 'bg-red-50' : 'bg-white'
    }`}>
      <div className="container mx-auto px-4 py-4">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div>
            <h1 className="text-xl lg:text-2xl font-bold text-gray-900">
              GoRSUDP Real-time Seismic Monitoring
              {isTriggered && (
                <span className="ml-3 px-2 py-1 text-sm font-bold bg-red-500 text-white rounded-lg animate-pulse">
                  🚨 EARTHQUAKE ALERT
                </span>
              )}
            </h1>
            <p className="text-sm text-gray-600 mt-1">
              Real-time seismic data visualization and monitoring system
            </p>
          </div>
          
          <div className="flex items-center gap-3">
            {isTriggered && (
              <div className="px-3 py-2 rounded-lg border bg-red-100 text-red-800 border-red-200">
                <div className="flex items-center space-x-2">
                  <div className="w-3 h-3 rounded-full bg-red-500 animate-pulse" />
                  <span className="text-sm font-bold">
                    ALERT ACTIVE
                  </span>
                </div>
              </div>
            )}
            
            <div className={`px-3 py-2 rounded-lg border ${getStatusColor(connectionStatus)}`}>
              <div className="flex items-center space-x-2">
                <div className={`w-2 h-2 rounded-full ${
                  connectionStatus === 'connected' ? 'bg-green-500' :
                  connectionStatus === 'connecting' ? 'bg-yellow-500 animate-pulse' :
                  'bg-red-500'
                }`} />
                <span className="text-sm font-medium">
                  {getStatusText(connectionStatus)}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </header>
  )
}