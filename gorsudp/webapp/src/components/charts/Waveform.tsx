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
  createLinearAxis,
  createGridLines,
  getTimeWindow,
  getDataRange,
  removeDC,
  nanometersToMicrometers
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
    x: d3.ScaleTime<number, number>
    y: d3.ScaleLinear<number, number>
  } | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const renderingRef = useRef(false) // Prevent duplicate rendering
  const lastRenderKey = useRef<string>('')
  const mountedRef = useRef(false) // Track component mount state

  // Mount effect to track component lifecycle
  useEffect(() => {
    console.log('🔥 Waveform component MOUNTED for channel:', channel)
    mountedRef.current = true
    
    return () => {
      console.log('💀 Waveform component UNMOUNTED for channel:', channel)
      mountedRef.current = false
    }
  }, [channel])

  useEffect(() => {
    const renderTime = new Date().toISOString()
    const componentId = `waveform-${channel}-${Math.random().toString(36).substr(2, 9)}`
    
    // Create a unique key for this render based on data and props
    const renderKey = `${channel}-${data.length}-${width}-${height}-${timeWindow}`
    
    console.log('🌊 Waveform render:', componentId, 'data:', data.length, 'time:', renderTime)
    console.log('🌊 SVG ref element:', svgRef.current?.id || 'no-id')
    console.log('🔐 Render key:', renderKey, 'last key:', lastRenderKey.current)
    console.log('🔐 Already rendering:', renderingRef.current)
    console.log('🔐 Component mounted:', mountedRef.current)
    
    // Skip rendering if component is not properly mounted
    if (!mountedRef.current) {
      console.log('⛔ Skipping render: component not mounted')
      return
    }
    
    // Check if already rendering or same render
    if (renderingRef.current) {
      console.log('⛔ Skipping render: already in progress')
      return
    }
    
    if (renderKey === lastRenderKey.current) {
      console.log('⛔ Skipping render: same render key')
      return
    }
    
    // Set rendering guard
    renderingRef.current = true
    lastRenderKey.current = renderKey
    
    console.log('✅ Starting render with key:', renderKey)
    
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
      console.log('🧹 Before clear - SVG elements:', svg.selectAll('*').size())
      console.log('🧹 Before clear - Paths specifically:', svg.selectAll('path').size())
      console.log('🧹 SVG innerHTML before:', svgRef.current.innerHTML.length, 'chars')
      
      svgRef.current.innerHTML = ''
      
      console.log('🧹 After innerHTML clear - SVG elements:', svg.selectAll('*').size())
      console.log('🧹 After innerHTML clear - Paths specifically:', svg.selectAll('path').size())
      console.log('🧹 SVG innerHTML after:', svgRef.current.innerHTML.length, 'chars')
      
      // Set SVG dimensions and stable ID based on channel only
      const svgId = `waveform-svg-${channel}`  // Stable ID based on channel
      svg
        .attr('id', svgId)
        .attr('width', width)
        .attr('height', height)
        .attr('viewBox', `0 0 ${width} ${height}`)
      
      console.log('🏷️ SVG ID set to stable ID:', svgId, '(component:', componentId, ')')
      
      // Check total SVG elements in the document
      const allSvgs = document.querySelectorAll('svg')
      const allWaveformSvgs = document.querySelectorAll('svg[id*="waveform"]')
      console.log('🔍 Total SVG elements in document:', allSvgs.length)
      console.log('🔍 Total waveform SVG elements:', allWaveformSvgs.length)

    // Get data within time window
    const windowedData = getTimeWindow(data, timeWindow)
    if (windowedData.length === 0) return

    // Remove DC component and convert units
    const processedData = removeDC(windowedData).map(d => ({
      ...d,
      value: units === 'nm/s' ? nanometersToMicrometers(d.value) : d.value
    }))
    
    console.log('📊 Processed:', processedData.length, 'points')

    // Setup scales - Use linear scale with millisecond timestamps for better precision
    const timeExtent = d3.extent(processedData, d => d.timestamp.getTime()) as [number, number]
    const xScale = d3.scaleLinear()
      .domain(timeExtent)
      .range([0, innerWidth])

    let yDomain: [number, number]
    if (autoScale) {
      yDomain = getDataRange(processedData, 0.1)
    } else {
      // Fixed scale based on typical seismic data range in μm/s
      yDomain = [-100, 100]
    }

    const yScale = d3.scaleLinear()
      .domain(yDomain)
      .range([innerHeight, 0])

    setScales({ x: xScale, y: yScale })

    // Create main group
    const mainGroup = createGroup(svg, 'main-group', `translate(${margin.left},${margin.top})`)

    // Create grid lines (temporarily disabled for debugging)
    console.log('🔲 Grid enabled:', showGrid)
    /*
    if (showGrid) {
      createGridLines(mainGroup, xScale, { width: innerWidth, height: innerHeight }, 'vertical')
      createGridLines(mainGroup, yScale, { width: innerWidth, height: innerHeight }, 'horizontal')
    }
    */

    // Create axes (temporarily disabled for debugging)
    console.log('📊 Temporarily disabling axes for debugging')
    /*
    const xAxis = createTimeAxis(xScale, 'bottom')
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
    */

    // Add axis labels (temporarily disabled for debugging)
    /*
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
      .text('Velocity (μm/s)')

    mainGroup
      .append('text')
      .attr('class', 'axis-label')
      .attr('transform', `translate(${innerWidth / 2}, ${innerHeight + margin.bottom})`)
      .style('text-anchor', 'middle')
      .style('font-size', '12px')
      .style('fill', '#666')
      .text('Time')
    */

    // Use processed data directly (filtering was done in processing step)
    const validData = processedData
    
    console.log('✅ Using data points:', validData.length)
    if (validData.length > 0) {
      console.log('Sample data:', {
        first: { t: validData[0].timestamp.toISOString(), v: validData[0].value },
        last: { t: validData[validData.length - 1].timestamp.toISOString(), v: validData[validData.length - 1].value }
      })
      
      // Check for duplicate timestamps that might cause issues
      const timeMap = new Map<string, number>()
      const timeMapMillis = new Map<number, number>()
      let duplicateCount = 0
      let duplicateMillisCount = 0
      
      validData.forEach((point, index) => {
        const timeKey = point.timestamp.toISOString()
        const timeMillis = point.timestamp.getTime()
        
        // Check ISO string duplicates
        if (timeMap.has(timeKey)) {
          duplicateCount++
          if (duplicateCount <= 3) {
            console.log(`🔴 Duplicate timestamp ${duplicateCount}: ${timeKey} at indices ${timeMap.get(timeKey)} and ${index}`)
          }
        } else {
          timeMap.set(timeKey, index)
        }
        
        // Check millisecond duplicates
        if (timeMapMillis.has(timeMillis)) {
          duplicateMillisCount++
          if (duplicateMillisCount <= 3) {
            console.log(`🟡 Duplicate milliseconds ${duplicateMillisCount}: ${timeMillis} at indices ${timeMapMillis.get(timeMillis)} and ${index}`)
            console.log(`   ISO strings: ${validData[timeMapMillis.get(timeMillis)!].timestamp.toISOString()} vs ${point.timestamp.toISOString()}`)
          }
        } else {
          timeMapMillis.set(timeMillis, index)
        }
      })
      
      if (duplicateCount > 0) {
        console.log(`🔴 Total duplicate timestamps found: ${duplicateCount}`)
      }
      if (duplicateMillisCount > 0) {
        console.log(`🟡 Total duplicate milliseconds found: ${duplicateMillisCount}`)
      }
      
      // Check timestamp precision
      if (validData.length > 2) {
        const diff1 = validData[1].timestamp.getTime() - validData[0].timestamp.getTime()
        const diff2 = validData[2].timestamp.getTime() - validData[1].timestamp.getTime()
        console.log(`⏱️ Time differences: ${diff1}ms, ${diff2}ms (expected: 10ms)`)
        console.log(`⏱️ First 3 timestamps:`)
        validData.slice(0, 3).forEach((d, i) => {
          console.log(`   ${i}: ${d.timestamp.toISOString()} (${d.timestamp.getTime()}ms)`)
        })
      }
      
      // Log first few coordinates that will be generated
      const sampleCoords = validData.slice(0, 5).map(d => `(${xScale(d.timestamp.getTime()).toFixed(3)}, ${yScale(d.value).toFixed(1)})`)
      console.log('🎯 First 5 coordinates:', sampleCoords.join(' → '))
      
      // Check for duplicate X coordinates
      const xCoordMap = new Map<string, number>()
      let duplicateXCount = 0
      validData.forEach((point, index) => {
        const xCoord = xScale(point.timestamp.getTime()).toFixed(3)
        if (xCoordMap.has(xCoord)) {
          duplicateXCount++
          if (duplicateXCount <= 3) {
            const prevIndex = xCoordMap.get(xCoord)!
            console.log(`🟠 Duplicate X coordinate ${duplicateXCount}: ${xCoord} at indices ${prevIndex} and ${index}`)
            console.log(`   Times: ${validData[prevIndex].timestamp.toISOString()} vs ${point.timestamp.toISOString()}`)
            console.log(`   Millis: ${validData[prevIndex].timestamp.getTime()} vs ${point.timestamp.getTime()}`)
          }
        } else {
          xCoordMap.set(xCoord, index)
        }
      })
      
      if (duplicateXCount > 0) {
        console.log(`🟠 Total duplicate X coordinates found: ${duplicateXCount}`)
      }
    }
    
    // Create line generator (remove .defined() to prevent unwanted connections)
    const line = d3.line<WaveformData>()
      .x(d => xScale(d.timestamp.getTime()))
      .y(d => yScale(d.value))
      .curve(d3.curveLinear)

    // TEMPORARILY DISABLE segment splitting - use single path for debugging
    console.log('🧪 Testing single path without segmentation')
    
    // Check state before drawing
    console.log('🔍 Before drawing - paths in mainGroup:', mainGroup.selectAll('path').size())
    console.log('🔍 Before drawing - paths in SVG:', svg.selectAll('path').size())
    
    // Generate path data manually to debug
    const pathData = line(validData)
    console.log('🔍 Generated path data length:', pathData?.length || 0)
    console.log('🔍 Path data preview (first 150 chars):', pathData?.substring(0, 150) + '...')
    
    // Count how many "M" (moveTo) commands are in the path
    const moveToCount = (pathData?.match(/M/g) || []).length
    console.log('🔍 MoveTo commands in path:', moveToCount)
    
    // Check for patterns that might indicate duplication
    if (pathData && pathData.length > 300) {
      const firstPart = pathData.substring(0, 150)
      const restOfPath = pathData.substring(150)
      if (restOfPath.includes(firstPart.substring(1, 50))) { // Skip 'M' command
        console.log('🚨 FOUND DUPLICATE PATTERN IN PATH DATA!')
        console.log('🚨 First part:', firstPart)
        const duplicateIndex = restOfPath.indexOf(firstPart.substring(1, 50))
        console.log('🚨 Duplicate starts at position:', 150 + duplicateIndex)
      }
    }

    const pathElement = mainGroup
      .append('path')
      .datum(validData)
      .attr('class', 'waveform-path')
      .attr('fill', 'none')
      .attr('stroke', '#1f77b4')
      .attr('stroke-width', 1.5)
      .attr('d', pathData)
    
    // Check state after drawing
    console.log('📈 After drawing - paths in mainGroup:', mainGroup.selectAll('path').size())
    console.log('📈 After drawing - paths in SVG:', svg.selectAll('path').size())
    console.log('📈 Single path drawn with', validData.length, 'points')
    
    // Verify the path element was actually created
    const createdPath = pathElement.node()
    if (createdPath) {
      console.log('📈 Created path element:', {
        tag: createdPath.tagName,
        class: createdPath.className.baseVal,
        stroke: createdPath.getAttribute('stroke'),
        parent: createdPath.parentElement?.tagName
      })
    } else {
      console.error('❌ Failed to create path element!')
    }
    
    // Check DOM paths across all SVGs
    const allPaths = document.querySelectorAll('path')
    const allWaveformPaths = document.querySelectorAll('path[class*="waveform"]')
    console.log('🔍 Total path elements in entire document:', allPaths.length)
    console.log('🔍 Total waveform path elements in document:', allWaveformPaths.length)
    
    // Log the current SVG's content in detail
    console.log('📝 Current SVG innerHTML length:', svgRef.current.innerHTML.length)
    console.log('📝 Current SVG content (first 200 chars):', svgRef.current.innerHTML.substring(0, 200) + '...')
    
    // Count all elements within the SVG
    const allElementsInSvg = svgRef.current.querySelectorAll('*')
    const allPathsInSvg = svgRef.current.querySelectorAll('path')
    const allGroupsInSvg = svgRef.current.querySelectorAll('g')
    const allLinesInSvg = svgRef.current.querySelectorAll('line')
    
    console.log('🔬 Elements in this SVG:')
    console.log('  - Total elements:', allElementsInSvg.length)
    console.log('  - Path elements:', allPathsInSvg.length)
    console.log('  - Group elements:', allGroupsInSvg.length) 
    console.log('  - Line elements:', allLinesInSvg.length)
    
    // Log each path element details
    allPathsInSvg.forEach((path, index) => {
      console.log(`  📍 Path ${index + 1}:`, {
        class: path.className.baseVal,
        stroke: path.getAttribute('stroke'),
        strokeWidth: path.getAttribute('stroke-width'),
        fill: path.getAttribute('fill'),
        dLength: path.getAttribute('d')?.length || 0
      })
    })
    
    // Log each line element details
    allLinesInSvg.forEach((line, index) => {
      console.log(`  📏 Line ${index + 1}:`, {
        class: line.className.baseVal,
        stroke: line.getAttribute('stroke'),
        strokeWidth: line.getAttribute('stroke-width'),
        x1: line.getAttribute('x1'),
        y1: line.getAttribute('y1'),
        x2: line.getAttribute('x2'),
        y2: line.getAttribute('y2')
      })
    })
    
    // Store SVG reference globally for debugging
    if (typeof window !== 'undefined') {
      (window as any).debugWaveformSVG = svgRef.current
      console.log('🔧 SVG stored in window.debugWaveformSVG for inspection')
      console.log('🔧 Run in console: window.debugWaveformSVG.innerHTML')
    }

    // Add zero line (temporarily disabled for debugging)
    console.log('🟢 Zero line Y position:', yScale(0), 'innerHeight:', innerHeight)
    
    // TEMPORARILY COMMENT OUT zero line to test if it's causing the 2nd line
    /*
    mainGroup
      .append('line')
      .attr('class', 'zero-line')
      .attr('x1', 0)
      .attr('x2', innerWidth)
      .attr('y1', yScale(0))
      .attr('y2', yScale(0))
      .attr('stroke', '#666')
      .attr('stroke-width', 1)
      .attr('stroke-dasharray', '3,3')
      .attr('opacity', 0.5)
    */

      setIsLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to render waveform')
      setIsLoading(false)
    } finally {
      // Always clear rendering guard
      renderingRef.current = false
      console.log('🔓 Render complete, guard cleared')
    }
  }, [data, width, height, timeWindow, showGrid, autoScale, units])

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
        style={{ border: '1px solid #e0e0e0' }}
      />
    </div>
  )
}