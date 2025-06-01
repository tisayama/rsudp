import * as d3 from 'd3'
import type { WaveformData } from '@/types'

// Chart dimensions and margins
export const defaultMargin = {
  top: 20,
  right: 30,
  bottom: 40,
  left: 60
}

// Time formatters
export const timeFormat = d3.timeFormat('%H:%M:%S')
export const timeFormatDetailed = d3.timeFormat('%H:%M:%S.%L')

// Number formatters
export const numberFormat = d3.format('.2f')
export const scientificFormat = d3.format('.2e')

// Color scales for different chart types
export const categoricalColors = d3.scaleOrdinal(d3.schemeCategory10)

// Viridis color scale for spectrograms
export const viridisColorScale = d3.scaleSequential(d3.interpolateViridis)
export const plasmaColorScale = d3.scaleSequential(d3.interpolatePlasma)
export const infernoColorScale = d3.scaleSequential(d3.interpolateInferno)
export const magmaColorScale = d3.scaleSequential(d3.interpolateMagma)

// Get color scale by name
export const getColorScale = (scaleName: string) => {
  switch (scaleName) {
    case 'plasma':
      return plasmaColorScale
    case 'inferno':
      return infernoColorScale
    case 'magma':
      return magmaColorScale
    case 'viridis':
    default:
      return viridisColorScale
  }
}

// Utility functions for data processing
export const calculateMean = (data: number[]): number => {
  if (data.length === 0) return 0
  return data.reduce((sum, value) => sum + value, 0) / data.length
}

export const calculateStandardDeviation = (data: number[]): number => {
  if (data.length === 0) return 0
  const mean = calculateMean(data)
  const variance = data.reduce((sum, value) => sum + Math.pow(value - mean, 2), 0) / data.length
  return Math.sqrt(variance)
}

// Remove DC component (mean) from data
export const removeDC = (data: WaveformData[]): WaveformData[] => {
  if (data.length === 0) return data
  
  const values = data.map(d => d.value)
  const mean = calculateMean(values)
  
  return data.map(d => ({
    ...d,
    value: d.value - mean
  }))
}

// Scale conversion utilities
export const nanometersToMicrometers = (nm: number): number => nm / 1000
export const micrometersToNanometers = (um: number): number => um * 1000

// Time domain utilities
export const getTimeWindow = (data: WaveformData[], windowSeconds: number): WaveformData[] => {
  if (data.length === 0) return data
  
  const latestTime = data[data.length - 1].timestamp
  const cutoffTime = new Date(latestTime.getTime() - windowSeconds * 1000)
  
  return data.filter(d => d.timestamp >= cutoffTime)
}

// Find data range for auto-scaling
export const getDataRange = (data: WaveformData[], padding = 0.1): [number, number] => {
  if (data.length === 0) return [0, 1]
  
  const values = data.map(d => d.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  
  const range = max - min
  const paddingAmount = range * padding
  
  return [min - paddingAmount, max + paddingAmount]
}

// SVG element management utilities
export const clearSVG = (svg: d3.Selection<SVGSVGElement, unknown, null, undefined>) => {
  svg.selectAll('*').remove()
}

export const createGroup = (
  svg: d3.Selection<SVGSVGElement, unknown, null, undefined>,
  className: string,
  transform?: string
) => {
  const group = svg.append('g').attr('class', className)
  if (transform) {
    group.attr('transform', transform)
  }
  return group
}

// Axis creation utilities
export const createTimeAxis = (
  scale: d3.ScaleTime<number, number>,
  position: 'top' | 'bottom' = 'bottom'
) => {
  const axis = position === 'top' ? d3.axisTop(scale) : d3.axisBottom(scale)
  return axis.tickFormat((domainValue: d3.NumberValue | Date) => {
    if (domainValue instanceof Date) {
      return timeFormat(domainValue)
    }
    return timeFormat(new Date(domainValue as number))
  })
}

export const createLinearAxis = (
  scale: d3.ScaleLinear<number, number>,
  position: 'left' | 'right' = 'left'
) => {
  const axis = position === 'left' ? d3.axisLeft(scale) : d3.axisRight(scale)
  return axis.tickFormat(numberFormat)
}

// Grid line utilities
export const createGridLines = (
  parent: d3.Selection<SVGGElement, unknown, null, undefined>,
  scale: d3.ScaleTime<number, number> | d3.ScaleLinear<number, number>,
  dimension: { width: number; height: number },
  orientation: 'horizontal' | 'vertical'
) => {
  const tickSize = orientation === 'horizontal' ? -dimension.width : -dimension.height
  
  if (orientation === 'horizontal') {
    const linearScale = scale as d3.ScaleLinear<number, number>
    const axis = d3.axisLeft(linearScale).tickSize(tickSize).tickFormat(() => '')
    
    return parent
      .append('g')
      .attr('class', `grid grid-${orientation}`)
      .call(axis)
      .selectAll('line')
      .attr('stroke', '#e0e0e0')
      .attr('stroke-dasharray', '2,2')
      .attr('opacity', 0.7)
  } else {
    const timeScale = scale as d3.ScaleTime<number, number>
    const axis = d3.axisBottom(timeScale).tickSize(tickSize).tickFormat(() => '')
    
    return parent
      .append('g')
      .attr('class', `grid grid-${orientation}`)
      .call(axis)
      .selectAll('line')
      .attr('stroke', '#e0e0e0')
      .attr('stroke-dasharray', '2,2')
      .attr('opacity', 0.7)
  }
}