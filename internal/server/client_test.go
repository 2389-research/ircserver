package server

import (
	"net"
	"strings"
	"testing"
	"time"
)

// mockConn implements net.Conn interface for testing
type mockConn struct {
	readData  *strings.Reader
	writeData strings.Builder
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return m.readData.Read(b) }
func (m *mockConn) Write(b []byte) (n int, err error)  { return m.writeData.Write(b) }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestNewClient(t *testing.T) {
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn)

	if client.conn != conn {
		t.Error("NewClient did not set connection correctly")
	}
	if client.channels == nil {
		t.Error("NewClient did not initialize channels map")
	}
	if client.writer == nil {
		t.Error("NewClient did not initialize writer")
	}
}

func TestClientSend(t *testing.T) {
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn)

	message := "TEST MESSAGE"
	err := client.Send(message)
	if err != nil {
		t.Errorf("Send returned unexpected error: %v", err)
	}

	expected := message + "\r\n"
	if got := conn.writeData.String(); got != expected {
		t.Errorf("Send wrote %q, want %q", got, expected)
	}
}

func TestClientString(t *testing.T) {
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn)

	// Test with no nickname
	if got := client.String(); got != "unknown" {
		t.Errorf("String() = %q, want %q", got, "unknown")
	}

	// Test with nickname
	client.nick = "testuser"
	if got := client.String(); got != "testuser" {
		t.Errorf("String() = %q, want %q", got, "testuser")
	}
}
