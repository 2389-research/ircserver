package server

import (
	"context"
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

func TestMessageRateLimiting(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn, cfg)
	client.nick = "user1"
	srv.clients["user1"] = client

	targetConn := &mockConn{readData: strings.NewReader("")}
	target := NewClient(targetConn, cfg)
	target.nick = "user2"
	srv.clients["user2"] = target

	// Send messages rapidly
	for i := 0; i < 15; i++ {
		srv.handlePrivMsg(client, fmt.Sprintf("user2 :Message %d", i))
	}

	// Wait for initial batch to be processed
	time.Sleep(time.Second * 1)

	// Check first window
	output := targetConn.writeData.String()
	count := strings.Count(output, "Message")
	if count > 10 {
		t.Errorf("First window: Expected max 10 messages, got %d", count)
	}

	targetConn.writeData.Reset()

	// Wait for second window
	time.Sleep(time.Second * 2)

	// Check second window
	output = targetConn.writeData.String()
	count = strings.Count(output, "Message")
	if count > 5 { // Remaining messages
		t.Errorf("Second window: Expected remaining messages (<=5), got %d", count)
	}
}
