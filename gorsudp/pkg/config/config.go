// Package config provides configuration management for gorsudp
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Config represents the complete application configuration
type Config struct {
	Settings     Settings     `json:"settings" yaml:"settings"`
	Alert        Alert        `json:"alert" yaml:"alert"`
	Plot         Plot         `json:"plot" yaml:"plot"`
	Write        Write        `json:"write" yaml:"write"`
	PrintData    PrintData    `json:"printdata" yaml:"printdata"`
	Telegram     Telegram     `json:"telegram" yaml:"telegram"`
	Twitter      Twitter      `json:"twitter" yaml:"twitter"`
	Discord      Discord      `json:"discord" yaml:"discord"`
	Bluesky      Bluesky      `json:"bluesky" yaml:"bluesky"`
	GoogleChat   GoogleChat   `json:"googlechat" yaml:"googlechat"`
	LINE         LINE         `json:"line" yaml:"line"`
	SNS          SNS          `json:"sns" yaml:"sns"`
	AlertSound   AlertSound   `json:"alertsound" yaml:"alertsound"`
	Forward      Forward      `json:"forward" yaml:"forward"`
	RSAM         RSAM         `json:"rsam" yaml:"rsam"`
	Custom       Custom       `json:"custom" yaml:"custom"`
}

// Settings contains general application settings
type Settings struct {
	Port        int    `json:"port" yaml:"port"`
	Station     string `json:"station" yaml:"station"`
	Network     string `json:"network" yaml:"network"`
	Location    string `json:"location" yaml:"location"`
	OutDir      string `json:"output_dir" yaml:"output_dir"`
	DataDir     string `json:"data_dir" yaml:"data_dir"`
	ScreenshotDir string `json:"screenshot_dir" yaml:"screenshot_dir"`
	LogDir      string `json:"log_dir" yaml:"log_dir"`
	Debug       bool   `json:"debug" yaml:"debug"`
}

// Alert contains STA/LTA earthquake detection settings
type Alert struct {
	Enabled   bool      `json:"enabled" yaml:"enabled"`
	Channel   string    `json:"channel" yaml:"channel"`
	STA       float64   `json:"sta" yaml:"sta"`           // Short Term Average window (seconds)
	LTA       float64   `json:"lta" yaml:"lta"`           // Long Term Average window (seconds)
	Threshold float64   `json:"threshold" yaml:"threshold"` // STA/LTA trigger threshold
	Reset     float64   `json:"reset" yaml:"reset"`       // Reset threshold
	Highpass  float64   `json:"highpass" yaml:"highpass"` // Highpass filter (Hz)
	Lowpass   float64   `json:"lowpass" yaml:"lowpass"`   // Lowpass filter (Hz)
	Duration  float64   `json:"duration" yaml:"duration"` // Duration requirement (seconds)
	Deconv    string    `json:"deconv" yaml:"deconv"`     // Deconvolution type
	Units     string    `json:"units" yaml:"units"`       // Display units
}

// Plot contains real-time plotting settings
type Plot struct {
	Enabled         bool     `json:"enabled" yaml:"enabled"`
	Channels        []string `json:"channels" yaml:"channels"`
	Duration        int      `json:"duration" yaml:"duration"`     // Plot duration (seconds)
	Spectrogram     bool     `json:"spectrogram" yaml:"spectrogram"`
	Fullscreen      bool     `json:"fullscreen" yaml:"fullscreen"`
	Kiosk           bool     `json:"kiosk" yaml:"kiosk"`
	EqScreenshots   bool     `json:"eq_screenshots" yaml:"eq_screenshots"`
	Deconv          bool     `json:"deconv" yaml:"deconv"`
	Units           string   `json:"units" yaml:"units"`
	RefreshInterval int      `json:"refresh_interval" yaml:"refresh_interval"` // Refresh interval (ms)
}

// Write contains data writing settings
type Write struct {
	Enabled  bool     `json:"enabled" yaml:"enabled"`
	Channels []string `json:"channels" yaml:"channels"`
	OutDir   string   `json:"outdir" yaml:"outdir"`
}

// PrintData contains data printing settings
type PrintData struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// Telegram contains Telegram notification settings
type Telegram struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Token      string `json:"token" yaml:"token"`
	ChatID     string `json:"chat_id" yaml:"chat_id"`
	SendImages bool   `json:"send_images" yaml:"send_images"`
}

// Twitter contains Twitter notification settings
type Twitter struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	ConsumerKey     string `json:"consumer_key" yaml:"consumer_key"`
	ConsumerSecret  string `json:"consumer_secret" yaml:"consumer_secret"`
	AccessToken     string `json:"access_token" yaml:"access_token"`
	AccessSecret    string `json:"access_secret" yaml:"access_secret"`
	TweetImages     bool   `json:"tweet_images" yaml:"tweet_images"`
	ExtraText       string `json:"extra_text" yaml:"extra_text"`
}

// Discord contains Discord notification settings
type Discord struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	WebhookURL  string `json:"webhook_url" yaml:"webhook_url"`
	UseEmbed    bool   `json:"use_embed" yaml:"use_embed"`
	SendImages  bool   `json:"send_images" yaml:"send_images"`
}

// Bluesky contains Bluesky notification settings
type Bluesky struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
	PostImages bool   `json:"post_images" yaml:"post_images"`
}

// GoogleChat contains Google Chat notification settings
type GoogleChat struct {
	Enabled                    bool   `json:"enabled" yaml:"enabled"`
	WebhookURL                string `json:"webhook_url" yaml:"webhook_url"`
	SendImages                bool   `json:"send_images" yaml:"send_images"`
	S3BucketName              string `json:"s3_bucket_name" yaml:"s3_bucket_name"`
	S3AWSRegion               string `json:"s3_aws_region" yaml:"s3_aws_region"`
	S3UploadTimeoutSeconds    int    `json:"s3_upload_timeout_seconds" yaml:"s3_upload_timeout_seconds"`
}

// LINE contains LINE notification settings
type LINE struct {
	Enabled                   bool   `json:"enabled" yaml:"enabled"`
	ChannelAccessToken        string `json:"channel_access_token" yaml:"channel_access_token"`
	ToIDs                     string `json:"to_ids" yaml:"to_ids"`
	SendImages                bool   `json:"send_images" yaml:"send_images"`
	S3BucketName              string `json:"s3_bucket_name" yaml:"s3_bucket_name"`
	S3AWSRegion               string `json:"s3_aws_region" yaml:"s3_aws_region"`
	S3UploadTimeoutSeconds    int    `json:"s3_upload_timeout_seconds" yaml:"s3_upload_timeout_seconds"`
}

// SNS contains AWS SNS notification settings
type SNS struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	TopicARN  string `json:"topic_arn" yaml:"topic_arn"`
	AWSRegion string `json:"aws_region" yaml:"aws_region"`
}

// AlertSound contains alert sound settings
type AlertSound struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Sound   string `json:"sound" yaml:"sound"`
}

// Forward contains data forwarding settings
type Forward struct {
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	Address      []string `json:"address" yaml:"address"`
	Port         []int    `json:"port" yaml:"port"`
	Channels     []string `json:"channels" yaml:"channels"`
	ForwardData  bool     `json:"fwd_data" yaml:"fwd_data"`
	ForwardAlarm bool     `json:"fwd_alarms" yaml:"fwd_alarms"`
}

// RSAM contains RSAM (Real-time Seismic Amplitude Measurement) settings
type RSAM struct {
	Enabled  bool    `json:"enabled" yaml:"enabled"`
	Quiet    bool    `json:"quiet" yaml:"quiet"`
	Interval int     `json:"interval" yaml:"interval"` // Interval in seconds
	Channel  string  `json:"channel" yaml:"channel"`
	Address  string  `json:"address" yaml:"address"`
	Port     int     `json:"port" yaml:"port"`
	Deconv   string  `json:"deconv" yaml:"deconv"`
}

// Custom contains custom module settings
type Custom struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	CodePath     string `json:"codepath" yaml:"codepath"`
	ExecCommand  string `json:"execcommand" yaml:"execcommand"`
	Args         string `json:"args" yaml:"args"`
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", filename)
	}
	
	// Read file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	
	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %v", err)
	}
	
	// Validate and set defaults
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %v", err)
	}
	
	config.SetDefaults()
	
	return &config, nil
}

// SaveConfig saves configuration to a JSON file
func (c *Config) SaveConfig(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %v", err)
	}
	
	// Write to file
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	
	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate port
	if c.Settings.Port < 1 || c.Settings.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Settings.Port)
	}
	
	// Validate station name
	if c.Settings.Station == "" {
		return fmt.Errorf("station name cannot be empty")
	}
	
	// Validate alert settings
	if c.Alert.Enabled {
		if c.Alert.STA <= 0 {
			return fmt.Errorf("alert STA must be positive")
		}
		if c.Alert.LTA <= 0 {
			return fmt.Errorf("alert LTA must be positive")
		}
		if c.Alert.STA >= c.Alert.LTA {
			return fmt.Errorf("alert STA must be less than LTA")
		}
		if c.Alert.Threshold <= 0 {
			return fmt.Errorf("alert threshold must be positive")
		}
		if c.Alert.Reset <= 0 {
			return fmt.Errorf("alert reset threshold must be positive")
		}
	}
	
	// Validate plot settings
	if c.Plot.Enabled {
		if c.Plot.Duration <= 0 {
			return fmt.Errorf("plot duration must be positive")
		}
	}
	
	return nil
}

// SetDefaults sets default values for unspecified configuration options
func (c *Config) SetDefaults() {
	// Settings defaults
	if c.Settings.Port == 0 {
		c.Settings.Port = 8888
	}
	if c.Settings.Network == "" {
		c.Settings.Network = "AM"
	}
	if c.Settings.Location == "" {
		c.Settings.Location = "00"
	}
	if c.Settings.OutDir == "" {
		c.Settings.OutDir = "./output"
	}
	if c.Settings.DataDir == "" {
		c.Settings.DataDir = "./data"
	}
	if c.Settings.ScreenshotDir == "" {
		c.Settings.ScreenshotDir = "./screenshots"
	}
	if c.Settings.LogDir == "" {
		c.Settings.LogDir = "./logs"
	}
	
	// Alert defaults
	if c.Alert.STA == 0 {
		c.Alert.STA = 5.0
	}
	if c.Alert.LTA == 0 {
		c.Alert.LTA = 30.0
	}
	if c.Alert.Threshold == 0 {
		c.Alert.Threshold = 1.6
	}
	if c.Alert.Reset == 0 {
		c.Alert.Reset = 1.55
	}
	if c.Alert.Channel == "" {
		c.Alert.Channel = "HZ"
	}
	
	// Plot defaults
	if c.Plot.Duration == 0 {
		c.Plot.Duration = 30
	}
	if len(c.Plot.Channels) == 0 {
		c.Plot.Channels = []string{"all"}
	}
	if c.Plot.RefreshInterval == 0 {
		c.Plot.RefreshInterval = 1000 // 1 second
	}
	
	// Write defaults
	if len(c.Write.Channels) == 0 {
		c.Write.Channels = []string{"all"}
	}
	if c.Write.OutDir == "" {
		c.Write.OutDir = c.Settings.DataDir
	}
}

// DefaultConfig returns a configuration with all default values
func DefaultConfig() *Config {
	config := &Config{
		Settings: Settings{
			Port:          8888,
			Station:       "Z0000",
			Network:       "AM",
			Location:      "00",
			OutDir:        "./output",
			DataDir:       "./data",
			ScreenshotDir: "./screenshots",
			LogDir:        "./logs",
			Debug:         false,
		},
		Alert: Alert{
			Enabled:   true,
			Channel:   "HZ",
			STA:       5.0,
			LTA:       30.0,
			Threshold: 1.6,
			Reset:     1.55,
			Highpass:  0.8,
			Lowpass:   9.0,
			Duration:  0.0,
			Deconv:    "CHAN",
			Units:     "CHAN",
		},
		Plot: Plot{
			Enabled:         false,
			Channels:        []string{"all"},
			Duration:        30,
			Spectrogram:     true,
			Fullscreen:      false,
			Kiosk:           false,
			EqScreenshots:   false,
			Deconv:          false,
			Units:           "CHAN",
			RefreshInterval: 1000,
		},
		Write: Write{
			Enabled:  false,
			Channels: []string{"all"},
			OutDir:   "./data",
		},
		PrintData: PrintData{
			Enabled: false,
		},
	}
	
	return config
}

// GetConfigDir returns the default configuration directory
func GetConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./config"
	}
	return filepath.Join(homeDir, ".gorsudp")
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(filename string) error {
	config := DefaultConfig()
	return config.SaveConfig(filename)
}