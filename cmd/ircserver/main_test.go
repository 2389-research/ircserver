package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		fallback string
		want     string
	}{
		{
			name:     "returns fallback when env not set",
			key:      "TEST_KEY",
			envValue: "",
			fallback: "default",
			want:     "default",
		},
		{
			name:     "returns env value when set",
			key:      "TEST_KEY",
			envValue: "custom",
			fallback: "default",
			want:     "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			if got := getEnv(tt.key, tt.fallback); got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessageHandling(t *testing.T) {
	t.Run("PRIVMSG to users", func(t *testing.T) {
		// Add test logic for PRIVMSG to users
	})

	t.Run("PRIVMSG to channels", func(t *testing.T) {
		// Add test logic for PRIVMSG to channels
	})

	t.Run("NOTICE handling", func(t *testing.T) {
		// Add test logic for NOTICE handling
	})

	t.Run("Message size limits", func(t *testing.T) {
		// Add test logic for message size limits
	})

	t.Run("Invalid message formats", func(t *testing.T) {
		// Add test logic for invalid message formats
	})

	t.Run("Message broadcasting performance", func(t *testing.T) {
		// Add test logic for message broadcasting performance
	})
}
