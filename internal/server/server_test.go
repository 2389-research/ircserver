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

// testClient wraps Client for testing
type testClient struct {
	*Client
	messages []string
}

func (t *testClient) Send(msg string) error {
	// First capture the message
	t.messages = append(t.messages, msg)
	// Then let the underlying client handle it
	if t.Client != nil {
		return t.Client.Send(msg)
	}
	return nil
}

// Ensure testClient implements necessary methods
var _ interface{ Send(string) error } = (*testClient)(nil)

func newTestClient(nick string, cfg *config.Config) *testClient {
	mockConn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(mockConn, cfg)
	client.nick = nick
	
	tc := &testClient{
		Client:   client,
		messages: make([]string, 0),
	}
	return tc
}

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
func setupTestServer() *Server {
	cfg := config.DefaultConfig()
	store := &mockStore{}
	return New("localhost", "0", store, cfg)
}

func TestPrivMsgToUser(t *testing.T) {
	s := setupTestServer()
	cfg := config.DefaultConfig()
	
	// Setup sender and recipient
	sender := newTestClient("alice", cfg)
	recipient := newTestClient("bob", cfg)
	
	s.clients[sender.nick] = sender.Client
	s.clients[recipient.nick] = recipient.Client
	
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "basic message",
			message:  "PRIVMSG bob :Hello there!",
			expected: ":alice PRIVMSG bob :Hello there!",
		},
		{
			name:     "multi-word message",
			message:  "PRIVMSG bob :Hello there, how are you?",
			expected: ":alice PRIVMSG bob :Hello there, how are you?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipient.messages = nil // Clear previous messages
			
			err := s.handleMessage(context.Background(), sender.Client, tt.message)
			if err != nil {
				t.Errorf("handleMessage() error = %v", err)
				return
			}
			
			if len(recipient.messages) != 1 {
				t.Errorf("Expected 1 message, got %d", len(recipient.messages))
				return
			}
			
			if recipient.messages[0] != tt.expected {
				t.Errorf("Expected message '%s', got '%s'", tt.expected, recipient.messages[0])
			}
		})
	}
}

func TestPrivMsgToChannel(t *testing.T) {
	s := setupTestServer()
	
	// Setup clients and channel
	cfg := config.DefaultConfig()
	sender := newTestClient("alice", cfg)
	recipient1 := newTestClient("bob", cfg)
	recipient2 := newTestClient("charlie", cfg)
	
	channelName := "#test"
	// Register clients with server
	s.clients[sender.nick] = sender.Client
	s.clients[recipient1.nick] = recipient1.Client
	s.clients[recipient2.nick] = recipient2.Client

	// Create channel with testClient wrappers
	s.channels[channelName] = &Channel{
		Name: channelName,
		Clients: map[string]*Client{
			sender.nick:     sender.Client,
			recipient1.nick: recipient1.Client,
			recipient2.nick: recipient2.Client,
		},
		Created: time.Now(),
	}

	// Store testClient wrappers for message checking
	testClients := map[string]*testClient{
		sender.nick:     sender,
		recipient1.nick: recipient1,
		recipient2.nick: recipient2,
	}
	
	// Test sending channel message
	s.handleMessage(context.Background(), sender.Client, "PRIVMSG #test :Hello channel!")
	
	expected := ":alice PRIVMSG #test :Hello channel!"
	
	// Check that both recipients got the message using the testClient wrappers
	for _, nick := range []string{"bob", "charlie"} {
		testClient := testClients[nick]
		if len(testClient.messages) != 1 {
			t.Errorf("Expected 1 message for %s, got %d", nick, len(testClient.messages))
			continue
		}
		if testClient.messages[0] != expected {
			t.Errorf("Expected message '%s' for %s, got '%s'", expected, nick, testClient.messages[0])
		}
	}
}

func TestNoticeHandling(t *testing.T) {
	s := setupTestServer()
	
	cfg := config.DefaultConfig()
	sender := newTestClient("alice", cfg)
	recipient := newTestClient("bob", cfg)
	
	s.clients[sender.nick] = sender.Client
	s.clients[recipient.nick] = recipient.Client
	
	// Test NOTICE message
	s.handleMessage(context.Background(), sender.Client, "NOTICE bob :Server maintenance in 5 minutes")
	
	if len(recipient.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(recipient.messages))
	}
	
	expected := ":alice NOTICE bob :Server maintenance in 5 minutes"
	if recipient.messages[0] != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, recipient.messages[0])
	}
}

func TestMessageSizeLimits(t *testing.T) {
	s := setupTestServer()
	
	cfg := config.DefaultConfig()
	sender := newTestClient("alice", cfg)
	recipient := newTestClient("bob", cfg)
	
	s.clients[sender.nick] = sender.Client
	s.clients[recipient.nick] = recipient.Client
	
	// Create message that exceeds 512 bytes (IRC protocol limit)
	longMessage := "PRIVMSG bob :" + strings.Repeat("x", 500)
	
	// Test sending oversized message
	s.handleMessage(context.Background(), sender.Client, longMessage)
	
	// Verify recipient received nothing due to size limit
	if len(recipient.messages) != 0 {
		t.Errorf("Expected no messages due to size limit, got %d", len(recipient.messages))
	}
}

func TestInvalidMessageFormats(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"Empty message", ""},
		{"Missing target", "PRIVMSG"},
		{"Missing colon", "PRIVMSG bob Hello"},
		{"Invalid command", "INVALID bob :Hello"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupTestServer()
			cfg := config.DefaultConfig()
			sender := newTestClient("alice", cfg)
			recipient := newTestClient("bob", cfg)
			
			s.clients[sender.nick] = sender.Client
			s.clients[recipient.nick] = recipient.Client
			
			// Test invalid message
			s.handleMessage(context.Background(), sender.Client, tt.message)
			
			// Verify no messages were sent
			if len(recipient.messages) != 0 {
				t.Errorf("Expected no messages for invalid format, got %d", len(recipient.messages))
			}
		})
	}
}

func BenchmarkChannelBroadcast(b *testing.B) {
	s := setupTestServer()
	cfg := config.DefaultConfig()
	sender := newTestClient("alice", cfg)
	
	// Create a channel with 100 users
	channelName := "#test"
	channel := &Channel{
		Name:    channelName,
		Clients: make(map[string]*Client),
		Created: time.Now(),
	}
	
	for i := 0; i < 100; i++ {
		client := newTestClient(fmt.Sprintf("user%d", i), cfg)
		channel.Clients[client.nick] = client.Client
	}
	
	s.channels[channelName] = channel
	
	message := "PRIVMSG #test :Hello everyone!"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.handleMessage(context.Background(), sender.Client, message)
	}
}
