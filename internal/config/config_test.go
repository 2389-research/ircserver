package config

import (
	"os"
	"testing"
	"time"
)

func TestValidConfigLoading(t *testing.T) {
	cfg, err := Load("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	if cfg.Server.Name != "My IRC Server" {
		t.Errorf("Expected server name 'My IRC Server', got '%s'", cfg.Server.Name)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected server host 'localhost', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != "6667" {
		t.Errorf("Expected server port '6667', got '%s'", cfg.Server.Port)
	}
	if cfg.Server.WebPort != "8080" {
		t.Errorf("Expected server web port '8080', got '%s'", cfg.Server.WebPort)
	}
	if cfg.Storage.LogPath != "irc.log" {
		t.Errorf("Expected log path 'irc.log', got '%s'", cfg.Storage.LogPath)
	}
	if cfg.Storage.SQLitePath != "irc.db" {
		t.Errorf("Expected SQLite path 'irc.db', got '%s'", cfg.Storage.SQLitePath)
	}
	if cfg.IRC.DefaultChannel != "#general" {
		t.Errorf("Expected default channel '#general', got '%s'", cfg.IRC.DefaultChannel)
	}
	if cfg.IRC.MaxMessageLength != 512 {
		t.Errorf("Expected max message length 512, got %d", cfg.IRC.MaxMessageLength)
	}
}

func TestInvalidConfigHandling(t *testing.T) {
	_, err := Load("testdata/invalid_config.yaml")
	if err == nil {
		t.Fatal("Expected error for invalid config, got nil")
	}
}

func TestDefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Name != "IRC Server" {
		t.Errorf("Expected default server name 'IRC Server', got '%s'", cfg.Server.Name)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected default server host 'localhost', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != "6667" {
		t.Errorf("Expected default server port '6667', got '%s'", cfg.Server.Port)
	}
	if cfg.Server.WebPort != "8080" {
		t.Errorf("Expected default server web port '8080', got '%s'", cfg.Server.WebPort)
	}
	if cfg.Storage.LogPath != "irc.log" {
		t.Errorf("Expected default log path 'irc.log', got '%s'", cfg.Storage.LogPath)
	}
	if cfg.Storage.SQLitePath != "irc.db" {
		t.Errorf("Expected default SQLite path 'irc.db', got '%s'", cfg.Storage.SQLitePath)
	}
	if cfg.IRC.DefaultChannel != "#general" {
		t.Errorf("Expected default channel '#general', got '%s'", cfg.IRC.DefaultChannel)
	}
	if cfg.IRC.MaxMessageLength != 512 {
		t.Errorf("Expected default max message length 512, got %d", cfg.IRC.MaxMessageLength)
	}
	if cfg.IRC.ReadTimeout != 300*time.Second {
		t.Errorf("Expected default read timeout 300s, got %v", cfg.IRC.ReadTimeout)
	}
	if cfg.IRC.WriteTimeout != 60*time.Second {
		t.Errorf("Expected default write timeout 60s, got %v", cfg.IRC.WriteTimeout)
	}
	if cfg.IRC.MaxBufferSize != 4096 {
		t.Errorf("Expected default max buffer size 4096, got %d", cfg.IRC.MaxBufferSize)
	}
	if cfg.IRC.IdleTimeout != 600*time.Second {
		t.Errorf("Expected default idle timeout 600s, got %v", cfg.IRC.IdleTimeout)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("IRC_SERVER_NAME", "Env IRC Server")
	defer os.Unsetenv("IRC_SERVER_NAME")

	cfg := DefaultConfig()
	if cfg.Server.Name != "Env IRC Server" {
		t.Errorf("Expected server name 'Env IRC Server', got '%s'", cfg.Server.Name)
	}
}

func TestConfigReloading(t *testing.T) {
	// Create testdata directory if it doesn't exist
	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatalf("Failed to create testdata directory: %v", err)
	}
	
	cfg, err := Load("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	// Modify the config file
	newConfigContent := `
server:
  name: "New IRC Server"
  host: "127.0.0.1"
  port: "6668"
  web_port: "8081"
storage:
  log_path: "new_irc.log"
  sqlite_path: "new_irc.db"
irc:
  default_channel: "#newchannel"
  max_message_length: 1024
`
	err = os.WriteFile("testdata/valid_config.yaml", []byte(newConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify config file: %v", err)
	}

	// Reload the config
	cfg, err = Load("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if cfg.Server.Name != "New IRC Server" {
		t.Errorf("Expected server name 'New IRC Server', got '%s'", cfg.Server.Name)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected server host '127.0.0.1', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != "6668" {
		t.Errorf("Expected server port '6668', got '%s'", cfg.Server.Port)
	}
	if cfg.Server.WebPort != "8081" {
		t.Errorf("Expected server web port '8081', got '%s'", cfg.Server.WebPort)
	}
	if cfg.Storage.LogPath != "new_irc.log" {
		t.Errorf("Expected log path 'new_irc.log', got '%s'", cfg.Storage.LogPath)
	}
	if cfg.Storage.SQLitePath != "new_irc.db" {
		t.Errorf("Expected SQLite path 'new_irc.db', got '%s'", cfg.Storage.SQLitePath)
	}
	if cfg.IRC.DefaultChannel != "#newchannel" {
		t.Errorf("Expected default channel '#newchannel', got '%s'", cfg.IRC.DefaultChannel)
	}
	if cfg.IRC.MaxMessageLength != 1024 {
		t.Errorf("Expected max message length 1024, got %d", cfg.IRC.MaxMessageLength)
	}

	// Cleanup
	if err := os.Remove("testdata/valid_config.yaml"); err != nil {
		t.Logf("Warning: Failed to cleanup test config file: %v", err)
	}
}
