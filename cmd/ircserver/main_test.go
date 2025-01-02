package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"ircserver/internal/config"
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

func TestIntegration(t *testing.T) {
	cfg := config.DefaultConfig()
	store, err := persistence.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize in-memory database: %v", err)
	}
	defer store.Close()

	srv := server.New(cfg.Server.Host, cfg.Server.Port, store, cfg)
	go func() {
		if err := srv.Start(); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
	}()
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond) // Give server time to start

	t.Run("TestFullClientLifecycle", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte("NICK testuser\r\nUSER test 0 * :Test User\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Give server time to process

		// Check if user is registered
		if _, exists := srv.Clients["testuser"]; !exists {
			t.Fatalf("User not registered")
		}
	})

	t.Run("TestMultipleClientsInteraction", func(t *testing.T) {
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

		_, err = conn1.Write([]byte("NICK user1\r\nUSER test 0 * :User 1\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		_, err = conn2.Write([]byte("NICK user2\r\nUSER test 0 * :User 2\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Give server time to process

		// Check if users are registered
		if _, exists := srv.Clients["user1"]; !exists {
			t.Fatalf("User1 not registered")
		}
		if _, exists := srv.Clients["user2"]; !exists {
			t.Fatalf("User2 not registered")
		}

		_, err = conn1.Write([]byte("JOIN #test\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		_, err = conn2.Write([]byte("JOIN #test\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Give server time to process

		// Check if users are in the channel
		channel, exists := srv.Channels["#test"]
		if !exists {
			t.Fatalf("Channel not created")
		}
		if !channel.HasClient("user1") {
			t.Fatalf("User1 not in channel")
		}
		if !channel.HasClient("user2") {
			t.Fatalf("User2 not in channel")
		}
	})

	t.Run("TestPersistenceAcrossRestarts", func(t *testing.T) {
		conn, err := net.Dial("tcp", "localhost:6667")
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte("NICK persistentuser\r\nUSER test 0 * :Persistent User\r\n"))
		if err != nil {
			t.Fatalf("Failed to send data: %v", err)
		}

		time.Sleep(100 * time.Millisecond) // Give server time to process

		// Check if user is registered
		if _, exists := srv.Clients["persistentuser"]; !exists {
			t.Fatalf("User not registered")
		}

		srv.Shutdown()
		time.Sleep(100 * time.Millisecond) // Give server time to shutdown

		srv = server.New(cfg.Server.Host, cfg.Server.Port, store, cfg)
		go func() {
			if err := srv.Start(); err != nil {
				t.Fatalf("Failed to start server: %v", err)
			}
		}()
		defer srv.Shutdown()

		time.Sleep(100 * time.Millisecond) // Give server time to start

		// Check if user is still registered
		if _, exists := srv.Clients["persistentuser"]; !exists {
			t.Fatalf("User not persisted")
		}
	})

	t.Run("TestWebInterfaceIntegration", func(t *testing.T) {
		webServer, err := server.NewWebServer(srv)
		if err != nil {
			t.Fatalf("Failed to initialize web server: %v", err)
		}
		go func() {
			if err := webServer.Start(":8080"); err != nil {
				t.Fatalf("Failed to start web server: %v", err)
			}
		}()
		defer webServer.Shutdown()

		time.Sleep(100 * time.Millisecond) // Give web server time to start

		resp, err := http.Get("http://localhost:8080/api/data")
		if err != nil {
			t.Fatalf("Failed to get data from web interface: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Unexpected status code: %v", resp.StatusCode)
		}
	})

	t.Run("TestWithRealIRCClients", func(t *testing.T) {
		// This test requires a real IRC client to connect to the server
		// and perform basic operations like NICK, USER, JOIN, PRIVMSG, etc.
		// This can be done manually or using an automated IRC client library.
	})
}
