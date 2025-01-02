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

func TestJoinResponseMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn, cfg)
	client.nick = "user1"

	// Test joining channel with no topic
	srv.handleJoin(client, "#test")

	output := conn.writeData.String()
	if !strings.Contains(output, "JOIN #test") {
		t.Error("Expected JOIN confirmation message")
	}
	if !strings.Contains(output, "331") || !strings.Contains(output, "No topic is set") {
		t.Error("Expected no topic message")
	}
	if !strings.Contains(output, "353") || !strings.Contains(output, "user1") {
		t.Error("Expected names list")
	}
	if !strings.Contains(output, "366") {
		t.Error("Expected end of names list")
	}

	// Reset connection buffer
	conn.writeData.Reset()

	// Test joining channel with topic
	channel := srv.channels["#test"]
	channel.SetTopic("Test Topic")

	client2 := NewClient(&mockConn{readData: strings.NewReader("")}, cfg)
	client2.nick = "user2"
	srv.handleJoin(client2, "#test")

	output = client2.conn.(*mockConn).writeData.String()
	if !strings.Contains(output, "332") || !strings.Contains(output, "Test Topic") {
		t.Error("Expected topic message")
	}
}

func TestChannelMultiUserOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}

	// Create fresh server and clients for each subtest
	var srv *Server
	var clients []*Client
	var conns []*mockConn

	setup := func() {
		srv = New("localhost", "0", store, cfg)
		clients = make([]*Client, 3)
		conns = make([]*mockConn, 3)
		for i := range clients {
			conns[i] = &mockConn{readData: strings.NewReader("")}
			clients[i] = NewClient(conns[i], cfg)
			clients[i].nick = fmt.Sprintf("user%d", i+1)
		}
	}

	setup()

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
	srv.handlePrivMsg(clients[0], "#test :Hello channel!")
	for i := 1; i < len(clients); i++ {
		if !strings.Contains(conns[i].writeData.String(), "Hello channel!") {
			t.Errorf("Expected client %d to receive broadcast message", i+1)
		}
	}

	// Test PART error cases
	t.Run("part non-existent channel", func(t *testing.T) {
		setup() // Reset state
		srv.handlePart(clients[0], "#nonexistent")
		if !strings.Contains(conns[0].writeData.String(), "403") {
			t.Error("Expected error response for non-existent channel")
		}
	})

}
func TestMessageDelivery(t *testing.T) {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	srv := New("localhost", "0", store, cfg)

	tests := []struct {
		name       string
		sender     string
		target     string
		message    string
		msgType    string
		wantError  bool
		errorCode  string
	}{
		{"valid private message", "user1", "user2", "Hello!", "PRIVMSG", false, ""},
		{"valid channel message", "user1", "#test", "Hello channel!", "PRIVMSG", false, ""},
		{"empty target", "user1", "", "Hello!", "PRIVMSG", true, "411"},
		{"empty message", "user1", "user2", "", "PRIVMSG", true, "412"},
		{"multiple targets", "user1", "user2,user3", "Hello all!", "PRIVMSG", false, ""},
		{"notice to channel", "user1", "#test", "Notice!", "NOTICE", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup sender
			senderConn := &mockConn{readData: strings.NewReader("")}
			sender := NewClient(senderConn, cfg)
			sender.nick = tt.sender
			srv.clients[tt.sender] = sender

			// Setup target(s)
			if strings.HasPrefix(tt.target, "#") {
				srv.channels[tt.target] = NewChannel(tt.target)
			} else {
				for _, target := range strings.Split(tt.target, ",") {
					if target != "" {
						targetConn := &mockConn{readData: strings.NewReader("")}
						targetClient := NewClient(targetConn, cfg)
						targetClient.nick = target
						srv.clients[target] = targetClient
					}
				}
			}

			// Test message delivery
			if tt.msgType == "PRIVMSG" {
				srv.handlePrivMsg(sender, fmt.Sprintf("%s :%s", tt.target, tt.message))
			} else {
				srv.handleNotice(sender, fmt.Sprintf("%s :%s", tt.target, tt.message))
			}

			// Check for expected errors
			output := senderConn.writeData.String()
			if tt.wantError {
				if !strings.Contains(output, tt.errorCode) {
					t.Errorf("Expected error code %s, got output: %s", tt.errorCode, output)
				}
			} else {
				// Verify message delivery
				if strings.HasPrefix(tt.target, "#") {
					channel := srv.channels[tt.target]
					for _, client := range channel.Clients {
						if client.nick != tt.sender {
							clientConn := client.conn.(*mockConn)
							if !strings.Contains(clientConn.writeData.String(), tt.message) {
								t.Errorf("Expected channel message delivery to %s", client.nick)
							}
						}
					}
				} else {
					for _, target := range strings.Split(tt.target, ",") {
						targetClient := srv.clients[target]
						targetConn := targetClient.conn.(*mockConn)
						if !strings.Contains(targetConn.writeData.String(), tt.message) {
							t.Errorf("Expected message delivery to %s", target)
						}
					}
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

	// Wait for rate limit window
	time.Sleep(time.Second * 3)

	// Count delivered messages
	output := targetConn.writeData.String()
	count := strings.Count(output, "Message")
	if count > 10 {
		t.Errorf("Expected rate limiting to allow max 10 messages, got %d", count)
	}
}
