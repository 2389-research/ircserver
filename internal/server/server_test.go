package server

import (
	"context"
	"ircserver/internal/config"
	"ircserver/internal/persistence"
	"strings"
	"testing"
)

// mockStore implements persistence.Store interface for testing
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
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	// Create test clients
	client1 := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client1.nick = "user1"
	client2 := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client2.nick = "user2"

	// Test joining a channel
	srv.handleJoin(client1, "#test")

	if len(srv.channels) != 1 {
		t.Error("Expected 1 channel, got", len(srv.channels))
	}

	channel := srv.channels["#test"]
	if !channel.HasClient("user1") {
		t.Error("Expected user1 to be in channel")
	}

	// Test second user joining
	srv.handleJoin(client2, "#test")
	if !channel.HasClient("user2") {
		t.Error("Expected user2 to be in channel")
	}

	// Test parting
	srv.handlePart(client1, "#test")
	if channel.HasClient("user1") {
		t.Error("Expected user1 to have left channel")
	}

	// Test channel cleanup
	srv.handlePart(client2, "#test")
	if len(srv.channels) != 0 {
		t.Error("Expected empty channel to be removed")
	}
}
