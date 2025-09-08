package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `
log_level = "debug"
bot_token = "test_token"
opencode_port = 8080

[[repositories]]
name = "test-repo"
path = "/path/to/repo"

[[models]]
provider_id = "openrouter"
model_id = "gpt-4"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Change working directory to temp dir so LoadConfig finds our test file
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test loading config
	err = LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify config was loaded correctly
	if AppConfig.LogLevel != "debug" {
		t.Errorf("Expected log_level 'debug', got '%s'", AppConfig.LogLevel)
	}

	if AppConfig.BotToken != "test_token" {
		t.Errorf("Expected bot_token 'test_token', got '%s'", AppConfig.BotToken)
	}

	if AppConfig.OpencodePort != 8080 {
		t.Errorf("Expected opencode_port 8080, got %d", AppConfig.OpencodePort)
	}

	if len(AppConfig.Repositories) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(AppConfig.Repositories))
	} else {
		if AppConfig.Repositories[0].Name != "test-repo" {
			t.Errorf("Expected repository name 'test-repo', got '%s'", AppConfig.Repositories[0].Name)
		}
		if AppConfig.Repositories[0].Path != "/path/to/repo" {
			t.Errorf("Expected repository path '/path/to/repo', got '%s'", AppConfig.Repositories[0].Path)
		}
	}

	if len(AppConfig.Models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(AppConfig.Models))
	} else {
		if AppConfig.Models[0].ProviderID != "openrouter" {
			t.Errorf("Expected model provider_id 'openrouter', got '%s'", AppConfig.Models[0].ProviderID)
		}
		if AppConfig.Models[0].ModelID != "gpt-4" {
			t.Errorf("Expected model_id 'gpt-4', got '%s'", AppConfig.Models[0].ModelID)
		}
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	// Change to a directory without config.toml
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	err := LoadConfig()
	if err == nil {
		t.Error("Expected error when config file is missing, got nil")
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Write invalid TOML content
	invalidContent := `
log_level = "debug"
[discord
token = "incomplete_section"
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	err = LoadConfig()
	if err == nil {
		t.Error("Expected error when config file has invalid TOML, got nil")
	}
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"default level", "invalid"},
		{"empty level", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This function doesn't return anything to test, but we can ensure it doesn't panic
			SetLogLevel(tt.level)
		})
	}
}
