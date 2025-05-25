package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := ioutil.TempDir("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "test_config.json")

	// Create a basic config
	testConfig := Config{
		Settings: Settings{
			Port:    8888,
			Station: "TEST",
			Network: "AM",
			Debug:   true,
		},
		Alert: Alert{
			Enabled:   true,
			Channel:   "HZ",
			STA:       5.0,
			LTA:       30.0,
			Threshold: 1.6,
			Reset:     1.55,
		},
	}

	// Write config to file
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = ioutil.WriteFile(configFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading the config
	loadedConfig, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedConfig.Settings.Port != 8888 {
		t.Errorf("Port = %d, want 8888", loadedConfig.Settings.Port)
	}

	if loadedConfig.Settings.Station != "TEST" {
		t.Errorf("Station = %s, want TEST", loadedConfig.Settings.Station)
	}

	if !loadedConfig.Alert.Enabled {
		t.Error("Alert should be enabled")
	}

	if loadedConfig.Alert.STA != 5.0 {
		t.Errorf("STA = %f, want 5.0", loadedConfig.Alert.STA)
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("LoadConfig should fail for non-existent file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	// Create a temporary invalid JSON file
	tmpDir, err := ioutil.TempDir("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "invalid_config.json")

	// Write invalid JSON
	err = ioutil.WriteFile(configFile, []byte("invalid json content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Error("LoadConfig should fail for invalid JSON")
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "save_test.json")

	testConfig := Config{
		Settings: Settings{
			Port:    9999,
			Station: "SAVE_TEST",
			Network: "XX",
			Debug:   false,
		},
		Alert: Alert{
			Enabled:   false,
			Channel:   "HN",
			STA:       10.0,
			LTA:       50.0,
			Threshold: 2.0,
			Reset:     1.8,
		},
	}

	// Test saving config
	err = testConfig.SaveConfig(configFile)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify content
	loadedConfig, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Settings.Port != 9999 {
		t.Errorf("Saved port = %d, want 9999", loadedConfig.Settings.Port)
	}

	if loadedConfig.Settings.Station != "SAVE_TEST" {
		t.Errorf("Saved station = %s, want SAVE_TEST", loadedConfig.Settings.Station)
	}

	if loadedConfig.Alert.Enabled {
		t.Error("Alert should be disabled in saved config")
	}
}

func TestSaveConfig_InvalidPath(t *testing.T) {
	testConfig := Config{}

	// Try to save to an invalid path
	err := testConfig.SaveConfig("/invalid/path/that/does/not/exist/config.json")
	if err == nil {
		t.Error("SaveConfig should fail for invalid path")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check some default values
	if config.Settings.Port != 8888 {
		t.Errorf("Default port = %d, want 8888", config.Settings.Port)
	}

	if config.Settings.Station == "" {
		t.Error("Default station should not be empty")
	}

	if config.Settings.Network == "" {
		t.Error("Default network should not be empty")
	}

	// Alert should be enabled by default
	if !config.Alert.Enabled {
		t.Error("Default alert should be enabled")
	}

	if config.Alert.STA <= 0 {
		t.Errorf("Default STA = %f, should be positive", config.Alert.STA)
	}

	if config.Alert.LTA <= 0 {
		t.Errorf("Default LTA = %f, should be positive", config.Alert.LTA)
	}

	if config.Alert.STA >= config.Alert.LTA {
		t.Errorf("Default STA (%f) should be less than LTA (%f)", config.Alert.STA, config.Alert.LTA)
	}
}

func TestGetConfigDir(t *testing.T) {
	configDir := GetConfigDir()

	if configDir == "" {
		t.Error("Config dir should not be empty")
	}

	// Check if it's an absolute path
	if !filepath.IsAbs(configDir) {
		t.Errorf("Config dir should be absolute path, got: %s", configDir)
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "default_config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "default_config.json")

	err = CreateDefaultConfig(configFile)
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Default config file was not created")
	}

	// Load and verify it's valid
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load generated default config: %v", err)
	}

	if config.Settings.Port != 8888 {
		t.Errorf("Generated default port = %d, want 8888", config.Settings.Port)
	}

	// Try to generate again (might succeed depending on implementation)
	err = CreateDefaultConfig(configFile)
	// Note: CreateDefaultConfig might overwrite existing files, so we don't test for failure
}

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()

	if path == "" {
		t.Error("Default config path should not be empty")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Default config path should be absolute, got: %s", path)
	}

	if filepath.Ext(path) != ".json" {
		t.Errorf("Default config path should end with .json, got: %s", path)
	}
}

func TestConfig_JSON_Marshall_Unmarshall(t *testing.T) {
	config := DefaultConfig()

	// Marshal to JSON
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	// Unmarshal from JSON
	var newConfig Config
	err = json.Unmarshal(data, &newConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal config from JSON: %v", err)
	}

	// Compare some key values
	if newConfig.Settings.Port != config.Settings.Port {
		t.Errorf("Port mismatch after JSON round-trip: got %d, want %d",
			newConfig.Settings.Port, config.Settings.Port)
	}

	if newConfig.Alert.STA != config.Alert.STA {
		t.Errorf("STA mismatch after JSON round-trip: got %f, want %f",
			newConfig.Alert.STA, config.Alert.STA)
	}

	if newConfig.Alert.Enabled != config.Alert.Enabled {
		t.Errorf("Alert enabled mismatch after JSON round-trip: got %v, want %v",
			newConfig.Alert.Enabled, config.Alert.Enabled)
	}
}

func BenchmarkLoadConfig(b *testing.B) {
	// Create a temporary config file
	tmpDir, err := ioutil.TempDir("", "config_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "bench_config.json")

	// Generate default config file
	err = CreateDefaultConfig(configFile)
	if err != nil {
		b.Fatalf("Failed to generate config file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadConfig(configFile)
		if err != nil {
			b.Fatalf("LoadConfig failed: %v", err)
		}
	}
}

func BenchmarkSaveConfig(b *testing.B) {
	tmpDir, err := ioutil.TempDir("", "config_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configFile := filepath.Join(tmpDir, fmt.Sprintf("bench_config_%d.json", i))
		err := config.SaveConfig(configFile)
		if err != nil {
			b.Fatalf("SaveConfig failed: %v", err)
		}
	}
}
