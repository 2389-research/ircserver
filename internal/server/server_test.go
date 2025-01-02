package server

import (
	"context"
	"fmt"
	"math/rand"
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
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// WaitGroup for all operations
	var wg sync.WaitGroup

	// Launch concurrent join operations
	for _, client := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			// Each client joins random channels
			for _, channel := range channels[:rand.Intn(len(channels))+1] {
				srv.handleJoin(c, channel)
			}
		}(client)
	}

	// Wait for joins to complete
	wg.Wait()
	t.Log("Join operations completed")

	// Launch message operations
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

	// Wait for messages to complete
	wg.Wait()
	t.Log("Message operations completed")

	// Now launch part operations - ensure all clients leave all channels
	for _, channel := range channels {
		srv.mu.RLock()
		ch, exists := srv.channels[channel]
		srv.mu.RUnlock()

		if !exists {
			continue
		}

		// Get members under read lock
		ch.mu.RLock()
		members := make([]*Client, 0, len(ch.Members))
		for _, member := range ch.Members {
			members = append(members, member.Client)
		}
		ch.mu.RUnlock()

		// Now have each member part
		for _, client := range members {
			wg.Add(1)
			go func(c *Client, channelName string) {
				defer wg.Done()
				srv.handlePart(c, channelName)
			}(client, channel)
		}
	}

	// Wait for parts to complete
	wg.Wait()
	t.Log("Part operations completed")

	// Give a small window for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify final state
	srv.mu.RLock()
	remainingChannels := len(srv.channels)
	var channelNames []string
	for name := range srv.channels {
		channelNames = append(channelNames, name)
	}
	srv.mu.RUnlock()

	if remainingChannels != 0 {
		t.Errorf("Expected all channels to be removed, got %d. Remaining channels: %v",
			remainingChannels, channelNames)
	}

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

// TestConcurrentChannelJoins tests concurrent channel joins to validate safe operation
func TestConcurrentChannelJoins(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	const numClients = 100
	const numChannels = 10

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
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			for j := 0; j < numChannels; j++ {
				channelName := fmt.Sprintf("#channel%d", j)
				srv.handleJoin(client, channelName)
			}
		}(clients[i])
	}

	wg.Wait()

	// Verify all clients are in all channels
	for i := 0; i < numChannels; i++ {
		channelName := fmt.Sprintf("#channel%d", i)
		srv.mu.RLock()
		channel, exists := srv.channels[channelName]
		srv.mu.RUnlock()

		if !exists {
			t.Errorf("Expected channel %s to exist", channelName)
			continue
		}

		channel.mu.RLock()
		if len(channel.Members) != numClients {
			t.Errorf("Expected %d members in channel %s, got %d", numClients, channelName, len(channel.Members))
		}
		channel.mu.RUnlock()
	}
}

// TestConcurrentChannelLeaves tests concurrent channel leaves to validate safe operation
func TestConcurrentChannelLeaves(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	const numClients = 100
	const numChannels = 10

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

	// Join all clients to all channels
	for i := 0; i < numClients; i++ {
		for j := 0; j < numChannels; j++ {
			channelName := fmt.Sprintf("#channel%d", j)
			srv.handleJoin(clients[i], channelName)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			for j := 0; j < numChannels; j++ {
				channelName := fmt.Sprintf("#channel%d", j)
				srv.handlePart(client, channelName)
			}
		}(clients[i])
	}

	wg.Wait()

	// Verify all channels are empty
	for i := 0; i < numChannels; i++ {
		channelName := fmt.Sprintf("#channel%d", i)
		srv.mu.RLock()
		channel, exists := srv.channels[channelName]
		srv.mu.RUnlock()

		if exists {
			channel.mu.RLock()
			if len(channel.Members) != 0 {
				t.Errorf("Expected 0 members in channel %s, got %d", channelName, len(channel.Members))
			}
			channel.mu.RUnlock()
		}
	}
}

// TestConcurrentMessageDeliveries tests concurrent message deliveries to validate safe operation
func TestConcurrentMessageDeliveries(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	const numClients = 100
	const numChannels = 10
	const numMessages = 1000

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

	// Join all clients to all channels
	for i := 0; i < numClients; i++ {
		for j := 0; j < numChannels; j++ {
			channelName := fmt.Sprintf("#channel%d", j)
			srv.handleJoin(clients[i], channelName)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(msgNum int) {
			defer wg.Done()
			client := clients[msgNum%numClients]
			channel := fmt.Sprintf("#channel%d", msgNum%numChannels)
			msg := fmt.Sprintf("Message %d", msgNum)
			srv.deliverMessage(client, channel, "PRIVMSG", msg)
		}(i)
	}

	wg.Wait()

	// Verify message delivery
	for i := 0; i < numClients; i++ {
		client := clients[i]
		for j := 0; j < numChannels; j++ {
			channelName := fmt.Sprintf("#channel%d", j)
			if !strings.Contains(client.conn.(*mockConn).writeData.String(), channelName) {
				t.Errorf("Expected messages to be delivered to channel %s for client %s", channelName, client.nick)
			}
		}
	}
}
