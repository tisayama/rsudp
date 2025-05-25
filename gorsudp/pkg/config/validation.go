package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s (value: %s)", e.Field, e.Message, e.Value)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors"`
	Warnings []ValidationError `json:"warnings"`
}

// ValidateComprehensive performs comprehensive validation of the configuration
func (c *Config) ValidateComprehensive() *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationError, 0),
	}

	// Validate settings
	c.validateSettings(result)

	// Validate alert configuration
	c.validateAlert(result)

	// Validate plot configuration
	c.validatePlot(result)

	// Validate write configuration
	c.validateWrite(result)

	// Validate notification configurations
	c.validateNotifications(result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// validateSettings validates general settings
func (c *Config) validateSettings(result *ValidationResult) {
	// Port validation
	if c.Settings.Port < 1 || c.Settings.Port > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "settings.port",
			Value:   strconv.Itoa(c.Settings.Port),
			Message: "port must be between 1 and 65535",
		})
	}

	// Well-known ports warning
	if c.Settings.Port < 1024 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "settings.port",
			Value:   strconv.Itoa(c.Settings.Port),
			Message: "using privileged port (< 1024), ensure proper permissions",
		})
	}

	// Station code validation
	if !isValidStationCode(c.Settings.Station) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "settings.station",
			Value:   c.Settings.Station,
			Message: "station code must be 3-5 alphanumeric characters",
		})
	}

	// Network code validation
	if !isValidNetworkCode(c.Settings.Network) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "settings.network",
			Value:   c.Settings.Network,
			Message: "network code must be 1-2 uppercase letters",
		})
	}

	// Directory validation
	c.validateDirectory("settings.output_dir", c.Settings.OutDir, result)
	c.validateDirectory("settings.data_dir", c.Settings.DataDir, result)
	c.validateDirectory("settings.screenshot_dir", c.Settings.ScreenshotDir, result)
	c.validateDirectory("settings.log_dir", c.Settings.LogDir, result)
}

// validateAlert validates alert configuration
func (c *Config) validateAlert(result *ValidationResult) {
	if !c.Alert.Enabled {
		return
	}

	// STA/LTA validation
	if c.Alert.STA <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.sta",
			Value:   fmt.Sprintf("%.2f", c.Alert.STA),
			Message: "STA window must be positive",
		})
	}

	if c.Alert.LTA <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.lta",
			Value:   fmt.Sprintf("%.2f", c.Alert.LTA),
			Message: "LTA window must be positive",
		})
	}

	if c.Alert.STA >= c.Alert.LTA {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.sta",
			Value:   fmt.Sprintf("%.2f", c.Alert.STA),
			Message: "STA window must be smaller than LTA window",
		})
	}

	// Threshold validation
	if c.Alert.Threshold <= 1.0 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "alert.threshold",
			Value:   fmt.Sprintf("%.2f", c.Alert.Threshold),
			Message: "threshold <= 1.0 may cause false positives",
		})
	}

	if c.Alert.Reset >= c.Alert.Threshold {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.reset",
			Value:   fmt.Sprintf("%.2f", c.Alert.Reset),
			Message: "reset threshold must be lower than trigger threshold",
		})
	}

	// Filter validation
	if c.Alert.Highpass < 0 || c.Alert.Lowpass < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.highpass/lowpass",
			Value:   fmt.Sprintf("%.2f/%.2f", c.Alert.Highpass, c.Alert.Lowpass),
			Message: "filter frequencies must be non-negative",
		})
	}

	if c.Alert.Highpass > 0 && c.Alert.Lowpass > 0 && c.Alert.Highpass >= c.Alert.Lowpass {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.highpass",
			Value:   fmt.Sprintf("%.2f", c.Alert.Highpass),
			Message: "highpass frequency must be lower than lowpass frequency",
		})
	}

	// Channel validation
	if !isValidChannelCode(c.Alert.Channel) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "alert.channel",
			Value:   c.Alert.Channel,
			Message: "invalid channel code format",
		})
	}
}

// validatePlot validates plot configuration
func (c *Config) validatePlot(result *ValidationResult) {
	if !c.Plot.Enabled {
		return
	}

	// Port validation
	if c.Plot.Port < 1 || c.Plot.Port > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "plot.port",
			Value:   strconv.Itoa(c.Plot.Port),
			Message: "port must be between 1 and 65535",
		})
	}

	// Host validation
	if c.Plot.Host != "" && !isValidHost(c.Plot.Host) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "plot.host",
			Value:   c.Plot.Host,
			Message: "invalid host format",
		})
	}

	// Duration validation
	if c.Plot.Duration < 1 || c.Plot.Duration > 3600 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "plot.duration",
			Value:   strconv.Itoa(c.Plot.Duration),
			Message: "duration should be between 1 and 3600 seconds",
		})
	}

	// Refresh interval validation
	if c.Plot.RefreshInterval < 100 || c.Plot.RefreshInterval > 60000 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "plot.refresh_interval",
			Value:   strconv.Itoa(c.Plot.RefreshInterval),
			Message: "refresh interval should be between 100ms and 60s",
		})
	}

	// Channel validation
	for _, channel := range c.Plot.Channels {
		if channel != "all" && !isValidChannelCode(channel) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "plot.channels",
				Value:   channel,
				Message: "invalid channel code",
			})
		}
	}
}

// validateWrite validates write configuration
func (c *Config) validateWrite(result *ValidationResult) {
	if !c.Write.Enabled {
		return
	}

	// Output directory validation
	c.validateDirectory("write.outdir", c.Write.OutDir, result)

	// Channel validation
	for _, channel := range c.Write.Channels {
		if channel != "all" && !isValidChannelCode(channel) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "write.channels",
				Value:   channel,
				Message: "invalid channel code",
			})
		}
	}
}

// validateNotifications validates notification configurations
func (c *Config) validateNotifications(result *ValidationResult) {
	// Telegram validation
	if c.Telegram.Enabled {
		if c.Telegram.Token == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "telegram.token",
				Value:   "",
				Message: "token is required when Telegram is enabled",
			})
		}

		if c.Telegram.ChatID == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "telegram.chat_id",
				Value:   "",
				Message: "chat_id is required when Telegram is enabled",
			})
		} else if !isValidTelegramChatID(c.Telegram.ChatID) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "telegram.chat_id",
				Value:   c.Telegram.ChatID,
				Message: "chat_id must be numeric or start with @",
			})
		}
	}

	// Discord validation
	if c.Discord.Enabled {
		if c.Discord.WebhookURL == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "discord.webhook_url",
				Value:   "",
				Message: "webhook URL is required when Discord is enabled",
			})
		} else if !isValidDiscordWebhook(c.Discord.WebhookURL) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "discord.webhook_url",
				Value:   c.Discord.WebhookURL,
				Message: "invalid Discord webhook URL format",
			})
		}
	}

	// Twitter validation
	if c.Twitter.Enabled {
		if c.Twitter.ConsumerKey == "" || c.Twitter.ConsumerSecret == "" ||
			c.Twitter.AccessToken == "" || c.Twitter.AccessSecret == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "twitter",
				Value:   "",
				Message: "all Twitter credentials are required when Twitter is enabled",
			})
		}
	}
}

// Helper validation functions

func isValidStationCode(code string) bool {
	if len(code) < 3 || len(code) > 5 {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Z0-9]+$`, code)
	return matched
}

func isValidNetworkCode(code string) bool {
	if len(code) < 1 || len(code) > 2 {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Z]+$`, code)
	return matched
}

func isValidChannelCode(code string) bool {
	if code == "all" {
		return true
	}
	// Support wildcards
	if strings.HasSuffix(code, "*") {
		code = strings.TrimSuffix(code, "*")
	}
	if len(code) < 1 || len(code) > 3 {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Z0-9]+$`, code)
	return matched
}

func isValidHost(host string) bool {
	// Check if it's a valid IP address
	if net.ParseIP(host) != nil {
		return true
	}

	// Check if it's a valid hostname
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+$`, host)
	return matched && len(host) <= 253
}

func isValidTelegramChatID(chatID string) bool {
	// Check if it starts with @ (username)
	if strings.HasPrefix(chatID, "@") {
		return len(chatID) > 1
	}

	// Check if it's numeric (including negative for groups)
	_, err := strconv.ParseInt(chatID, 10, 64)
	return err == nil
}

func isValidDiscordWebhook(webhookURL string) bool {
	u, err := url.Parse(webhookURL)
	if err != nil {
		return false
	}

	return u.Scheme == "https" &&
		strings.Contains(u.Host, "discord") &&
		strings.Contains(u.Path, "/webhooks/")
}

func (c *Config) validateDirectory(field, dir string, result *ValidationResult) {
	if dir == "" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   field,
			Value:   "",
			Message: "directory path is empty",
		})
		return
	}

	// Check if path is absolute for production
	if !filepath.IsAbs(dir) {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   field,
			Value:   dir,
			Message: "relative path may cause issues in production",
		})
	}

	// Check if directory exists or can be created
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   field,
			Value:   dir,
			Message: fmt.Sprintf("cannot create directory: %v", err),
		})
	}

	// Check write permissions
	testFile := filepath.Join(dir, ".gorsudp_test")
	if file, err := os.Create(testFile); err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   field,
			Value:   dir,
			Message: "directory is not writable",
		})
	} else {
		file.Close()
		os.Remove(testFile)
	}
}

// ValidateAndReport validates configuration and prints a detailed report
func (c *Config) ValidateAndReport() error {
	result := c.ValidateComprehensive()

	if len(result.Warnings) > 0 {
		fmt.Println("Configuration Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  ⚠️  %s\n", warning.Error())
		}
		fmt.Println()
	}

	if len(result.Errors) > 0 {
		fmt.Println("Configuration Errors:")
		for _, err := range result.Errors {
			fmt.Printf("  ❌ %s\n", err.Error())
		}
		fmt.Println()
		return fmt.Errorf("configuration validation failed with %d errors", len(result.Errors))
	}

	fmt.Println("✅ Configuration validation passed")
	return nil
}

// GetValidationSummary returns a summary of validation results
func (r *ValidationResult) GetValidationSummary() string {
	if r.Valid {
		return fmt.Sprintf("✅ Valid (0 errors, %d warnings)", len(r.Warnings))
	}
	return fmt.Sprintf("❌ Invalid (%d errors, %d warnings)", len(r.Errors), len(r.Warnings))
}
