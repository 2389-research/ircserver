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
	// Add timeout for the whole test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const (
		numClients  = 50
		numChannels = 10
		numMessages = 100
	)

	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	// Create test channels
	channels := make([]string, numChannels)
	for i := 0; i < numChannels; i++ {
		channels[i] = fmt.Sprintf("#test%d", i)
	}

	// Create and register clients
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		mockConn := &mockConn{readData: strings.NewReader("")}
		clients[i] = NewClient(mockConn, cfg)
		clients[i].nick = fmt.Sprintf("user%d", i)
		clients[i].username = fmt.Sprintf("user%d", i)
		clients[i].realname = fmt.Sprintf("User %d", i)

		srv.mu.Lock()
		srv.clients[clients[i].nick] = clients[i]
		srv.mu.Unlock()
	}

	var wg sync.WaitGroup

	// Launch concurrent join operations
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.handleJoin(c, ch)
			}(client, channel)
		}
	}

	// Launch concurrent message operations
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(msgNum int) {
			defer wg.Done()
			client := clients[msgNum%numClients]
			channel := channels[msgNum%numChannels]
			msg := fmt.Sprintf("Message %d", msgNum)
			srv.deliverMessage(client, channel, "PRIVMSG", msg)
		}(i)
	}

	// Launch concurrent part operations
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.handlePart(c, ch)
			}(client, channel)
		}
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	case <-done:
		// Success
	}

	// Verify final state
	state := srv.getState()
	if len(state.channels) != 0 {
		t.Errorf("Expected all channels to be removed, got %d", len(state.channels))
	}

	// Clean up
	for _, client := range clients {
		client.Close()
	}
}

	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	// Create test channels and clients
	channels := []string{"#test1", "#test2"}
	numClients := 5
	clients := make([]*Client, numClients)

	// Initialize clients under server lock
	srv.mu.Lock()
	for i := 0; i < numClients; i++ {
		mockConn := &mockConn{readData: strings.NewReader("")}
		clients[i] = NewClient(mockConn, cfg)
		clients[i].nick = fmt.Sprintf("user%d", i)
		srv.clients[clients[i].nick] = clients[i]
	}
	srv.mu.Unlock()

	var wg sync.WaitGroup

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

	// Wait for all joins with timeout
	joinDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(joinDone)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out during joins")
		return
	case <-joinDone:
		// Joins completed successfully
	}

	// Verify channel membership under read lock
	srv.mu.RLock()
	for _, channel := range channels {
		if ch, exists := srv.channels[channel]; exists {
			if len(ch.Members) != numClients {
				t.Errorf("Expected %d members in channel %s, got %d",
					numClients, channel, len(ch.Members))
			}
		} else {
			t.Errorf("Channel %s not created", channel)
		}
	}
	srv.mu.RUnlock()

	// Concurrent parts - all at once
	wg = sync.WaitGroup{} // Reset WaitGroup for parts
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				srv.handlePart(c, ch)
			}(client, channel)
		}
	}

	// Wait for all parts with timeout
	partDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(partDone)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out during parts")
		return
	case <-partDone:
		// Parts completed successfully
	}

	// Give a small window for cleanup operations
	time.Sleep(50 * time.Millisecond)

	// Final verification under read lock
	srv.mu.RLock()
	remainingChannels := len(srv.channels)
	if remainingChannels != 0 {
		for name, ch := range srv.channels {
			t.Logf("Channel %s has %d members", name, len(ch.Members))
			for nick := range ch.Members {
				t.Logf("  - Member still present: %s", nick)
			}
		}
		t.Errorf("Expected all channels to be removed, got %d remaining", remainingChannels)
	}
	srv.mu.RUnlock()

	// Clean up
	for _, client := range clients {
		client.Close()
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
