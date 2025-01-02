package main

import (
	"net"
	"os"
	"testing"
	"time"
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

func TestServerAcceptsConnections(t *testing.T) {
	// Start server in a goroutine
	go func() {
		os.Setenv("IRC_HOST", "localhost")
		os.Setenv("IRC_PORT", "6668")
		main()
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect
	conn, err := net.Dial("tcp", "localhost:6668")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// If we got here, connection was successful
	// Send a test message
	testMsg := []byte("TEST\r\n")
	_, err = conn.Write(testMsg)
	if err != nil {
		t.Fatalf("Failed to write to connection: %v", err)
	}
}
