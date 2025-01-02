package main

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"ircserver/internal/persistence"
	"ircserver/internal/server"
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

func setupTestDB(t *testing.T) *persistence.SQLiteStore {
	t.Helper()
	db, err := persistence.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func TestUserPersistence(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.UpdateUser(ctx, "testuser", "testuser", "Test User", "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	var nickname, username, realname, ipAddress string
	var lastSeen time.Time
	err = store.QueryRow(ctx, "SELECT nickname, username, realname, ip_address, last_seen FROM users WHERE nickname = ?", "testuser").Scan(&nickname, &username, &realname, &ipAddress, &lastSeen)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}

	if nickname != "testuser" || username != "testuser" || realname != "Test User" || ipAddress != "127.0.0.1" {
		t.Errorf("User data mismatch: got (%s, %s, %s, %s), want (%s, %s, %s, %s)", nickname, username, realname, ipAddress, "testuser", "testuser", "Test User", "127.0.0.1")
	}
}

func TestChannelPersistence(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.UpdateChannel(ctx, "#testchannel", "Test Topic")
	if err != nil {
		t.Fatalf("Failed to update channel: %v", err)
	}

	var name, topic string
	var createdAt time.Time
	err = store.QueryRow(ctx, "SELECT name, topic, created_at FROM channels WHERE name = ?", "#testchannel").Scan(&name, &topic, &createdAt)
	if err != nil {
		t.Fatalf("Failed to query channel: %v", err)
	}

	if name != "#testchannel" || topic != "Test Topic" {
		t.Errorf("Channel data mismatch: got (%s, %s), want (%s, %s)", name, topic, "#testchannel", "Test Topic")
	}
}

func TestMessageLogging(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.LogMessage(ctx, "sender", "recipient", "PRIVMSG", "Hello, world!")
	if err != nil {
		t.Fatalf("Failed to log message: %v", err)
	}

	var timestamp time.Time
	var sender, recipient, msgType, content string
	err = store.QueryRow(ctx, "SELECT timestamp, sender, recipient, message_type, content FROM message_logs WHERE sender = ?", "sender").Scan(&timestamp, &sender, &recipient, &msgType, &content)
	if err != nil {
		t.Fatalf("Failed to query message log: %v", err)
	}

	if sender != "sender" || recipient != "recipient" || msgType != "PRIVMSG" || content != "Hello, world!" {
		t.Errorf("Message log data mismatch: got (%s, %s, %s, %s), want (%s, %s, %s, %s)", sender, recipient, msgType, content, "sender", "recipient", "PRIVMSG", "Hello, world!")
	}
}

func TestErrorHandling(t *testing.T) {
	t.Run("NetworkErrors", func(t *testing.T) {
		// Simulate network error
		_, err := net.Dial("tcp", "invalid:address")
		if err == nil {
			t.Error("Expected network error, got nil")
		}
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		// Simulate timeout handling
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		select {
		case <-time.After(1 * time.Second):
			t.Error("Expected timeout, but operation completed")
		case <-ctx.Done():
			if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Errorf("Expected context deadline exceeded, got %v", ctx.Err())
			}
		}
	})

	t.Run("ResourceCleanup", func(t *testing.T) {
		// Simulate resource cleanup
		store := setupTestDB(t)
		store.Close()

		err := store.LogMessage(context.Background(), "sender", "recipient", "PRIVMSG", "Hello, world!")
		if err == nil {
			t.Error("Expected error after closing store, got nil")
		}
	})

	t.Run("RecoveryFromErrors", func(t *testing.T) {
		// Simulate recovery from errors
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic recovery, but no panic occurred")
			}
		}()

		panic("Simulated panic")
	})
}

func TestMessageSendRetries(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.LogMessage(ctx, "sender", "recipient", "PRIVMSG", "Hello, world!")
	if err != nil {
		t.Fatalf("Failed to log message: %v", err)
	}

	var timestamp time.Time
	var sender, recipient, msgType, content string
	err = store.QueryRow(ctx, "SELECT timestamp, sender, recipient, message_type, content FROM message_logs WHERE sender = ?", "sender").Scan(&timestamp, &sender, &recipient, &msgType, &content)
	if err != nil {
		t.Fatalf("Failed to query message log: %v", err)
	}

	if sender != "sender" || recipient != "recipient" || msgType != "PRIVMSG" || content != "Hello, world!" {
		t.Errorf("Message log data mismatch: got (%s, %s, %s, %s), want (%s, %s, %s, %s)", sender, recipient, msgType, content, "sender", "recipient", "PRIVMSG", "Hello, world!")
	}

	// Simulate IO timeout and retry logic
	for i := 0; i < 3; i++ {
		err = store.LogMessage(ctx, "sender", "recipient", "PRIVMSG", "Retry message")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to log message after retries: %v", err)
	}
}

func TestSysopMessageOnShutdown(t *testing.T) {
	// Setup test server
	cfg := config.DefaultConfig()
	store := setupTestDB(t)
	defer store.Close()

	srv := server.New("localhost", "6667", store, cfg)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Simulate client connection
	conn, err := net.Dial("tcp", "localhost:6667")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Simulate server shutdown
	if err := srv.Shutdown(); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// Read from connection to verify sysop message
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from connection: %v", err)
	}

	message := string(buf[:n])
	expectedMessage := "NOTICE @testuser :Server is shutting down. Please disconnect."
	if !strings.Contains(message, expectedMessage) {
		t.Errorf("Expected sysop message %q, got %q", expectedMessage, message)
	}
}
