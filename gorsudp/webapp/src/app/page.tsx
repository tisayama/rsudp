'use client'

import { useEffect, useState } from 'react'
import { Header } from '@/components/layout/Header'
import { StatusBar } from '@/components/layout/StatusBar'
import { ChannelTabs } from '@/components/layout/ChannelTabs'
import { Waveform } from '@/components/charts/Waveform'
import { Spectrogram } from '@/components/charts/Spectrogram'
import { STALTAChart } from '@/components/charts/STALTAChart'
import { AlertsPanel } from '@/components/layout/AlertsPanel'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useRealtimeData } from '@/hooks/useRealtimeData'
import { useChartDimensions } from '@/hooks/useResponsive'
// import { useBatchedData } from '@/hooks/useBatchedData'
import { getWebSocketUrl, getApiBaseUrl, getAppConfig } from '@/utils/config'
import type { 
  PlotDataMessage, 
  AlertMessage, 
  SystemStatusMessage,
  ConfigMessage,
  STALTAMessage,
  STALTAData,
  WebSocketMessage 
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
  const [stalTAData, setSTALTAData] = useState<STALTAData[]>([])
  const [isTriggered, setIsTriggered] = useState(false)

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
  
  // Temporarily disable batching to debug the issue
  // const processBatch = (messages: WebSocketMessage[]) => {
  //   messages.forEach(message => {
  //     switch (message.type) {
  //       case 'plot_data':
  //         const plotData = message as PlotDataMessage
  //         addData(plotData.data)
  //         break
  //         
  //       case 'alert':
  //         const alertData = message as AlertMessage
  //         setAlerts(prev => [alertData.data, ...prev.slice(0, 9)]) // Keep last 10 alerts
  //         break
  //         
  //       case 'system_status':
  //         const statusData = message as SystemStatusMessage
  //         setSystemStatus(statusData.data)
  //         break
  //         
  //       case 'config':
  //         const configData = message as ConfigMessage
  //         setBackendConfig(configData.data)
  //         break
  //     }
  //   })
  // }
  // 
  // const { addData: addToBatch } = useBatchedData<WebSocketMessage>(processBatch, {
  //   batchSize: 5,      // Process every 5 messages
  //   maxDelay: 50       // Or every 50ms, whichever comes first
  // })

  // Handle WebSocket messages (reverted to direct processing)
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
        
      case 'sta_lta_data':
        const stalTAMessage = lastMessage as STALTAMessage
        const newSTALTAData: STALTAData = {
          timestamp: new Date(stalTAMessage.data.timestamp),
          channel: stalTAMessage.data.channel,
          staValue: stalTAMessage.data.sta_value,
          ltaValue: stalTAMessage.data.lta_value,
          ratio: stalTAMessage.data.ratio,
          threshold: stalTAMessage.data.threshold,
          reset: stalTAMessage.data.reset,
          triggered: stalTAMessage.data.triggered
        }
        
        setSTALTAData(prev => {
          const updated = [...prev, newSTALTAData]
          // Keep only last 5 minutes of data (300 points at 1Hz)
          return updated.slice(-300)
        })
        
        setIsTriggered(stalTAMessage.data.triggered)
        break
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lastMessage]) // addData is omitted intentionally as it's stable

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
  // Force re-render when channel data updates by including channels in dependency
  const currentChannelData = activeChannel ? channels.get(activeChannel)?.data || [] : []
  const { chartWidth, chartHeight, isMobile } = useChartDimensions()

  return (
    <div className="min-h-screen bg-gray-50">
      <Header connectionStatus={connectionStatus} isTriggered={isTriggered} />
      
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
          
          <div className="chart-container">
            <div className="w-full overflow-x-auto">
              <STALTAChart
                data={stalTAData}
                width={chartWidth}
                height={chartHeight * 0.7}
                timeWindow={120}
                isTriggered={isTriggered}
              />
            </div>
          </div>
        </div>
        
        <AlertsPanel alerts={alerts} />
      </main>
    </div>
  )
}