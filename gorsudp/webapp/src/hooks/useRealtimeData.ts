import { useState, useCallback } from 'react'
import type { ChannelData, WaveformData, PlotDataMessage } from '@/types'

interface UseRealtimeDataReturn {
  channels: Map<string, ChannelData>
  activeChannel: string
  setActiveChannel: (channel: string) => void
  addData: (plotData: PlotDataMessage['data']) => void
  getChannelData: (channel: string) => WaveformData[]
  getChannelList: () => string[]
  clearChannel: (channel: string) => void
  clearAllChannels: () => void
}

export const useRealtimeData = (maxDataPoints = 12000): UseRealtimeDataReturn => {
  const [channels, setChannels] = useState<Map<string, ChannelData>>(new Map())
  const [activeChannel, setActiveChannel] = useState<string>('')

  const addData = useCallback((plotData: PlotDataMessage['data']) => {
    setChannels(prevChannels => {
      // Create a new Map to ensure React detects the change
      const updated = new Map(prevChannels)
      
      // Get existing channel data or create new
      const existingData = updated.get(plotData.channel)
      const channelData = existingData ? {
        ...existingData,
        data: [...existingData.data]  // Create new array to ensure immutability
      } : {
        data: [],
        units: plotData.units,
        sampleRate: plotData.sample_rate
      }

      // Use individual sample timestamps if provided, otherwise calculate them
      const newSamples: WaveformData[] = plotData.samples.map((value, index) => {
        let timestamp: Date
        
        if (plotData.sample_timestamps && plotData.sample_timestamps[index]) {
          // Use precise timestamp provided by the server
          timestamp = new Date(plotData.sample_timestamps[index])
        } else {
          // Fallback: calculate timestamp from base time and sample rate
          const baseTime = new Date(plotData.timestamp)
          const sampleInterval = 1000 / plotData.sample_rate // ms per sample
          timestamp = new Date(baseTime.getTime() + (index * sampleInterval))
        }
        
        return {
          timestamp,
          value
        }
      })

      // Gap detection disabled temporarily for debugging
      // if (channelData.data.length > 0 && newSamples.length > 0) {
      //   const lastExistingTime = channelData.data[channelData.data.length - 1].timestamp.getTime()
      //   const firstNewTime = newSamples[0].timestamp.getTime()
      //   const expectedGap = 1000 / (plotData.sample_rate || 100) // Expected gap in ms
      //   const actualGap = firstNewTime - lastExistingTime
      //   
      //   // If gap is more than 1.5x expected (allowing for some jitter), log warning
      //   if (actualGap > expectedGap * 1.5) {
      //     console.warn(`⚠️ Data gap detected in ${plotData.channel}: ${actualGap}ms (expected ~${expectedGap}ms)`)
      //   }
      // }
      
      // Add new samples to existing data
      channelData.data.push(...newSamples)

      // Keep only recent data points to prevent memory overflow  
      // Always maintain a sliding window of maxDataPoints
      if (channelData.data.length > maxDataPoints) {
        // Keep the most recent data
        channelData.data = channelData.data.slice(-maxDataPoints)
      }

      // Update channel info
      channelData.units = plotData.units
      channelData.sampleRate = plotData.sample_rate

      updated.set(plotData.channel, channelData)
      return updated
    })

    // Set first channel as active if none selected
    if (!activeChannel && plotData.channel) {
      setActiveChannel(plotData.channel)
    }
  }, [maxDataPoints, activeChannel])

  const getChannelData = useCallback((channel: string): WaveformData[] => {
    const channelData = channels.get(channel)
    return channelData?.data || []
  }, [channels])

  const getChannelList = useCallback((): string[] => {
    return Array.from(channels.keys()).sort()
  }, [channels])

  const clearChannel = useCallback((channel: string) => {
    setChannels(prevChannels => {
      const updated = new Map(prevChannels)
      updated.delete(channel)
      return updated
    })

    // If cleared channel was active, switch to another channel or clear active
    if (activeChannel === channel) {
      const remainingChannels = Array.from(channels.keys()).filter(ch => ch !== channel)
      setActiveChannel(remainingChannels.length > 0 ? remainingChannels[0] : '')
    }
  }, [channels, activeChannel])

  const clearAllChannels = useCallback(() => {
    setChannels(new Map())
    setActiveChannel('')
  }, [])

  const handleSetActiveChannel = useCallback((channel: string) => {
    if (channels.has(channel)) {
      setActiveChannel(channel)
    }
  }, [channels])

  return {
    channels,
    activeChannel,
    setActiveChannel: handleSetActiveChannel,
    addData,
    getChannelData,
    getChannelList,
    clearChannel,
    clearAllChannels
  }
}