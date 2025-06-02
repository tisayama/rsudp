'use client'

import { useRef, useEffect, useState } from 'react'
import * as d3 from 'd3'
import type { STALTAData } from '@/types'
import { LoadingSpinner } from '@/components/ui/LoadingSpinner'
import { NoDataMessage } from '@/components/ui/NoDataMessage'
import { 
  defaultMargin, 
  clearSVG, 
  createGroup, 
  createLinearAxis,
  createGridLines
} from '@/utils/d3-utils'

interface STALTAChartProps {
  data: STALTAData[]
  width: number
  height: number
  timeWindow?: number
  showGrid?: boolean
  isTriggered?: boolean
}

export const STALTAChart: React.FC<STALTAChartProps> = ({
  data,
  width,
  height,
  timeWindow = 120,
  showGrid = true,
  isTriggered = false
}) => {
  const svgRef = useRef<SVGSVGElement>(null)
  const [isProcessing, setIsProcessing] = useState(false)

  useEffect(() => {
    if (!svgRef.current || data.length === 0) return

    const svg = d3.select(svgRef.current)
    const margin = defaultMargin
    const innerWidth = width - margin.left - margin.right
    const innerHeight = height - margin.top - margin.bottom

    // Clear previous content
    clearSVG(svg)

    // Set SVG dimensions
    svg
      .attr('width', width)
      .attr('height', height)
      .attr('viewBox', `0 0 ${width} ${height}`)

    // Setup time domain - show last timeWindow seconds
    const now = Date.now()
    const startTime = now - timeWindow * 1000
    
    // Filter data to time window
    const windowedData = data.filter(d => d.timestamp.getTime() >= startTime)
    
    if (windowedData.length === 0) return

    // Setup scales
    const timeDomain: [number, number] = [startTime, now]
    
    const xScale = d3.scaleLinear()
      .domain(timeDomain)
      .range([0, innerWidth])

    // Find max ratio for Y scale, but ensure we show at least up to threshold + 0.5
    const maxRatio = Math.max(
      d3.max(windowedData, d => d.ratio) || 0,
      (windowedData[0]?.threshold || 1.6) + 0.5
    )
    
    const yScale = d3.scaleLinear()
      .domain([0, maxRatio])
      .range([innerHeight, 0])

    // Create main group
    const mainGroup = createGroup(svg, 'main-group', `translate(${margin.left},${margin.top})`)

    // Create background color based on alert status
    if (isTriggered) {
      mainGroup
        .append('rect')
        .attr('class', 'alert-background')
        .attr('x', 0)
        .attr('y', 0)
        .attr('width', innerWidth)
        .attr('height', innerHeight)
        .attr('fill', '#ffebee')
        .attr('opacity', 0.5)
    }

    // Create grid lines
    if (showGrid) {
      createGridLines(mainGroup, xScale, { width: innerWidth, height: innerHeight }, 'vertical')
      createGridLines(mainGroup, yScale, { width: innerWidth, height: innerHeight }, 'horizontal')
    }

    // Draw threshold line
    if (windowedData.length > 0) {
      const threshold = windowedData[0].threshold
      const reset = windowedData[0].reset

      // Threshold line (red)
      mainGroup
        .append('line')
        .attr('class', 'threshold-line')
        .attr('x1', 0)
        .attr('x2', innerWidth)
        .attr('y1', yScale(threshold))
        .attr('y2', yScale(threshold))
        .attr('stroke', '#dc3545')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '5,5')

      // Reset line (orange)
      mainGroup
        .append('line')
        .attr('class', 'reset-line')
        .attr('x1', 0)
        .attr('x2', innerWidth)
        .attr('y1', yScale(reset))
        .attr('y2', yScale(reset))
        .attr('stroke', '#fd7e14')
        .attr('stroke-width', 1)
        .attr('stroke-dasharray', '3,3')

      // Threshold label
      mainGroup
        .append('text')
        .attr('class', 'threshold-label')
        .attr('x', innerWidth - 5)
        .attr('y', yScale(threshold) - 5)
        .attr('text-anchor', 'end')
        .style('font-size', '11px')
        .style('fill', '#dc3545')
        .style('font-weight', 'bold')
        .text(`Threshold: ${threshold.toFixed(2)}`)

      // Reset label
      mainGroup
        .append('text')
        .attr('class', 'reset-label')
        .attr('x', innerWidth - 5)
        .attr('y', yScale(reset) - 5)
        .attr('text-anchor', 'end')
        .style('font-size', '10px')
        .style('fill', '#fd7e14')
        .text(`Reset: ${reset.toFixed(2)}`)
    }

    // Create line generators
    const ratioLine = d3.line<STALTAData>()
      .x(d => xScale(d.timestamp.getTime()))
      .y(d => yScale(d.ratio))
      .curve(d3.curveLinear)

    // Draw STA/LTA ratio line
    mainGroup
      .append('path')
      .datum(windowedData)
      .attr('class', 'stalta-ratio-line')
      .attr('fill', 'none')
      .attr('stroke', isTriggered ? '#dc3545' : '#007bff')
      .attr('stroke-width', 2)
      .attr('d', ratioLine)

    // Add data points
    mainGroup
      .selectAll('.stalta-point')
      .data(windowedData)
      .enter()
      .append('circle')
      .attr('class', 'stalta-point')
      .attr('cx', d => xScale(d.timestamp.getTime()))
      .attr('cy', d => yScale(d.ratio))
      .attr('r', d => d.triggered ? 4 : 2)
      .attr('fill', d => d.triggered ? '#dc3545' : '#007bff')
      .attr('opacity', 0.8)

    // Create axes
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
      .text('STA/LTA Ratio')

    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', `translate(${innerWidth / 2}, ${innerHeight + margin.bottom})`)
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', '#666')
      .text('Time')

  }, [data, width, height, timeWindow, showGrid, isTriggered])

  if (!data.length) {
    return (
      <div className="stalta-container">
        <div className="chart-header">
          <h3 className="chart-title">
            STA/LTA Ratio
          </h3>
        </div>
        <NoDataMessage 
          message="No STA/LTA data available"
          icon="📊"
        />
      </div>
    )
  }

  const latestData = data[data.length - 1]

  return (
    <div className="stalta-container">
      <div className="chart-header">
        <h3 className="chart-title">
          STA/LTA Ratio {latestData?.channel && `- ${latestData.channel}`}
        </h3>
        <div className="control-group">
          <span className={`control-label ${isTriggered ? 'text-red-600 font-bold' : 'text-gray-600'}`}>
            Current: {latestData?.ratio?.toFixed(3) || '---'}
          </span>
          <span className="control-label">
            Threshold: {latestData?.threshold?.toFixed(2) || '---'}
          </span>
          {isTriggered && (
            <div className="flex items-center space-x-2">
              <div className="w-3 h-3 bg-red-500 rounded-full animate-pulse"></div>
              <span className="control-label text-red-600 font-bold">
                ALERT
              </span>
            </div>
          )}
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
        className="stalta-chart"
        style={{ border: '1px solid #e0e0e0' }}
      />
      
      {/* Legend */}
      <div className="mt-2 flex items-center space-x-4 text-sm text-gray-600">
        <div className="flex items-center space-x-1">
          <div className="w-4 h-0.5 bg-blue-500"></div>
          <span>STA/LTA Ratio</span>
        </div>
        <div className="flex items-center space-x-1">
          <div className="w-4 h-0.5 bg-red-500 border-dashed"></div>
          <span>Threshold</span>
        </div>
        <div className="flex items-center space-x-1">
          <div className="w-4 h-0.5 bg-orange-500 border-dashed"></div>
          <span>Reset</span>
        </div>
      </div>
    </div>
  )
}