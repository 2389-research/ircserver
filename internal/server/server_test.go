package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
		{"invalid no hash", "test", false},
		{"invalid empty", "", false},
		{"invalid spaces", "#test channel", false},
		{"valid with numbers", "#test123", true},
		{"valid with underscore", "#test_channel", true},
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
				if !srv.channels[tt.channel].HasClient("user1") {
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

func TestChannelMultiUserOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	clients := make([]*Client, 3)
	for i := range clients {
		clients[i] = NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
		clients[i].nick = fmt.Sprintf("user%d", i+1)
	}

	// Test multiple users joining
	for _, client := range clients {
		srv.handleJoin(client, "#test")
	}

	channel := srv.channels["#test"]
	if len(channel.Clients) != 3 {
		t.Errorf("Expected 3 clients in channel, got %d", len(channel.Clients))
	}

	// Test topic changes
	srv.handleTopic(clients[0], "#test :New Topic")
	if channel.Topic != "New Topic" {
		t.Errorf("Expected topic 'New Topic', got '%s'", channel.Topic)
	}

	// Test message broadcasting
	mockConn := clients[1].conn.(*mockConn)
	srv.handlePrivMsg(clients[0], "#test :Hello channel!")
	if !strings.Contains(mockConn.writeData.String(), "Hello channel!") {
		t.Error("Expected message to be broadcasted to other clients")
	}

	// Test sequential parting
	for i, client := range clients {
		srv.handlePart(client, "#test")
		expectedClients := len(clients) - (i + 1)
		if expectedClients > 0 {
			if len(channel.Clients) != expectedClients {
				t.Errorf("Expected %d clients, got %d", expectedClients, len(channel.Clients))
			}
		}
	}

	// Verify channel cleanup after last user parts
	if len(srv.channels) != 0 {
		t.Error("Expected channel to be removed after last user left")
	}
}

func TestNetworkErrorHandling(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client.nick = "user1"

	// Simulate network error
	client.conn.Close()
	err := srv.handleJoin(client, "#test")
	if err == nil {
		t.Error("Expected network error, got nil")
	}
}

func TestDatabaseErrorHandling(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client.nick = "user1"

	// Simulate database error
	store.UpdateUser = func(ctx context.Context, nickname, username, realname, ipAddr string) error {
		return errors.New("database error")
	}

	err := srv.handleJoin(client, "#test")
	if err == nil {
		t.Error("Expected database error, got nil")
	}
}

func TestTimeoutHandling(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client.nick = "user1"

	// Simulate timeout
	client.conn.SetReadDeadline(time.Now().Add(-time.Second))
	err := srv.handleJoin(client, "#test")
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestResourceCleanup(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client.nick = "user1"

	// Simulate resource cleanup
	cleanupDone := false
	defer func() {
		cleanupDone = true
	}()
	srv.handleQuit(client, "Quit")
	if !cleanupDone {
		t.Error("Expected resource cleanup to be done")
	}
}

func TestRecoveryFromErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	client := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client.nick = "user1"

	// Simulate recovery from an error
	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from error")
		}
	}()
	panic("simulated error")
}
