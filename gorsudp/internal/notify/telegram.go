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
	"strconv"
	"time"

	"github.com/tisayama/gorsudp/pkg/config"
)

// TelegramProvider implements NotificationProvider for Telegram
type TelegramProvider struct {
	config      config.Telegram
	client      *http.Client
	rateLimiter *RateLimiter
}

// TelegramMessage represents a Telegram message request
type TelegramMessage struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// TelegramResponse represents a Telegram API response
type TelegramResponse struct {
	OK          bool        `json:"ok"`
	Result      interface{} `json:"result,omitempty"`
	ErrorCode   int         `json:"error_code,omitempty"`
	Description string      `json:"description,omitempty"`
}

// RateLimiter implements simple rate limiting
type RateLimiter struct {
	requests []time.Time
	limit    int
	window   time.Duration
}

// NewTelegramProvider creates a new Telegram notification provider
func NewTelegramProvider(cfg config.Telegram) *TelegramProvider {
	return &TelegramProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(20, time.Minute), // Telegram allows 20 messages per minute per bot
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make([]time.Time, 0),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed under rate limits
func (rl *RateLimiter) Allow() bool {
	now := time.Now()

	// Remove old requests outside the window
	cutoff := now.Add(-rl.window)
	newRequests := make([]time.Time, 0)
	for _, req := range rl.requests {
		if req.After(cutoff) {
			newRequests = append(newRequests, req)
		}
	}
	rl.requests = newRequests

	// Check if we're under the limit
	if len(rl.requests) >= rl.limit {
		return false
	}

	// Add this request
	rl.requests = append(rl.requests, now)
	return true
}

// GetName returns the provider name
func (tp *TelegramProvider) GetName() string {
	return "telegram"
}

// IsEnabled returns whether the provider is enabled
func (tp *TelegramProvider) IsEnabled() bool {
	return tp.config.Enabled && tp.config.Token != "" && tp.config.ChatID != ""
}

// SendText sends a text message via Telegram
func (tp *TelegramProvider) SendText(message string) error {
	if !tp.IsEnabled() {
		return fmt.Errorf("telegram provider is not enabled or configured")
	}

	// Check rate limits
	if !tp.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// Prepare message
	msg := TelegramMessage{
		ChatID:                tp.config.ChatID,
		Text:                  message,
		ParseMode:             "Markdown",
		DisableWebPagePreview: true,
	}

	// Send message
	return tp.sendMessage(msg)
}

// SendImage sends a message with an image via Telegram
func (tp *TelegramProvider) SendImage(message string, imagePath string) error {
	if !tp.IsEnabled() {
		return fmt.Errorf("telegram provider is not enabled or configured")
	}

	if !tp.config.SendImages {
		// If images are disabled, send text only
		return tp.SendText(message)
	}

	// Check rate limits
	if !tp.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// Check if image file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		// If image doesn't exist, send text only
		return tp.SendText(message + "\n\n(Image could not be attached)")
	}

	// Send image with caption
	return tp.sendPhoto(message, imagePath)
}

// sendMessage sends a text message to Telegram
func (tp *TelegramProvider) sendMessage(msg TelegramMessage) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tp.config.Token)

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := tp.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error: %s (code: %d)",
			telegramResp.Description, telegramResp.ErrorCode)
	}

	return nil
}

// sendPhoto sends a photo with caption to Telegram
func (tp *TelegramProvider) sendPhoto(caption string, imagePath string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", tp.config.Token)

	// Open image file
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %v", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add chat_id field
	if err := writer.WriteField("chat_id", tp.config.ChatID); err != nil {
		return fmt.Errorf("failed to write chat_id field: %v", err)
	}

	// Add caption field
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("failed to write caption field: %v", err)
		}
		if err := writer.WriteField("parse_mode", "Markdown"); err != nil {
			return fmt.Errorf("failed to write parse_mode field: %v", err)
		}
	}

	// Add photo field
	filename := filepath.Base(imagePath)
	part, err := writer.CreateFormFile("photo", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %v", err)
	}

	writer.Close()

	// Send request
	resp, err := tp.client.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error: %s (code: %d)",
			telegramResp.Description, telegramResp.ErrorCode)
	}

	return nil
}

// ValidateConfig validates the Telegram configuration
func (tp *TelegramProvider) ValidateConfig() error {
	if !tp.config.Enabled {
		return nil // Not enabled, no validation needed
	}

	if tp.config.Token == "" {
		return fmt.Errorf("telegram token is required")
	}

	if tp.config.ChatID == "" {
		return fmt.Errorf("telegram chat_id is required")
	}

	// Validate chat_id format (should be numeric or start with @)
	if tp.config.ChatID[0] != '@' {
		if _, err := strconv.ParseInt(tp.config.ChatID, 10, 64); err != nil {
			return fmt.Errorf("invalid chat_id format: must be numeric or start with @")
		}
	}

	return nil
}

// TestConnection tests the Telegram connection
func (tp *TelegramProvider) TestConnection() error {
	if !tp.IsEnabled() {
		return fmt.Errorf("telegram provider is not enabled or configured")
	}

	// Test with getMe API call
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", tp.config.Token)

	resp, err := tp.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error: %s (code: %d)",
			telegramResp.Description, telegramResp.ErrorCode)
	}

	return nil
}
