package screenshot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ScreenshotManager manages web page screenshots using headless Chrome
type ScreenshotManager struct {
	chromeCmd     *exec.Cmd
	chromeRunning bool
	url           string
	outputDir     string
}

// NewScreenshotManager creates a new screenshot manager
func NewScreenshotManager(url, outputDir string) *ScreenshotManager {
	return &ScreenshotManager{
		url:       url,
		outputDir: outputDir,
	}
}

// Start starts the headless Chrome instance
func (sm *ScreenshotManager) Start() error {
	// Check if Google Chrome or Chromium is available
	chromePaths := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium-browser",
		"chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
	}

	var chromePath string
	for _, path := range chromePaths {
		if _, err := exec.LookPath(path); err == nil {
			chromePath = path
			break
		}
	}

	if chromePath == "" {
		return fmt.Errorf("Chrome or Chromium not found. Please install Google Chrome or Chromium")
	}

	log.Printf("Using Chrome at: %s", chromePath)

	// Ensure output directory exists
	if err := os.MkdirAll(sm.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create screenshot directory: %v", err)
	}

	// Start headless Chrome in background
	// We'll keep it running and take screenshots on demand
	go sm.runHeadlessChrome(chromePath)

	return nil
}

// runHeadlessChrome runs Chrome in headless mode in the background
func (sm *ScreenshotManager) runHeadlessChrome(chromePath string) {
	args := []string{
		"--headless=new",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--window-size=1200,800",
		"--user-data-dir=/tmp/chrome-gorsudp",
		sm.url,
	}

	sm.chromeCmd = exec.Command(chromePath, args...)
	sm.chromeRunning = true

	log.Printf("Starting headless Chrome with URL: %s", sm.url)

	if err := sm.chromeCmd.Start(); err != nil {
		log.Printf("Failed to start Chrome: %v", err)
		sm.chromeRunning = false
		return
	}

	// Wait for Chrome to start
	time.Sleep(5 * time.Second)

	log.Printf("Headless Chrome started successfully")

	// Wait for Chrome to exit
	if err := sm.chromeCmd.Wait(); err != nil {
		log.Printf("Chrome exited with error: %v", err)
	}
	sm.chromeRunning = false
}

// TakeScreenshot takes a screenshot of the web page
func (sm *ScreenshotManager) TakeScreenshot(filename string) error {
	if !sm.chromeRunning {
		return fmt.Errorf("Chrome is not running")
	}

	// Generate full path
	fullPath := filepath.Join(sm.outputDir, filename)

	// Use a separate Chrome instance for taking screenshot
	chromePaths := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium-browser",
		"chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
	}

	var chromePath string
	for _, path := range chromePaths {
		if _, err := exec.LookPath(path); err == nil {
			chromePath = path
			break
		}
	}

	if chromePath == "" {
		return fmt.Errorf("Chrome not found for screenshot")
	}

	args := []string{
		"--headless=new",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--window-size=1200,800",
		"--screenshot=" + fullPath,
		sm.url,
	}

	cmd := exec.Command(chromePath, args...)

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("screenshot failed: %v", err)
		}
		log.Printf("Screenshot saved: %s", fullPath)
		return nil
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("screenshot timeout")
	}
}

// Stop stops the Chrome instance
func (sm *ScreenshotManager) Stop() error {
	if sm.chromeCmd != nil && sm.chromeCmd.Process != nil {
		log.Printf("Stopping headless Chrome")
		if err := sm.chromeCmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill Chrome process: %v", err)
		}
		sm.chromeRunning = false
	}

	// Clean up Chrome user data directory
	if err := os.RemoveAll("/tmp/chrome-gorsudp"); err != nil {
		log.Printf("Warning: failed to clean up Chrome data: %v", err)
	}

	return nil
}

// IsRunning returns whether Chrome is currently running
func (sm *ScreenshotManager) IsRunning() bool {
	return sm.chromeRunning
}
