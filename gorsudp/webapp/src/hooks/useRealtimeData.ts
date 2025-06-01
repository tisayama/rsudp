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
    console.log('📊 Adding data for channel:', plotData.channel, 'samples:', plotData.samples.length)
    setChannels(prevChannels => {
      const updated = new Map(prevChannels)
      const channelData = updated.get(plotData.channel) || {
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

      // Add new samples to existing data
      channelData.data.push(...newSamples)

      // Keep only recent data points to prevent memory overflow
      if (channelData.data.length > maxDataPoints) {
        const excessPoints = channelData.data.length - maxDataPoints
        channelData.data.splice(0, excessPoints)
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
    // Return a new array reference to ensure React detects changes
    return channelData?.data ? [...channelData.data] : []
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