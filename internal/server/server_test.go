package server

import (
	"ircserver/internal/persistence"
	"net"
	"strings"
	"testing"
	"time"
)

// mockStore implements persistence.Store interface for testing
type mockStore struct{} 

var _ persistence.Store = (*mockStore)(nil) // Compile-time interface check

func (m *mockStore) Close() error                                                { return nil }
func (m *mockStore) LogMessage(sender, recipient, msgType, content string) error { return nil }
func (m *mockStore) UpdateUser(nickname, username, realname, ipAddr string) error { return nil }
func (m *mockStore) UpdateChannel(name, topic string) error                      { return nil }

func TestChannelOperations(t *testing.T) {
	store := &mockStore{}
	srv := New("localhost", "0", store)
	
	// Create test clients
	client1 := NewClient(&mockConn{readData: strings.NewReader("")})
	client1.nick = "user1"
	client2 := NewClient(&mockConn{readData: strings.NewReader("")})
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

func TestServerAcceptsConnections(t *testing.T) {
	store := &mockStore{}
	srv := New("localhost", "6668", store)

	// Start server in a goroutine
	go func() {
		err := srv.Start()
		if err != nil {
			t.Errorf("Server failed to start: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect
	conn, err := net.Dial("tcp", "localhost:6668")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// If we got here, connection was successful
	// Send a test message
	testMsg := []byte("TEST\r\n")
	_, err = conn.Write(testMsg)
	if err != nil {
		t.Fatalf("Failed to write to connection: %v", err)
	}
}
