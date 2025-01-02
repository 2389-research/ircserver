package main

import (
	"context"
	"os"
	"testing"
	"time"

	"ircserver/internal/persistence"
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
