import { useRef, useCallback, useEffect } from 'react'

interface BatchConfig {
  batchSize?: number
  maxDelay?: number  // Maximum delay in milliseconds before forcing a batch
}

/**
 * Hook that batches incoming data to reduce processing overhead
 * Useful for high-frequency real-time data streams
 */
export function useBatchedData<T>(
  onBatch: (batch: T[]) => void,
  config: BatchConfig = {}
) {
  const { batchSize = 10, maxDelay = 100 } = config
  
  const batchRef = useRef<T[]>([])
  const timerRef = useRef<NodeJS.Timeout | null>(null)
  
  const processBatch = useCallback(() => {
    if (batchRef.current.length > 0) {
      onBatch(batchRef.current)
      batchRef.current = []
    }
    
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [onBatch])
  
  const addData = useCallback((data: T) => {
    batchRef.current.push(data)
    
    // Process immediately if batch is full
    if (batchRef.current.length >= batchSize) {
      processBatch()
      return
    }
    
    // Set timer for max delay if not already set
    if (!timerRef.current) {
      timerRef.current = setTimeout(processBatch, maxDelay)
    }
  }, [batchSize, maxDelay, processBatch])
  
  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
        processBatch() // Process any remaining data
      }
    }
  }, [processBatch])
  
  return { addData }
}