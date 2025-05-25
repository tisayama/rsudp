// Package screenshot provides screenshot capture functionality for earthquake events
package screenshot

import (
	"time"
)

// ScreenshotConfig contains configuration for screenshot capture
type ScreenshotConfig struct {
	Enabled       bool          `json:"enabled"`
	OutputDir     string        `json:"output_dir"`
	Format        string        `json:"format"`         // png, jpg
	Quality       int           `json:"quality"`        // JPEG quality (1-100)
	Width         int           `json:"width"`          // Screenshot width
	Height        int           `json:"height"`         // Screenshot height
	Delay         time.Duration `json:"delay"`          // Delay before capture
	IncludeAlerts bool          `json:"include_alerts"` // Include alert information in image
	Watermark     bool          `json:"watermark"`      // Add timestamp watermark
}

// ScreenshotRequest represents a request to capture a screenshot
type ScreenshotRequest struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	EventType  string                 `json:"event_type"` // "alert", "reset", "manual"
	Channel    string                 `json:"channel"`
	OutputPath string                 `json:"output_path"`
	Metadata   map[string]interface{} `json:"metadata"`
	Priority   int                    `json:"priority"` // Higher number = higher priority
}

// ScreenshotResult represents the result of a screenshot capture
type ScreenshotResult struct {
	RequestID   string        `json:"request_id"`
	Success     bool          `json:"success"`
	OutputPath  string        `json:"output_path"`
	FileSize    int64         `json:"file_size"`
	Error       string        `json:"error,omitempty"`
	CaptureTime time.Time     `json:"capture_time"`
	Duration    time.Duration `json:"duration"`
}

// AlertInfo contains information about an earthquake alert for screenshot annotation
type AlertInfo struct {
	Channel     string    `json:"channel"`
	Timestamp   time.Time `json:"timestamp"`
	STALTARatio float64   `json:"stalta_ratio"`
	Threshold   float64   `json:"threshold"`
	Duration    float64   `json:"duration"`
	Station     string    `json:"station"`
	Network     string    `json:"network"`
}

// ScreenshotStats tracks screenshot capture statistics
type ScreenshotStats struct {
	TotalCaptured      int64     `json:"total_captured"`
	SuccessfulCaptured int64     `json:"successful_captured"`
	FailedCaptured     int64     `json:"failed_captured"`
	LastCapture        time.Time `json:"last_capture"`
	TotalFileSize      int64     `json:"total_file_size"`
	AverageFileSize    int64     `json:"average_file_size"`
	StartTime          time.Time `json:"start_time"`
}
