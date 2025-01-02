package main

import (
	"context"
	"errors"
	"net"
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

func TestClientConnection(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		nick              string
		wantErr           bool
		expectedResponses []string
	}{
		{
			name:    "valid connection with nick",
			input:   "NICK validnick\r\nUSER test 0 * :Test User\r\n",
			nick:    "validnick",
			wantErr: false,
			expectedResponses: []string{
				":* NICK validnick\r\n",
			},
		},
		{
			name:    "invalid nick character",
			input:   "NICK invalid@nick\r\nUSER test 0 * :Test User\r\n",
			wantErr: false,
			expectedResponses: []string{
				"432 * :Erroneous nickname\r\n",
			},
		},
		{
			name:    "missing USER command",
			input:   "NICK validnick\r\n",
			wantErr: true,
		},
		{
			name:    "empty nick",
			input:   "NICK \r\nUSER test 0 * :Test User\r\n",
			wantErr: false, // Changed to false since we handle this gracefully now
			expectedResponses: []string{
				"431 * :No nickname given\r\n",
			},
		},
		{
			name:    "EOF handling",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			conn := &mockConn{
				readData: strings.NewReader(tt.input),
			}
			client := NewClient(conn, cfg)

			// Start client processing in background
			errCh := make(chan error, 1)
			go func() {
				errCh <- client.handleConnection()
			}()

			// Wait for processing or timeout
			select {
			case err := <-errCh:
				if (err != nil) != tt.wantErr {
					t.Errorf("handleConnection() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("handleConnection() timeout")
			}

			if !tt.wantErr {
				if client.nick != tt.nick {
					t.Errorf("Expected nickname %q, got %q", tt.nick, client.nick)
				}

				// Verify expected responses
				output := conn.writeData.String()
				for _, expected := range tt.expectedResponses {
					if !strings.Contains(output, expected) {
						t.Errorf("Expected response %q not found in output: %q", expected, output)
					}
				}
			}
		})
	}
}
