'use client'

import { useRef, useEffect, useState } from 'react'
import * as d3 from 'd3'
import type { WaveformData } from '@/types'
import { LoadingSpinner } from '@/components/ui/LoadingSpinner'
import { NoDataMessage } from '@/components/ui/NoDataMessage'
import { 
  defaultMargin, 
  clearSVG, 
  createGroup, 
  createTimeAxis, 
  createSeismicLinearAxis,
  createGridLines,
  getTimeWindow,
  getDataRange,
  removeDC,
  seismicTheme,
  determineOptimalUnit
} from '@/utils/d3-utils'

interface WaveformProps {
  data: WaveformData[]
  channel: string
  width: number
  height: number
  timeWindow?: number // seconds
  showGrid?: boolean
  autoScale?: boolean
  units?: string
}

export const Waveform: React.FC<WaveformProps> = ({
  data,
  channel,
  width,
  height,
  timeWindow = 120,
  showGrid = true,
  autoScale = true,
  units = 'nm/s'
}) => {
  const svgRef = useRef<SVGSVGElement>(null)
  const [scales, setScales] = useState<{
    x: d3.ScaleLinear<number, number>
    y: d3.ScaleLinear<number, number>
  } | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const renderingRef = useRef(false) // Prevent duplicate rendering
  const lastRenderKey = useRef<string>('')
  const mountedRef = useRef(false) // Track component mount state

  // Mount effect to track component lifecycle
  useEffect(() => {
    mountedRef.current = true
    
    return () => {
      mountedRef.current = false
    }
  }, [channel])

  useEffect(() => {
    // Create a unique key for this render based on data content, not just length
    // Use the timestamp of the last data point to ensure updates when data changes
    const lastDataTimestamp = data.length > 0 ? data[data.length - 1].timestamp.getTime() : 0
    const renderKey = `${channel}-${data.length}-${lastDataTimestamp}-${width}-${height}-${timeWindow}`
    
    // Skip rendering if component is not properly mounted
    if (!mountedRef.current) {
      return
    }
    
    // Check if already rendering or same render
    if (renderingRef.current) {
      return
    }
    
    if (renderKey === lastRenderKey.current) {
      return
    }
    
    // Set rendering guard
    renderingRef.current = true
    lastRenderKey.current = renderKey
    
    if (!svgRef.current) {
      renderingRef.current = false
      return
    }
    
    if (!data.length) {
      const svg = d3.select(svgRef.current)
      clearSVG(svg)
      renderingRef.current = false
      return
    }

    try {
      setIsLoading(true)
      setError(null)

      const svg = d3.select(svgRef.current)
      const margin = defaultMargin
      const innerWidth = width - margin.left - margin.right
      const innerHeight = height - margin.top - margin.bottom

      // Completely reset SVG by setting innerHTML (most reliable method)
      svgRef.current.innerHTML = ''
      
      // Set SVG dimensions and stable ID based on channel only
      const svgId = `waveform-svg-${channel}`  // Stable ID based on channel
      svg
        .attr('id', svgId)
        .attr('width', width)
        .attr('height', height)
        .attr('viewBox', `0 0 ${width} ${height}`)
      

    // Get data within time window
    const windowedData = getTimeWindow(data, timeWindow)
    if (windowedData.length === 0) return

    // Remove DC component - data is already in m/s from Go backend
    const processedData = removeDC(windowedData)
    
    if (processedData.length === 0) return
    
    // Determine optimal unit label based on data range (no scaling needed)
    const dataRange = getDataRange(processedData, 0)
    const { unit } = determineOptimalUnit(dataRange)

    // Setup scales - Use linear scale with millisecond timestamps for better precision
    // Always show a fixed time window even if data doesn't fill it completely
    const now = new Date()
    const startTime = new Date(now.getTime() - timeWindow * 1000)
    
    // If we have data, use the actual time range, otherwise use the window
    let timeDomain: [number, number]
    if (processedData.length > 0) {
      const dataExtent = d3.extent(processedData, d => d.timestamp.getTime()) as [number, number]
      // Use the wider range to ensure scrolling works properly
      timeDomain = [
        Math.min(startTime.getTime(), dataExtent[0]),
        Math.max(now.getTime(), dataExtent[1])
      ]
    } else {
      timeDomain = [startTime.getTime(), now.getTime()]
    }
    
    const xScale = d3.scaleLinear()
      .domain(timeDomain)
      .range([0, innerWidth])

    let yDomain: [number, number]
    if (autoScale) {
      yDomain = getDataRange(processedData, 0.1)
    } else {
      // Fixed scale based on typical seismic data range in m/s
      yDomain = [-1e-4, 1e-4]  // ±0.0001 m/s, typical range for seismic background noise
    }

    const yScale = d3.scaleLinear()
      .domain(yDomain)
      .range([innerHeight, 0])

    setScales({ x: xScale, y: yScale })

    // Create main group
    const mainGroup = createGroup(svg, 'main-group', `translate(${margin.left},${margin.top})`)

    // Create grid lines with seismic theme colors
    if (showGrid) {
      const verticalGrid = createGridLines(mainGroup, xScale, { width: innerWidth, height: innerHeight }, 'vertical')
      const horizontalGrid = createGridLines(mainGroup, yScale, { width: innerWidth, height: innerHeight }, 'horizontal')
      
      // Apply seismic theme colors
      verticalGrid.attr('stroke', seismicTheme.gridColor)
      horizontalGrid.attr('stroke', seismicTheme.gridColor)
    }

    // Create axes
    // For linear scale, we need a custom axis formatter to show time
    const xAxis = d3.axisBottom(xScale)
      .tickFormat((d: any) => {
        const date = new Date(d)
        return d3.timeFormat('%H:%M:%S')(date)
      })
    
    const yAxis = createSeismicLinearAxis(yScale, 'left')

    // Add X axis
    const xAxisGroup = mainGroup
      .append('g')
      .attr('class', 'x-axis')
      .attr('transform', `translate(0,${innerHeight})`)
      .call(xAxis)
    
    // Style axis text with seismic theme
    xAxisGroup.selectAll('text')
      .style('fill', seismicTheme.foreground)

    // Add Y axis
    const yAxisGroup = mainGroup
      .append('g')
      .attr('class', 'y-axis')
      .call(yAxis)
    
    // Style axis text with seismic theme
    yAxisGroup.selectAll('text')
      .style('fill', seismicTheme.foreground)

    // Add axis labels with seismic theme colors
    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', 'rotate(-90)')
      .attr('y', 0 - margin.left)
      .attr('x', 0 - (innerHeight / 2))
      .attr('dy', '1em')
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', seismicTheme.foreground)
      .text(`Velocity (${unit})`)

    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', `translate(${innerWidth / 2}, ${innerHeight + margin.bottom})`)
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', seismicTheme.foreground)
      .text('Time')

    // Use processed data directly (already in correct m/s units)
    const validData = processedData
    
    // Create line generator (remove .defined() to prevent unwanted connections)
    const line = d3.line<WaveformData>()
      .x(d => xScale(d.timestamp.getTime()))
      .y(d => yScale(d.value))
      .curve(d3.curveLinear)

    // Generate path data
    const pathData = line(validData)

    const pathElement = mainGroup
      .append('path')
      .datum(validData)
      .attr('class', 'waveform-path')
      .attr('fill', 'none')
      .attr('stroke', seismicTheme.waveformColor)
      .attr('stroke-width', 1.5)
      .attr('d', pathData)
    

    // Add zero line with seismic theme colors
    mainGroup
      .append('line')
      .attr('class', 'zero-line')
      .attr('x1', 0)
      .attr('x2', innerWidth)
      .attr('y1', yScale(0))
      .attr('y2', yScale(0))
      .attr('stroke', seismicTheme.zeroLineColor)
      .attr('stroke-width', 1)
      .attr('stroke-dasharray', '3,3')
      .attr('opacity', 0.5)

      setIsLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to render waveform')
      setIsLoading(false)
    } finally {
      // Always clear rendering guard
      renderingRef.current = false
    }
  }, [data, width, height, timeWindow, showGrid, autoScale, units, channel])

  if (error) {
    return (
      <div className="waveform-container">
        <div className="chart-header">
          <h3 className="chart-title">
            Waveform {channel && `- ${channel}`}
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
      <div className="waveform-container">
        <div className="chart-header">
          <h3 className="chart-title">
            Waveform {channel && `- ${channel}`}
          </h3>
        </div>
        <NoDataMessage 
          message="No waveform data available"
          icon="📈"
        />
      </div>
    )
  }

  return (
    <div className="waveform-container">
      <div className="chart-header">
        <h3 className="chart-title">
          Waveform {channel && `- ${channel}`}
        </h3>
        <div className="control-group">
          {isLoading && <LoadingSpinner size="sm" className="text-blue-500" />}
          <span className="control-label">
            {data.length > 0 ? `${data.length} samples` : 'No data'}
          </span>
        </div>
      </div>
      <svg
        ref={svgRef}
        width={width}
        height={height}
        className="waveform-chart"
        style={{ border: `1px solid ${seismicTheme.gridColor}`, backgroundColor: seismicTheme.background }}
      />
    </div>
  )
}