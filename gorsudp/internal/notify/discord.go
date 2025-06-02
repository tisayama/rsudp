package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tisayama/gorsudp/pkg/config"
)

// DiscordProvider implements NotificationProvider for Discord
type DiscordProvider struct {
	config      config.Discord
	client      *http.Client
	rateLimiter *RateLimiter
}

// DiscordWebhook represents a Discord webhook message
type DiscordWebhook struct {
	Content  string         `json:"content,omitempty"`
	Username string         `json:"username,omitempty"`
	Embeds   []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Image       *DiscordEmbedImage  `json:"image,omitempty"`
	Thumbnail   *DiscordEmbedImage  `json:"thumbnail,omitempty"`
	Author      *DiscordEmbedAuthor `json:"author,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
}

// DiscordEmbedFooter represents a Discord embed footer
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedImage represents a Discord embed image
type DiscordEmbedImage struct {
	URL string `json:"url"`
}

// DiscordEmbedAuthor represents a Discord embed author
type DiscordEmbedAuthor struct {
	Name    string `json:"name"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedField represents a Discord embed field
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Discord color constants
const (
	DiscordColorRed    = 0xFF0000 // Alert
	DiscordColorGreen  = 0x00FF00 // Reset
	DiscordColorBlue   = 0x0099FF // Info
	DiscordColorOrange = 0xFF9900 // Warning
)

// NewDiscordProvider creates a new Discord notification provider
func NewDiscordProvider(cfg config.Discord) *DiscordProvider {
	return &DiscordProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(5, time.Minute), // Discord allows 5 webhook calls per minute
	}
}

// GetName returns the provider name
func (dp *DiscordProvider) GetName() string {
	return "discord"
}

// IsEnabled returns whether the provider is enabled
func (dp *DiscordProvider) IsEnabled() bool {
	return dp.config.Enabled && dp.config.WebhookURL != ""
}

// SendText sends a text message via Discord
func (dp *DiscordProvider) SendText(message string) error {
	if !dp.IsEnabled() {
		return fmt.Errorf("discord provider is not enabled or configured")
	}

	// Check rate limits
	if !dp.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// Create webhook message
	webhook := DiscordWebhook{
		Username: "GoRSUDP Bot",
	}

	if dp.config.UseEmbed {
		// Send as embed
		embed := DiscordEmbed{
			Description: message,
			Color:       DiscordColorBlue,
			Timestamp:   time.Now().Format(time.RFC3339),
			Footer: &DiscordEmbedFooter{
				Text: "GoRSUDP Seismic Monitor",
			},
		}
		webhook.Embeds = []DiscordEmbed{embed}
	} else {
		// Send as plain text
		webhook.Content = message
	}

	// Send webhook
	return dp.sendWebhook(webhook)
}

// SendImage sends a message with an image via Discord
func (dp *DiscordProvider) SendImage(message string, imagePath string) error {
	if !dp.IsEnabled() {
		return fmt.Errorf("discord provider is not enabled or configured")
	}

	if !dp.config.SendImages {
		// If images are disabled, send text only
		return dp.SendText(message)
	}

	// Check rate limits
	if !dp.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// Check if image file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		// If image doesn't exist, send text only
		return dp.SendText(message + "\n\n(Image could not be attached)")
	}

	// Send image with message
	return dp.sendImageWebhook(message, imagePath)
}

// sendWebhook sends a Discord webhook
func (dp *DiscordProvider) sendWebhook(webhook DiscordWebhook) error {
	jsonData, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook: %v", err)
	}

	resp, err := dp.client.Post(dp.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord webhook failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendImageWebhook sends a Discord webhook with an image attachment
func (dp *DiscordProvider) sendImageWebhook(message string, imagePath string) error {
	// Open image file
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create webhook payload
	webhook := DiscordWebhook{
		Username: "GoRSUDP Bot",
	}

	if dp.config.UseEmbed {
		embed := DiscordEmbed{
			Title:       "🚨 Earthquake Alert",
			Description: message,
			Color:       DiscordColorRed,
			Timestamp:   time.Now().Format(time.RFC3339),
			Image: &DiscordEmbedImage{
				URL: "attachment://screenshot.png",
			},
			Footer: &DiscordEmbedFooter{
				Text: "GoRSUDP Seismic Monitor",
			},
		}
		webhook.Embeds = []DiscordEmbed{embed}
	} else {
		webhook.Content = message
	}

	// Add payload_json field
	payloadJSON, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %v", err)
	}

	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return fmt.Errorf("failed to write payload_json field: %v", err)
	}

	// Add file field
	filename := filepath.Base(imagePath)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %v", err)
	}

	writer.Close()

	// Send request
	resp, err := dp.client.Post(dp.config.WebhookURL, writer.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord webhook failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateAlertEmbed creates a Discord embed for earthquake alerts
func (dp *DiscordProvider) CreateAlertEmbed(alert Alert) DiscordEmbed {
	embed := DiscordEmbed{
		Title:       "🚨 Earthquake Alert",
		Description: fmt.Sprintf("Seismic activity detected on station %s.%s", alert.Network, alert.Station),
		Color:       DiscordColorRed,
		Timestamp:   alert.Timestamp.Format(time.RFC3339),
		Thumbnail: &DiscordEmbedImage{
			URL: "https://earthquake.usgs.gov/static/lfs/nshm/conterminous/images/PeakAcceleration.png",
		},
		Footer: &DiscordEmbedFooter{
			Text: "GoRSUDP Seismic Monitor",
		},
		Fields: []DiscordEmbedField{
			{
				Name:   "Channel",
				Value:  alert.Channel,
				Inline: true,
			},
			{
				Name:   "STA/LTA Ratio",
				Value:  fmt.Sprintf("%.2f", alert.STALTARatio),
				Inline: true,
			},
			{
				Name:   "Threshold",
				Value:  fmt.Sprintf("%.2f", alert.Threshold),
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  fmt.Sprintf("%.1f seconds", alert.Duration),
				Inline: true,
			},
			{
				Name:   "Time",
				Value:  alert.Timestamp.Format("2006-01-02 15:04:05 UTC"),
				Inline: false,
			},
		},
	}

	if alert.Location != "" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Location",
			Value:  alert.Location,
			Inline: true,
		})
	}

	if alert.Magnitude > 0 {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Magnitude",
			Value:  fmt.Sprintf("%.1f", alert.Magnitude),
			Inline: true,
		})
	}

	return embed
}

// CreateResetEmbed creates a Discord embed for alert resets
func (dp *DiscordProvider) CreateResetEmbed(reset Reset) DiscordEmbed {
	return DiscordEmbed{
		Title:       "✅ Alert Reset",
		Description: fmt.Sprintf("Alert conditions have ended on station %s.%s", reset.Network, reset.Station),
		Color:       DiscordColorGreen,
		Timestamp:   reset.Timestamp.Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "GoRSUDP Seismic Monitor",
		},
		Fields: []DiscordEmbedField{
			{
				Name:   "Channel",
				Value:  reset.Channel,
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  fmt.Sprintf("%.1f seconds", reset.Duration),
				Inline: true,
			},
			{
				Name:   "Time",
				Value:  reset.Timestamp.Format("2006-01-02 15:04:05 UTC"),
				Inline: false,
			},
		},
	}
}

// ValidateConfig validates the Discord configuration
func (dp *DiscordProvider) ValidateConfig() error {
	if !dp.config.Enabled {
		return nil // Not enabled, no validation needed
	}

	if dp.config.WebhookURL == "" {
		return fmt.Errorf("discord webhook URL is required")
	}

	// Basic webhook URL validation
	if len(dp.config.WebhookURL) < 50 || !contains(dp.config.WebhookURL, "discord.com/api/webhooks/") {
		return fmt.Errorf("invalid Discord webhook URL format")
	}

	return nil
}

// TestConnection tests the Discord webhook
func (dp *DiscordProvider) TestConnection() error {
	if !dp.IsEnabled() {
		return fmt.Errorf("discord provider is not enabled or configured")
	}

	// Send test message
	webhook := DiscordWebhook{
		Username: "GoRSUDP Bot",
		Content:  "🔧 GoRSUDP Discord integration test - connection successful!",
	}

	return dp.sendWebhook(webhook)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
