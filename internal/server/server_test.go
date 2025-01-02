package server

import (
	"net"
	"testing"
	"time"
)

func TestServerAcceptsConnections(t *testing.T) {
	srv := New("localhost", "6668")

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
