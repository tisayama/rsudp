// Package plot provides real-time plotting functionality for seismic data
package plot

import (
	"time"

	"github.com/gorilla/websocket"
)

// PlotData represents data for plotting
type PlotData struct {
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
	Samples   []float64 `json:"samples"`
	SampleRate float64  `json:"sample_rate"`
	Units     string    `json:"units"`
}

// SpectrogramData represents spectrogram data
type SpectrogramData struct {
	Channel     string      `json:"channel"`
	Timestamp   time.Time   `json:"timestamp"`
	Frequencies []float64   `json:"frequencies"`
	Times       []float64   `json:"times"`
	Power       [][]float64 `json:"power"`
}

// PlotConfig holds configuration for the plotting system
type PlotConfig struct {
	Enabled              bool     `json:"enabled"`
	Channels             []string `json:"channels"`
	Duration             int      `json:"duration"`        // seconds
	Spectrogram          bool     `json:"spectrogram"`
	Fullscreen           bool     `json:"fullscreen"`
	Kiosk                bool     `json:"kiosk"`
	EQScreenshots        bool     `json:"eq_screenshots"`
	Deconv               bool     `json:"deconv"`
	Units                string   `json:"units"`
	RefreshInterval      int      `json:"refresh_interval"` // milliseconds
	Port                 int      `json:"port"`
	Host                 string   `json:"host"`
	FilterWaveform       bool     `json:"filter_waveform"`
	FilterSpectrogram    bool     `json:"filter_spectrogram"`
	FilterHighpass       float64  `json:"filter_highpass"`
	FilterLowpass        float64  `json:"filter_lowpass"`
	FilterCorners        int      `json:"filter_corners"`
	SpectrogramFreqRange bool     `json:"spectrogram_freq_range"`
	UpperLimit           float64  `json:"upper_limit"`
	LowerLimit           float64  `json:"lower_limit"`
	LogarithmicYAxis     bool     `json:"logarithmic_y_axis"`
}

// WebSocketMessage represents messages sent over WebSocket
type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Client represents a WebSocket client
type Client struct {
	conn     *websocket.Conn
	send     chan WebSocketMessage
	channels []string // channels this client is interested in
}

// WebSocketManager manages WebSocket connections
type WebSocketManager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan WebSocketMessage
	stop       chan struct{}
	stopped    bool
}

// PlotMessage types
const (
	MessageTypePlotData       = "plot_data"
	MessageTypeSpectrogram    = "spectrogram"
	MessageTypeAlert          = "alert"
	MessageTypeReset          = "reset"
	MessageTypeConfig         = "config"
	MessageTypeChannelList    = "channel_list"
	MessageTypeSystemStatus   = "system_status"
)

// AlertMessage represents an alert event for the plot system
type AlertMessage struct {
	Channel    string    `json:"channel"`
	Timestamp  time.Time `json:"timestamp"`
	STALTARatio float64  `json:"stalta_ratio"`
	Threshold  float64   `json:"threshold"`
	Duration   float64   `json:"duration"`
	Message    string    `json:"message"`
}

// SystemStatus represents system status information
type SystemStatus struct {
	Timestamp       time.Time `json:"timestamp"`
	ActiveChannels  []string  `json:"active_channels"`
	ClientCount     int       `json:"client_count"`
	PacketsReceived int64     `json:"packets_received"`
	AlertsTriggered int64     `json:"alerts_triggered"`
	Uptime          string    `json:"uptime"`
}