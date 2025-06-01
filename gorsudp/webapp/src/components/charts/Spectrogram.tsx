'use client'

import { useRef, useEffect, useState, useCallback } from 'react'
import * as d3 from 'd3'
import type { WaveformData, SpectrogramData, ColorScale } from '@/types'
import { LoadingSpinner } from '@/components/ui/LoadingSpinner'
import { NoDataMessage } from '@/components/ui/NoDataMessage'
import { 
  defaultMargin, 
  clearSVG, 
  createGroup, 
  createTimeAxis, 
  createLinearAxis,
  createGridLines,
  getTimeWindow,
  getColorScale
} from '@/utils/d3-utils'

interface SpectrogramProps {
  data: WaveformData[]
  channel: string
  width: number
  height: number
  fftSize?: number
  overlapRatio?: number
  frequencyRange: [number, number]
  timeWindow?: number
  colorScale?: ColorScale
  showGrid?: boolean
}

interface SpectrogramColumn {
  timestamp: number
  frequencies: number[]
  powers: number[]
  normalizedPowers: number[]
  data: Array<{
    frequency: number
    power: number
    normalizedPower: number
  }>
}

export const Spectrogram: React.FC<SpectrogramProps> = ({
  data,
  channel,
  width,
  height,
  fftSize = 256,
  overlapRatio = 0.5,
  frequencyRange,
  timeWindow = 120,
  colorScale = 'viridis',
  showGrid = true
}) => {
  const svgRef = useRef<SVGSVGElement>(null)
  const [spectrogramData, setSpectrogramData] = useState<SpectrogramColumn[]>([])
  const [isProcessing, setIsProcessing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const workerRef = useRef<Worker>()

  // Initialize Web Worker
  useEffect(() => {
    if (typeof window !== 'undefined') {
      workerRef.current = new Worker('/workers/fft-worker.js')
      
      workerRef.current.onmessage = (e) => {
        setIsProcessing(false)
        
        if (e.data.error) {
          console.error('FFT Worker error:', e.data.error)
          setError(`FFT processing failed: ${e.data.error}`)
          return
        }
        
        // Only clear error if there was an error before
        setError(prev => prev ? null : prev)
        
        const result: SpectrogramColumn = e.data
        
        setSpectrogramData(prev => {
          const updated = [...prev, result]
          
          // Keep only recent data within time window
          const cutoffTime = Date.now() - (timeWindow * 1000)
          return updated.filter(col => col.timestamp > cutoffTime)
        })
      }
      
      workerRef.current.onerror = (workerError) => {
        console.error('FFT Worker error:', workerError)
        setError('Web Worker failed to process FFT')
        setIsProcessing(false)
      }
    }
    
    return () => {
      if (workerRef.current) {
        workerRef.current.terminate()
      }
    }
  }, [timeWindow])

  // Process FFT when new data arrives
  const processFFT = useCallback(async (samples: number[], timestamp?: number) => {
    if (!workerRef.current || samples.length < fftSize) {
      return
    }
    
    setIsProcessing(true)
    
    // Pass timestamp if provided, otherwise use current time
    workerRef.current.postMessage({
      samples,
      fftSize,
      sampleRate: 100, // Default sample rate
      frequencyRange,
      timestamp: timestamp || Date.now()
    })
  }, [fftSize, frequencyRange])

  // Track last processed timestamp to avoid reprocessing
  const lastProcessedTimestamp = useRef<number>(0)
  
  // Update spectrogram when new data arrives with overlap
  useEffect(() => {
    if (data.length < fftSize) return
    
    // Calculate step size based on overlap
    const stepSize = Math.floor(fftSize * (1 - overlapRatio))
    
    // Find the latest complete window we haven't processed yet
    const latestDataTimestamp = data[data.length - 1]?.timestamp.getTime() || 0
    
    // Only process if we have new data
    if (latestDataTimestamp <= lastProcessedTimestamp.current) {
      return
    }
    
    // Start from the end and work backwards to find unprocessed windows
    for (let i = data.length - fftSize; i >= 0; i -= stepSize) {
      const windowData = data.slice(i, i + fftSize)
      if (windowData.length === fftSize) {
        // Use the timestamp of the middle of the window
        const middleIndex = Math.floor(windowData.length / 2)
        const windowTimestamp = windowData[middleIndex].timestamp.getTime()
        
        // Process if this window is newer than last processed
        if (windowTimestamp > lastProcessedTimestamp.current) {
          const samples = windowData.map(d => d.value)
          processFFT(samples, windowTimestamp)
          lastProcessedTimestamp.current = windowTimestamp
          break // Process one window per update
        }
      }
    }
  }, [data, fftSize, overlapRatio, processFFT])

  // Render spectrogram
  useEffect(() => {
    if (!svgRef.current || spectrogramData.length === 0) return

    const svg = d3.select(svgRef.current)
    const margin = defaultMargin
    const innerWidth = width - margin.left - margin.right
    const innerHeight = height - margin.top - margin.bottom

    // Clear previous content
    clearSVG(svg)

    // Setup scales - Use same approach as Waveform for consistent time axis
    const now = Date.now()
    const startTime = now - timeWindow * 1000
    
    // Use linear scale with millisecond timestamps for consistency with Waveform
    let timeDomain: [number, number]
    if (spectrogramData.length > 0) {
      const dataExtent = d3.extent(spectrogramData, d => d.timestamp) as [number, number]
      // Use fixed time window to match Waveform behavior
      timeDomain = [
        Math.min(startTime, dataExtent[0]),
        Math.max(now, dataExtent[1])
      ]
    } else {
      timeDomain = [startTime, now]
    }
    
    const xScale = d3.scaleLinear()
      .domain(timeDomain)
      .range([0, innerWidth])

    const yScale = d3.scaleLinear()
      .domain(frequencyRange)
      .range([innerHeight, 0])

    // Get color scale
    const colorScaleFunc = getColorScale(colorScale)
      .domain([0, 1])

    // Create main group
    const mainGroup = createGroup(svg, 'main-group', `translate(${margin.left},${margin.top})`)

    // Create grid lines
    if (showGrid) {
      createGridLines(mainGroup, xScale, { width: innerWidth, height: innerHeight }, 'vertical')
      createGridLines(mainGroup, yScale, { width: innerWidth, height: innerHeight }, 'horizontal')
    }

    // Calculate rectangle dimensions
    const timeStep = spectrogramData.length > 1 
      ? (timeDomain[1] - timeDomain[0]) / (spectrogramData.length - 1) 
      : 1000 // 1 second default
    const rectWidth = Math.max(2, (innerWidth / spectrogramData.length))
    
    // Create rectangles for spectrogram
    const columns = mainGroup.selectAll('.spectrogram-column')
      .data(spectrogramData)
      .enter()
      .append('g')
      .attr('class', 'spectrogram-column')

    columns.each(function(columnData) {
      const column = d3.select(this)
      
      const freqStep = (frequencyRange[1] - frequencyRange[0]) / columnData.data.length
      
      column.selectAll('rect')
        .data(columnData.data)
        .enter()
        .append('rect')
        .attr('x', xScale(columnData.timestamp))
        .attr('y', d => {
          const y = yScale(d.frequency + freqStep)
          return isNaN(y) || !isFinite(y) ? 0 : y
        })
        .attr('width', isNaN(rectWidth) || !isFinite(rectWidth) ? 1 : rectWidth)
        .attr('height', d => {
          const height = yScale(d.frequency) - yScale(d.frequency + freqStep)
          return isNaN(height) || !isFinite(height) ? 1 : Math.max(1, height)
        })
        .attr('fill', d => colorScaleFunc(d.normalizedPower))
        .attr('stroke', 'none')
    })

    // Create axes - Use custom formatter for linear scale
    const xAxis = d3.axisBottom(xScale)
      .tickFormat((d: any) => {
        const date = new Date(d)
        return d3.timeFormat('%H:%M:%S')(date)
      })
    const yAxis = createLinearAxis(yScale, 'left')

    // Add X axis
    mainGroup
      .append('g')
      .attr('class', 'x-axis')
      .attr('transform', `translate(0,${innerHeight})`)
      .call(xAxis)

    // Add Y axis
    mainGroup
      .append('g')
      .attr('class', 'y-axis')
      .call(yAxis)

    // Add axis labels
    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', 'rotate(-90)')
      .attr('y', 0 - margin.left)
      .attr('x', 0 - (innerHeight / 2))
      .attr('dy', '1em')
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', '#666')
      .text('Frequency (Hz)')

    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', `translate(${innerWidth / 2}, ${innerHeight + margin.bottom})`)
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', '#666')
      .text('Time')

  }, [spectrogramData, width, height, frequencyRange, colorScale, showGrid, timeWindow])

  if (error) {
    return (
      <div className="spectrogram-container">
        <div className="chart-header">
          <h3 className="chart-title">
            Spectrogram {channel && `- ${channel}`}
          </h3>
        </div>
        <div className="flex items-center justify-center h-64 text-red-500">
          <div className="text-center">
            <div className="text-2xl mb-2">❌</div>
            <div>Error: {error}</div>
          </div>
        </div>
      </div>
    )
  }

  if (!data.length) {
    return (
      <div className="spectrogram-container">
        <div className="chart-header">
          <h3 className="chart-title">
            Spectrogram {channel && `- ${channel}`}
          </h3>
        </div>
        <NoDataMessage 
          message="No spectrogram data available"
          icon="🌈"
        />
      </div>
    )
  }

  return (
    <div className="spectrogram-container">
      <div className="chart-header">
        <h3 className="chart-title">
          Spectrogram {channel && `- ${channel}`}
        </h3>
        <div className="control-group">
          <span className="control-label">
            FFT Size: {fftSize}
          </span>
          <span className="control-label">
            {frequencyRange[0]}-{frequencyRange[1]} Hz
          </span>
          {isProcessing && (
            <div className="flex items-center space-x-2">
              <LoadingSpinner size="sm" className="text-blue-500" />
              <span className="control-label text-blue-600">
                Processing...
              </span>
            </div>
          )}
        </div>
      </div>
      <svg
        ref={svgRef}
        width={width}
        height={height}
        className="spectrogram-chart"
        style={{ border: '1px solid #e0e0e0' }}
      />
      
      {/* Color scale legend */}
      <div className="mt-2 flex items-center space-x-2 text-sm text-gray-600">
        <span>Power:</span>
        <div className="flex items-center space-x-1">
          <span>Low</span>
          <div 
            className="w-20 h-3 border border-gray-300"
            style={{ 
              background: `linear-gradient(to right, ${getColorScale(colorScale)(0)}, ${getColorScale(colorScale)(1)})` 
            }}
          />
          <span>High</span>
        </div>
      </div>
    </div>
  )
}