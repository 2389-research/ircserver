package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ircserver/internal/config"
	"ircserver/internal/persistence"
)

// mockStore implements persistence.Store interface for testing.
type mockStore struct{}

var _ persistence.Store = (*mockStore)(nil) // Compile-time interface check

func (m *mockStore) Close() error { return nil }

func (m *mockStore) LogMessage(ctx context.Context, sender, recipient, msgType, content string) error {
	return nil
}

func (m *mockStore) UpdateUser(ctx context.Context, nickname, username, realname, ipAddr string) error {
	return nil
}
func (m *mockStore) UpdateChannel(ctx context.Context, name, topic string) error { return nil }

func TestChannelOperations(t *testing.T) {
	tests := []struct {
		name    string
		channel string
		want    bool
	}{
		{"valid channel", "#test", true},
		{"valid ampersand channel", "&test", true},
		{"invalid no prefix", "test", false},
		{"invalid empty", "", false},
		{"invalid spaces", "#test channel", false},
		{"invalid comma", "#test,channel", false},
		{"invalid control-G", "#test\aname", false},
		{"valid with numbers", "#test123", true},
		{"valid with special chars", "#test-._[]{}\\`", true},
		{"invalid too long", "#" + strings.Repeat("a", 200), false},
	}

	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset server state
			srv = New("localhost", "0", store, cfg)

			client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
			client.nick = "user1"

			srv.handleJoin(client, tt.channel)

			if tt.want {
				if len(srv.channels) != 1 {
					t.Errorf("Expected 1 channel, got %d", len(srv.channels))
				}
				channel := srv.channels[tt.channel]
				if member := channel.Members[client.nick]; member == nil {
					t.Error("Expected user1 to be in channel")
				}
			} else {
				if len(srv.channels) != 0 {
					t.Error("Expected no channels for invalid channel name")
				}
			}
		})
	}
}

// TestConcurrentOperations verifies thread-safety of server operations
func TestConcurrentOperations(t *testing.T) {
	// Add timeout to prevent test from hanging
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	// Create test channels and clients
	channels := []string{"#test1", "#test2", "#test3"}
	numClients := 10
	clients := make([]*Client, numClients)
	
	// Initialize clients under server lock
	srv.mu.Lock()
	for i := 0; i < numClients; i++ {
		clients[i] = NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
		clients[i].nick = fmt.Sprintf("user%d", i)
		srv.clients[clients[i].nick] = clients[i]
	}
	srv.mu.Unlock()

	var wg sync.WaitGroup
	errChan := make(chan error, numClients*len(channels)*3) // Buffer for all possible errors

	// Concurrent joins
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.handleJoin(c, ch)
			}(client, channel)
		}
	}
	wg.Wait()

	// Verify channel membership
	srv.mu.RLock()
	for _, channel := range channels {
		if ch, exists := srv.channels[channel]; exists {
			if len(ch.Members) != numClients {
				t.Errorf("Expected %d members in channel %s, got %d", numClients, channel, len(ch.Members))
			}
		} else {
			t.Errorf("Channel %s not created", channel)
		}
	}
	srv.mu.RUnlock()

	// Concurrent messages
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.deliverMessage(c, ch, "PRIVMSG", "test message")
			}(client, channel)
		}
	}
	wg.Wait()

	// Concurrent parts - do this in phases to avoid deadlocks
	for _, channel := range channels {
		wg.Add(len(clients))
		for _, client := range clients {
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.handlePart(c, ch)
			}(client, channel)
		}
		wg.Wait() // Wait for each channel to be fully cleared before moving to next
	}

	// Final verification with timeout
	done := make(chan bool)
	go func() {
		srv.mu.RLock()
		remainingChannels := len(srv.channels)
		if remainingChannels != 0 {
			t.Logf("WARNING: %d channels still exist:", remainingChannels)
			for name, ch := range srv.channels {
				t.Logf("Channel %s has %d members", name, len(ch.Members))
				for nick := range ch.Members {
					t.Logf("  - Member: %s", nick)
				}
			}
			t.Errorf("Expected all channels to be removed, got %d remaining", remainingChannels)
		}
		srv.mu.RUnlock()
		done <- true
	}()

	timeout := time.After(5 * time.Second)

	select {
	case <-timeout:
		t.Fatal("Test timed out after 5 seconds")
	case <-done:
		// Test completed successfully
	}
}

func TestMessageEchoPrevention(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client1 := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client1.nick = "user1"
	client2 := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client2.nick = "user2"

	srv.clients[client1.nick] = client1
	srv.clients[client2.nick] = client2

	channelName := "#test"
	srv.channels[channelName] = NewChannel(channelName)
	srv.channels[channelName].AddClient(client1, UserModeNormal)
	srv.channels[channelName].AddClient(client2, UserModeNormal)

	message := "Hello, world!"
	srv.deliverMessage(client1, channelName, "PRIVMSG", message)

	if strings.Contains(client1.conn.(*mockConn).writeData.String(), message) {
		t.Errorf("Expected message not to be echoed back to the sender")
	}

	if !strings.Contains(client2.conn.(*mockConn).writeData.String(), message) {
		t.Errorf("Expected message to be delivered to other users")
	}
}
