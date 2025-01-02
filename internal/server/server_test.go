package server

import (
	"context"
	"strings"
	"testing"

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
