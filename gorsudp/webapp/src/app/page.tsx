'use client'

import { useEffect, useState } from 'react'
import { Header } from '@/components/layout/Header'
import { StatusBar } from '@/components/layout/StatusBar'
import { ChannelTabs } from '@/components/layout/ChannelTabs'
import { Waveform } from '@/components/charts/Waveform'
import { Spectrogram } from '@/components/charts/Spectrogram'
import { AlertsPanel } from '@/components/layout/AlertsPanel'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useRealtimeData } from '@/hooks/useRealtimeData'
import { useChartDimensions } from '@/hooks/useResponsive'
import { getWebSocketUrl, getApiBaseUrl, getAppConfig } from '@/utils/config'
import type { 
  PlotDataMessage, 
  AlertMessage, 
  SystemStatusMessage,
  ConfigMessage 
} from '@/types'

export default function Home() {
  const [systemStatus, setSystemStatus] = useState({
    active_channels: [] as string[],
    client_count: 0,
    packets_received: 0,
    alerts_triggered: 0,
    uptime: '0s'
  })
  const [alerts, setAlerts] = useState<AlertMessage['data'][]>([])
  const [backendConfig, setBackendConfig] = useState({
    lower_limit: 0,
    upper_limit: 50,
    spectrogram_freq_range: true
  })

  // Application configuration
  const appConfig = getAppConfig()
  
  // WebSocket connection
  const wsUrl = getWebSocketUrl()
  
  const { connectionStatus, lastMessage } = useWebSocket(wsUrl)
  
  // Real-time data management
  const {
    channels,
    activeChannel,
    setActiveChannel,
    addData,
    getChannelData,
    getChannelList
  } = useRealtimeData()

  // Handle WebSocket messages
  useEffect(() => {
    if (!lastMessage) return

    switch (lastMessage.type) {
      case 'plot_data':
        const plotData = lastMessage as PlotDataMessage
        addData(plotData.data)
        break
        
      case 'alert':
        const alertData = lastMessage as AlertMessage
        setAlerts(prev => [alertData.data, ...prev.slice(0, 9)]) // Keep last 10 alerts
        break
        
      case 'system_status':
        const statusData = lastMessage as SystemStatusMessage
        setSystemStatus(statusData.data)
        break
        
      case 'config':
        const configData = lastMessage as ConfigMessage
        setBackendConfig(configData.data)
        break
    }
  }, [lastMessage, addData])

  // Fetch initial config
  useEffect(() => {
    if (connectionStatus === 'connected') {
      const apiBaseUrl = getApiBaseUrl()
      fetch(`${apiBaseUrl}/api/config`)
        .then(response => response.json())
        .then(data => setBackendConfig(data))
        .catch(error => console.error('Failed to fetch config:', error))
    }
  }, [connectionStatus])

  const channelList = getChannelList()
  const currentChannelData = activeChannel ? getChannelData(activeChannel) : []
  const { chartWidth, chartHeight, isMobile } = useChartDimensions()

  return (
    <div className="min-h-screen bg-gray-50">
      <Header connectionStatus={connectionStatus} />
      
      <main className="container mx-auto px-4 py-6 space-y-6">
        <StatusBar status={systemStatus} />
        
        {channelList.length > 0 && (
          <ChannelTabs
            channels={channelList}
            activeChannel={activeChannel}
            onChannelChange={setActiveChannel}
          />
        )}
        
        <div className="grid grid-cols-1 gap-6">
          <div className="chart-container">
            <div className="w-full overflow-x-auto">
              <Waveform
                data={currentChannelData}
                channel={activeChannel}
                width={chartWidth}
                height={chartHeight}
                timeWindow={appConfig.defaultTimeWindow}
              />
            </div>
          </div>
          
          {/* TEMPORARILY DISABLE Spectrogram for debugging 2-line issue */}
          {/*
          <div className="chart-container">
            <div className="w-full overflow-x-auto">
              <Spectrogram
                data={currentChannelData}
                channel={activeChannel}
                width={chartWidth}
                height={chartHeight}
                fftSize={appConfig.defaultFFTSize}
                overlapRatio={0.5}
                frequencyRange={[backendConfig.lower_limit, backendConfig.upper_limit]}
              />
            </div>
          </div>
          */}
        </div>
        
        <AlertsPanel alerts={alerts} />
      </main>
    </div>
  )
}