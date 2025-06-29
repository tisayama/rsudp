import { test, expect } from '@playwright/test'

test.describe('Waveform and Spectrogram Display', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the application
    await page.goto('http://localhost:3000')
    
    // Wait for initial WebSocket connection
    await page.waitForTimeout(2000)
  })

  test('should display waveform with data', async ({ page }) => {
    // Wait for waveform container to be visible
    const waveformContainer = page.locator('.waveform-container').first()
    await expect(waveformContainer).toBeVisible()

    // Check if the SVG element exists and has content
    const waveformSvg = waveformContainer.locator('svg.waveform-chart')
    await expect(waveformSvg).toBeVisible()
    
    // Wait for waveform path to be drawn
    await page.waitForTimeout(3000) // Give time for data to arrive
    
    // Check if waveform path exists
    const waveformPath = waveformSvg.locator('path.waveform-path')
    await expect(waveformPath).toBeVisible()
    
    // Verify the path has data (d attribute should not be empty)
    const pathData = await waveformPath.getAttribute('d')
    expect(pathData).toBeTruthy()
    expect(pathData).not.toBe('')
    expect(pathData?.length).toBeGreaterThan(10) // Should have substantial path data
    
    // Check for axes
    await expect(waveformSvg.locator('.x-axis')).toBeVisible()
    await expect(waveformSvg.locator('.y-axis')).toBeVisible()
    
    // Check for axis labels
    await expect(waveformSvg.locator('text').filter({ hasText: 'Velocity' })).toBeVisible()
    await expect(waveformSvg.locator('text').filter({ hasText: 'Time' })).toBeVisible()
  })

  test('should display spectrogram with data', async ({ page }) => {
    // Wait for spectrogram container to be visible
    const spectrogramContainer = page.locator('.spectrogram-container').first()
    await expect(spectrogramContainer).toBeVisible()

    // Check if the canvas element exists
    const spectrogramCanvas = spectrogramContainer.locator('canvas.spectrogram-canvas')
    await expect(spectrogramCanvas).toBeVisible()
    
    // Check if the SVG overlay exists
    const spectrogramSvg = spectrogramContainer.locator('svg.spectrogram-chart')
    await expect(spectrogramSvg).toBeVisible()
    
    // Wait for spectrogram to process data
    await page.waitForTimeout(5000) // FFT processing takes time
    
    // Check for axes in SVG overlay
    await expect(spectrogramSvg.locator('.x-axis')).toBeVisible()
    await expect(spectrogramSvg.locator('.y-axis')).toBeVisible()
    
    // Check for axis labels
    await expect(spectrogramSvg.locator('text').filter({ hasText: 'Frequency' })).toBeVisible()
    await expect(spectrogramSvg.locator('text').filter({ hasText: 'Time' })).toBeVisible()
    
    // Verify canvas has been drawn to (check dimensions)
    const canvasWidth = await spectrogramCanvas.evaluate(el => (el as HTMLCanvasElement).width)
    const canvasHeight = await spectrogramCanvas.evaluate(el => (el as HTMLCanvasElement).height)
    expect(canvasWidth).toBeGreaterThan(0)
    expect(canvasHeight).toBeGreaterThan(0)
  })

  test('should show channel tabs when data is received', async ({ page }) => {
    // Wait for data to arrive
    await page.waitForTimeout(3000)
    
    // Check if channel tabs are visible
    const channelTabsContainer = page.locator('.channel-tab').first()
    await expect(channelTabsContainer).toBeVisible()
    
    // Should have at least one channel tab
    const tabs = page.locator('.channel-tab')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(0)
    
    // First tab should be active
    const firstTab = tabs.first()
    await expect(firstTab).toHaveClass(/active/)
  })

  test('should update waveform data continuously', async ({ page }) => {
    // Wait for initial data
    await page.waitForTimeout(3000)
    
    const waveformPath = page.locator('path.waveform-path').first()
    
    // Get initial path data
    const initialPathData = await waveformPath.getAttribute('d')
    expect(initialPathData).toBeTruthy()
    
    // Wait for updates
    await page.waitForTimeout(2000)
    
    // Get updated path data
    const updatedPathData = await waveformPath.getAttribute('d')
    expect(updatedPathData).toBeTruthy()
    
    // Path data should have changed (indicating new data)
    expect(updatedPathData).not.toBe(initialPathData)
  })

  test('should handle no data gracefully', async ({ page, context }) => {
    // Block WebSocket connections to simulate no data
    await context.route('ws://localhost:8080/ws', route => route.abort())
    
    // Reload page
    await page.reload()
    
    // Should show no data message for waveform
    const noDataMessage = page.locator('.waveform-container h3:has-text("No waveform data available")')
    await expect(noDataMessage).toBeVisible({ timeout: 10000 })
    
    // Should show no data message for spectrogram
    const spectrogramNoData = page.locator('.spectrogram-container h3:has-text("No spectrogram data available")')
    await expect(spectrogramNoData).toBeVisible()
  })

  test('should switch between channels', async ({ page }) => {
    // Wait for multiple channels to appear
    await page.waitForTimeout(5000)
    
    const tabs = page.locator('.channel-tab')
    const tabCount = await tabs.count()
    
    if (tabCount > 1) {
      // Click on second channel
      const secondTab = tabs.nth(1)
      const secondChannelName = await secondTab.textContent()
      await secondTab.click()
      
      // Verify tab is active
      await expect(secondTab).toHaveClass(/active/)
      
      // Verify waveform title updated
      const waveformTitle = page.locator('.waveform-container .chart-title').first()
      await expect(waveformTitle).toContainText(secondChannelName || '')
      
      // Verify spectrogram title updated
      const spectrogramTitle = page.locator('.spectrogram-container .chart-title').first()
      await expect(spectrogramTitle).toContainText(secondChannelName || '')
    }
  })
})