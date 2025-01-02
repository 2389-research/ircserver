package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration settings.
type Config struct {
	Server struct {
		Name    string `yaml:"name"`
		Host    string `yaml:"host"`
		Port    string `yaml:"port"`
		WebPort string `yaml:"web_port"`
	} `yaml:"server"`
	Storage struct {
		LogPath    string `yaml:"log_path"`
		SQLitePath string `yaml:"sqlite_path"`
	} `yaml:"storage"`
	IRC struct {
		DefaultChannel   string        `yaml:"default_channel"`
		MaxMessageLength int           `yaml:"max_message_length"`
		ReadTimeout      time.Duration `yaml:"read_timeout"`
		WriteTimeout     time.Duration `yaml:"write_timeout"`
		MaxBufferSize    int           `yaml:"max_buffer_size"`
		IdleTimeout      time.Duration `yaml:"idle_timeout"`
		MaxRetries      int           `yaml:"max_retries"`
		RetryDelay      time.Duration `yaml:"retry_delay"`
	} `yaml:"irc"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	cfg := &Config{}
	if envName := os.Getenv("IRC_SERVER_NAME"); envName != "" {
		cfg.Server.Name = envName
	} else {
		cfg.Server.Name = "My IRC Server"
	}
	cfg.Server.Host = "localhost"
	cfg.Server.Port = "6667"
	cfg.Server.WebPort = "8080"
	cfg.Storage.LogPath = "irc.log"
	cfg.Storage.SQLitePath = "irc.db"
	cfg.IRC.DefaultChannel = "#general"
	cfg.IRC.MaxMessageLength = 512
	cfg.IRC.ReadTimeout = 300 * time.Second
	cfg.IRC.WriteTimeout = 60 * time.Second
	cfg.IRC.MaxBufferSize = 4096
	cfg.IRC.IdleTimeout = 600 * time.Second
	cfg.IRC.MaxRetries = 3
	cfg.IRC.RetryDelay = 1 * time.Second
	return cfg
}

// Load reads the configuration file and returns a Config struct.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// If no config file specified, return defaults
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return cfg, nil
}
