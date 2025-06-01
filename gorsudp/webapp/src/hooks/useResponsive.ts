import { useState, useEffect } from 'react'

interface WindowSize {
  width: number
  height: number
}

export const useWindowSize = (): WindowSize => {
  const [windowSize, setWindowSize] = useState<WindowSize>({
    width: 0,
    height: 0,
  })

  useEffect(() => {
    function handleResize() {
      setWindowSize({
        width: window.innerWidth,
        height: window.innerHeight,
      })
    }

    // Set initial size
    if (typeof window !== 'undefined') {
      handleResize()
    }

    window.addEventListener('resize', handleResize)
    
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  return windowSize
}

export const useChartDimensions = (containerPadding = 120) => {
  const { width } = useWindowSize()
  
  const chartWidth = Math.max(400, Math.min(1100, width - containerPadding))
  const isMobile = width < 768
  const isTablet = width >= 768 && width < 1024
  const isDesktop = width >= 1024

  return {
    chartWidth,
    chartHeight: isMobile ? 250 : 300,
    isMobile,
    isTablet,
    isDesktop
  }
}