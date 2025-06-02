// Package notify provides notification functionality for earthquake alerts
package notify

import (
	"time"
)

// NotificationProvider interface defines methods for sending notifications
type NotificationProvider interface {
	SendText(message string) error
	SendImage(message string, imagePath string) error
	GetName() string
	IsEnabled() bool
}

// NotificationJob represents a notification task
type NotificationJob struct {
	ID         string                 `json:"id"`
	Type       NotificationType       `json:"type"`
	Provider   string                 `json:"provider"`
	Message    string                 `json:"message"`
	ImagePath  string                 `json:"image_path,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Retry      int                    `json:"retry"`
	MaxRetries int                    `json:"max_retries"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationType represents the type of notification
type NotificationType int

const (
	NotificationText NotificationType = iota
	NotificationImage
	NotificationAlert
	NotificationReset
)

func (nt NotificationType) String() string {
	switch nt {
	case NotificationText:
		return "text"
	case NotificationImage:
		return "image"
	case NotificationAlert:
		return "alert"
	case NotificationReset:
		return "reset"
	default:
		return "unknown"
	}
}

// NotificationResult represents the result of a notification attempt
type NotificationResult struct {
	JobID     string        `json:"job_id"`
	Provider  string        `json:"provider"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// RateLimit represents rate limiting configuration
type RateLimit struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	Enabled           bool          `json:"enabled"`
	WindowSize        time.Duration `json:"window_size"`
}

// NotificationConfig represents configuration for notification system
type NotificationConfig struct {
	Enabled    bool          `json:"enabled"`
	Workers    int           `json:"workers"`
	QueueSize  int           `json:"queue_size"`
	RetryDelay time.Duration `json:"retry_delay"`
	MaxRetries int           `json:"max_retries"`
	Timeout    time.Duration `json:"timeout"`
}

// Alert represents an earthquake alert for notifications
type Alert struct {
	Channel     string    `json:"channel"`
	Timestamp   time.Time `json:"timestamp"`
	STALTARatio float64   `json:"stalta_ratio"`
	Threshold   float64   `json:"threshold"`
	Duration    float64   `json:"duration"`
	Magnitude   float64   `json:"magnitude,omitempty"`
	Location    string    `json:"location,omitempty"`
	Station     string    `json:"station"`
	Network     string    `json:"network"`
}

// Reset represents an alert reset notification
type Reset struct {
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
	Station   string    `json:"station"`
	Network   string    `json:"network"`
	Duration  float64   `json:"duration"`
}
