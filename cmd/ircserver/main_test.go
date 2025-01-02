package main

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"ircserver/internal/persistence"
	"ircserver/internal/config"
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

func TestIntegration(t *testing.T) {
	t.Run("FullClientLifecycle", func(t *testing.T) {
		// Setup server and client
		cfg := config.DefaultConfig()
		store := setupTestDB(t)
		srv := server.New("localhost", "6667", store, cfg)
		go srv.Start()
		defer srv.Shutdown()

		conn, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Simulate client actions
		client := server.NewClient(conn, cfg)
		client.Send("NICK testuser")
		client.Send("USER testuser 0 * :Test User")
		client.Send("JOIN #testchannel")
		client.Send("PRIVMSG #testchannel :Hello, world!")
		client.Send("QUIT")

		// Verify server state
		if len(srv.Clients()) != 0 {
			t.Errorf("Expected no clients, got %d", len(srv.Clients()))
		}
		if len(srv.Channels()) != 0 {
			t.Errorf("Expected no channels, got %d", len(srv.Channels()))
		}
	})

	t.Run("MultipleClientsInteraction", func(t *testing.T) {
		// Setup server and clients
		cfg := config.DefaultConfig()
		store := setupTestDB(t)
		srv := server.New("localhost", "6667", store, cfg)
		go srv.Start()
		defer srv.Shutdown()

		conn1, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn1.Close()

		conn2, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn2.Close()

		// Simulate client actions
		client1 := server.NewClient(conn1, cfg)
		client1.Send("NICK user1")
		client1.Send("USER user1 0 * :User One")
		client1.Send("JOIN #testchannel")

		client2 := server.NewClient(conn2, cfg)
		client2.Send("NICK user2")
		client2.Send("USER user2 0 * :User Two")
		client2.Send("JOIN #testchannel")
		client2.Send("PRIVMSG #testchannel :Hello from user2")

		// Verify interaction
		if len(srv.Clients()) != 2 {
			t.Errorf("Expected 2 clients, got %d", len(srv.Clients()))
		}
		if len(srv.Channels()) != 1 {
			t.Errorf("Expected 1 channel, got %d", len(srv.Channels()))
		}
	})

	t.Run("PersistenceAcrossRestarts", func(t *testing.T) {
		// Setup server and client
		cfg := config.DefaultConfig()
		store := setupTestDB(t)
		srv := server.New("localhost", "6667", store, cfg)
		go srv.Start()

		conn, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		// Simulate client actions
		client := server.NewClient(conn, cfg)
		client.Send("NICK testuser")
		client.Send("USER testuser 0 * :Test User")
		client.Send("JOIN #testchannel")
		client.Send("PRIVMSG #testchannel :Hello, world!")

		// Shutdown and restart server
		srv.Shutdown()
		srv = server.New("localhost", "6667", store, cfg)
		go srv.Start()
		defer srv.Shutdown()

		// Verify persistence
		if len(srv.Clients()) != 0 {
			t.Errorf("Expected no clients, got %d", len(srv.Clients()))
		}
		if len(srv.Channels()) != 1 {
			t.Errorf("Expected 1 channel, got %d", len(srv.Channels()))
		}
	})

	t.Run("WebInterfaceIntegration", func(t *testing.T) {
		// Setup server and web interface
		cfg := config.DefaultConfig()
		store := setupTestDB(t)
		srv := server.New("localhost", "6667", store, cfg)
		webSrv, err := server.NewWebServer(srv)
		if err != nil {
			t.Fatalf("Failed to create web server: %v", err)
		}
		go srv.Start()
		go webSrv.Start(":8080")
		defer srv.Shutdown()
		defer webSrv.Shutdown()

		// Simulate web interface actions
		resp, err := http.Get("http://localhost:8080/api/data")
		if err != nil {
			t.Fatalf("Failed to get web interface data: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %v", resp.Status)
		}
	})

	t.Run("RealIRCClients", func(t *testing.T) {
		// This test requires real IRC clients and is not implemented here
		t.Skip("Real IRC clients integration test is not implemented")
	})
}
