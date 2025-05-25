package screenshot

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// ScreenshotCapturer manages screenshot capture operations
type ScreenshotCapturer struct {
	config      ScreenshotConfig
	requestChan chan ScreenshotRequest
	resultChan  chan ScreenshotResult
	stats       ScreenshotStats
	mutex       sync.RWMutex
	stopChan    chan struct{}
	workers     []*ScreenshotWorker
}

// ScreenshotWorker processes screenshot requests
type ScreenshotWorker struct {
	id       int
	capturer *ScreenshotCapturer
	stopChan chan struct{}
}

// NewScreenshotCapturer creates a new screenshot capturer
func NewScreenshotCapturer(config ScreenshotConfig) *ScreenshotCapturer {
	if config.OutputDir == "" {
		config.OutputDir = "./screenshots"
	}
	if config.Format == "" {
		config.Format = "png"
	}
	if config.Quality == 0 {
		config.Quality = 90
	}
	if config.Width == 0 {
		config.Width = 1024
	}
	if config.Height == 0 {
		config.Height = 768
	}
	if config.Delay == 0 {
		config.Delay = 2 * time.Second
	}

	return &ScreenshotCapturer{
		config:      config,
		requestChan: make(chan ScreenshotRequest, 100),
		resultChan:  make(chan ScreenshotResult, 100),
		stats: ScreenshotStats{
			StartTime: time.Now(),
		},
		stopChan: make(chan struct{}),
		workers:  make([]*ScreenshotWorker, 0),
	}
}

// Start starts the screenshot capturer
func (sc *ScreenshotCapturer) Start(workerCount int) error {
	if !sc.config.Enabled {
		log.Println("Screenshot capturer is disabled")
		return nil
	}

	// Create output directory
	if err := os.MkdirAll(sc.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create screenshot directory: %v", err)
	}

	// Start workers
	for i := 0; i < workerCount; i++ {
		worker := &ScreenshotWorker{
			id:       i,
			capturer: sc,
			stopChan: make(chan struct{}),
		}
		sc.workers = append(sc.workers, worker)
		go worker.start()
	}

	// Start result processor
	go sc.processResults()

	log.Printf("Screenshot capturer started with %d workers", workerCount)
	return nil
}

// Stop stops the screenshot capturer
func (sc *ScreenshotCapturer) Stop() error {
	log.Println("Stopping screenshot capturer...")

	// Signal stop to all workers
	for _, worker := range sc.workers {
		close(worker.stopChan)
	}

	// Close channels
	close(sc.stopChan)
	close(sc.requestChan)

	log.Println("Screenshot capturer stopped")
	return nil
}

// CaptureScreenshot requests a screenshot capture
func (sc *ScreenshotCapturer) CaptureScreenshot(request ScreenshotRequest) error {
	if !sc.config.Enabled {
		return fmt.Errorf("screenshot capturer is disabled")
	}

	// Generate output path if not provided
	if request.OutputPath == "" {
		filename := sc.generateFilename(request)
		request.OutputPath = filepath.Join(sc.config.OutputDir, filename)
	}

	select {
	case sc.requestChan <- request:
		return nil
	default:
		return fmt.Errorf("screenshot request queue full")
	}
}

// CaptureAlertScreenshot captures a screenshot for an earthquake alert
func (sc *ScreenshotCapturer) CaptureAlertScreenshot(alertInfo AlertInfo) error {
	request := ScreenshotRequest{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Timestamp: alertInfo.Timestamp,
		EventType: "alert",
		Channel:   alertInfo.Channel,
		Priority:  10, // High priority for alerts
		Metadata: map[string]interface{}{
			"alert": alertInfo,
		},
	}

	return sc.CaptureScreenshot(request)
}

// GetStats returns current screenshot statistics
func (sc *ScreenshotCapturer) GetStats() ScreenshotStats {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()
	return sc.stats
}

// generateFilename generates a filename for the screenshot
func (sc *ScreenshotCapturer) generateFilename(request ScreenshotRequest) string {
	timestamp := request.Timestamp.Format("20060102_150405")
	return fmt.Sprintf("%s_%s_%s.%s", request.EventType, request.Channel, timestamp, sc.config.Format)
}

// processResults processes screenshot results
func (sc *ScreenshotCapturer) processResults() {
	for {
		select {
		case result := <-sc.resultChan:
			sc.updateStats(result)
			if result.Success {
				log.Printf("Screenshot captured: %s (%.2f KB)",
					result.OutputPath, float64(result.FileSize)/1024)
			} else {
				log.Printf("Screenshot failed: %s", result.Error)
			}
		case <-sc.stopChan:
			return
		}
	}
}

// updateStats updates capture statistics
func (sc *ScreenshotCapturer) updateStats(result ScreenshotResult) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.stats.TotalCaptured++
	sc.stats.LastCapture = result.CaptureTime

	if result.Success {
		sc.stats.SuccessfulCaptured++
		sc.stats.TotalFileSize += result.FileSize
		if sc.stats.SuccessfulCaptured > 0 {
			sc.stats.AverageFileSize = sc.stats.TotalFileSize / sc.stats.SuccessfulCaptured
		}
	} else {
		sc.stats.FailedCaptured++
	}
}

// start starts a screenshot worker
func (sw *ScreenshotWorker) start() {
	log.Printf("Screenshot worker %d started", sw.id)

	for {
		select {
		case request, ok := <-sw.capturer.requestChan:
			if !ok {
				log.Printf("Screenshot worker %d stopping (channel closed)", sw.id)
				return
			}
			sw.processRequest(request)

		case <-sw.stopChan:
			log.Printf("Screenshot worker %d stopping (stop signal)", sw.id)
			return
		}
	}
}

// processRequest processes a screenshot request
func (sw *ScreenshotWorker) processRequest(request ScreenshotRequest) {
	start := time.Now()

	// Add delay if configured
	if sw.capturer.config.Delay > 0 {
		time.Sleep(sw.capturer.config.Delay)
	}

	// Create result
	result := ScreenshotResult{
		RequestID:   request.ID,
		CaptureTime: time.Now(),
	}

	// Capture screenshot (simplified implementation - creates a dummy image)
	err := sw.captureScreenshot(request)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
		result.OutputPath = request.OutputPath

		// Get file size
		if stat, err := os.Stat(request.OutputPath); err == nil {
			result.FileSize = stat.Size()
		}
	}

	result.Duration = time.Since(start)

	// Send result
	select {
	case sw.capturer.resultChan <- result:
	default:
		log.Printf("Result channel full, dropping result for request %s", request.ID)
	}
}

// captureScreenshot performs the actual screenshot capture
func (sw *ScreenshotWorker) captureScreenshot(request ScreenshotRequest) error {
	// Create a dummy image for now (in a real implementation, this would capture the actual screen/browser)
	img := image.NewRGBA(image.Rect(0, 0, sw.capturer.config.Width, sw.capturer.config.Height))

	// Fill with background color
	bg := color.RGBA{240, 240, 240, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// Add some sample content
	sw.addSampleContent(img, request)

	// Add watermark if enabled
	if sw.capturer.config.Watermark {
		sw.addWatermark(img, request.Timestamp)
	}

	// Create output directory if needed
	dir := filepath.Dir(request.OutputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Save image
	file, err := os.Create(request.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	switch sw.capturer.config.Format {
	case "png":
		err = png.Encode(file, img)
	case "jpg", "jpeg":
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: sw.capturer.config.Quality})
	default:
		err = fmt.Errorf("unsupported format: %s", sw.capturer.config.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to encode image: %v", err)
	}

	return nil
}

// addSampleContent adds sample seismic plot content to the image
func (sw *ScreenshotWorker) addSampleContent(img *image.RGBA, request ScreenshotRequest) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Draw title
	sw.drawText(img, fmt.Sprintf("GoRSUDP - Seismic Monitor"), 20, 30, color.RGBA{0, 0, 0, 255})
	sw.drawText(img, fmt.Sprintf("Channel: %s", request.Channel), 20, 50, color.RGBA{0, 0, 0, 255})
	sw.drawText(img, fmt.Sprintf("Time: %s", request.Timestamp.Format("2006-01-02 15:04:05 UTC")), 20, 70, color.RGBA{0, 0, 0, 255})

	// Draw sample waveform
	centerY := height / 2
	waveColor := color.RGBA{0, 100, 200, 255}

	for x := 50; x < width-50; x++ {
		// Simple sine wave with some noise
		t := float64(x-50) / 50.0
		amplitude := 50.0
		y := centerY + int(amplitude*((0.8*sin(t*2.0))+(0.2*sin(t*10.0))))

		if y >= 0 && y < height && x >= 0 && x < width {
			img.Set(x, y, waveColor)
			// Make line thicker
			if y+1 < height {
				img.Set(x, y+1, waveColor)
			}
			if y-1 >= 0 {
				img.Set(x, y-1, waveColor)
			}
		}
	}

	// Add alert information if available
	if alertData, ok := request.Metadata["alert"].(AlertInfo); ok {
		alertY := height - 80
		sw.drawText(img, "🚨 EARTHQUAKE ALERT", 20, alertY, color.RGBA{255, 0, 0, 255})
		sw.drawText(img, fmt.Sprintf("STA/LTA Ratio: %.2f", alertData.STALTARatio), 20, alertY+20, color.RGBA{255, 0, 0, 255})
		sw.drawText(img, fmt.Sprintf("Station: %s.%s", alertData.Network, alertData.Station), 20, alertY+40, color.RGBA{255, 0, 0, 255})
	}
}

// addWatermark adds a timestamp watermark to the image
func (sw *ScreenshotWorker) addWatermark(img *image.RGBA, timestamp time.Time) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	watermarkText := fmt.Sprintf("GoRSUDP - %s", timestamp.Format("2006-01-02 15:04:05 UTC"))
	sw.drawText(img, watermarkText, width-300, height-20, color.RGBA{128, 128, 128, 255})
}

// drawText draws text on the image (simplified implementation)
func (sw *ScreenshotWorker) drawText(img *image.RGBA, text string, x, y int, col color.RGBA) {
	// Simple text drawing using basic font
	point := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(text)
}

// sin implements a simple sine function
func sin(x float64) float64 {
	// Simple sine approximation for demo
	const pi = 3.14159265359
	x = x - float64(int(x/(2*pi)))*(2*pi)
	if x < 0 {
		x = x + 2*pi
	}

	if x <= pi/2 {
		return x - (x*x*x)/6 + (x*x*x*x*x)/120
	} else if x <= pi {
		return sin(pi - x)
	} else if x <= 3*pi/2 {
		return -sin(x - pi)
	} else {
		return -sin(2*pi - x)
	}
}
