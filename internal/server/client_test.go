package server

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"ircserver/internal/config"
)

// mockConn implements net.Conn interface for testing.
type mockConn struct {
	readData  *strings.Reader
	writeData strings.Builder
}

func (m *mockConn) Read(b []byte) (n int, err error)  { return m.readData.Read(b) }
func (m *mockConn) Write(b []byte) (n int, err error) { return m.writeData.Write(b) }
func (m *mockConn) Close() error                      { return nil }
func (m *mockConn) LocalAddr() net.Addr               { return nil }

type mockAddr struct{}

func (a *mockAddr) Network() string { return "tcp" }
func (a *mockAddr) String() string  { return "test:1234" }

func (m *mockConn) RemoteAddr() net.Addr               { return &mockAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestNewClient(t *testing.T) {
	cfg := config.DefaultConfig()
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn, cfg)

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
	cfg := config.DefaultConfig()
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn, cfg)

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

func TestClientConnection(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		nick              string
		wantErr           bool
		expectedResponses []string
	}{
		{
			name:    "nickname too long",
			input:   "NICK toolongnick123\r\nUSER test 0 * :Test User\r\n",
			wantErr: false,
			expectedResponses: []string{
				"432 * toolongnick123 :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _\r\n",
			},
		},
		{
			name:    "nickname with invalid characters",
			input:   "NICK bad@nick\r\nUSER test 0 * :Test User\r\n",
			wantErr: false,
			expectedResponses: []string{
				"432 * bad@nick :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _\r\n",
			},
		},
		{
			name:    "nickname starting with number",
			input:   "NICK 1nick\r\nUSER test 0 * :Test User\r\n",
			wantErr: false,
			expectedResponses: []string{
				"432 * 1nick :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _\r\n",
			},
		},
		{
			name:    "valid nickname with underscore",
			input:   "NICK nick_123\r\nUSER test 0 * :Test User\r\n",
			nick:    "nick_123",
			wantErr: false,
			expectedResponses: []string{
				":* NICK nick_123\r\n",
			},
		},
		{
			name:    "valid connection with nick",
			input:   "NICK validnick\r\nUSER test 0 * :Test User\r\n",
			nick:    "validnick",
			wantErr: false,
			expectedResponses: []string{
				":* NICK validnick\r\n",
			},
		},
		{
			name:    "invalid nick character",
			input:   "NICK invalid@nick\r\nUSER test 0 * :Test User\r\n",
			wantErr: false,
			expectedResponses: []string{
				"432 * invalid@nick :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _\r\n",
			},
		},
		{
			name:    "missing USER command",
			input:   "NICK validnick\r\n",
			wantErr: true,
		},
		{
			name:    "empty nick",
			input:   "NICK \r\nUSER test 0 * :Test User\r\n",
			wantErr: false, // Changed to false since we handle this gracefully now
			expectedResponses: []string{
				"431 * :No nickname given\r\n",
			},
		},
		{
			name:    "EOF handling",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			conn := &mockConn{
				readData: strings.NewReader(tt.input),
			}
			client := NewClient(conn, cfg)

			// Start client processing in background
			errCh := make(chan error, 1)
			go func() {
				errCh <- client.handleConnection()
			}()

			// Wait for processing or timeout
			select {
			case err := <-errCh:
				if (err != nil) != tt.wantErr {
					t.Errorf("handleConnection() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("handleConnection() timeout")
			}

			if !tt.wantErr {
				if client.nick != tt.nick {
					t.Errorf("Expected nickname %q, got %q", tt.nick, client.nick)
				}

				// Verify expected responses
				output := conn.writeData.String()
				for _, expected := range tt.expectedResponses {
					if !strings.Contains(output, expected) {
						t.Errorf("Expected response %q not found in output: %q", expected, output)
					}
				}
			}
		})
	}
}

func TestConcurrentConnections(t *testing.T) {
	cfg := config.DefaultConfig()
	numClients := 10
	clients := make([]*Client, numClients)
	errCh := make(chan error, numClients)

	// Create and start multiple clients
	for i := 0; i < numClients; i++ {
		input := fmt.Sprintf("NICK user%d\r\nUSER test%d 0 * :Test User %d\r\n", i, i, i)
		conn := &mockConn{readData: strings.NewReader(input)}
		clients[i] = NewClient(conn, cfg)

		go func(client *Client) {
			errCh <- client.handleConnection()
		}(clients[i])
	}

	// Wait for all clients to process or timeout
	timeout := time.After(500 * time.Millisecond)
	for i := 0; i < numClients; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("Client %d connection failed: %v", i, err)
			}
		case <-timeout:
			t.Fatal("Concurrent connections test timed out")
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	cfg := config.DefaultConfig()
	conn := &mockConn{
		readData: strings.NewReader("TEST command\r\n"),
	}
	client := NewClient(conn, cfg)

	err := client.handleConnection()
	if err == nil {
		t.Error("Expected EOF error, got nil")
	}

	expected := "421 unknown TEST :Unknown command\r\n"
	if got := conn.writeData.String(); got != expected {
		t.Errorf("Expected response %q, got %q", expected, got)
	}
}

func TestClientString(t *testing.T) {
	cfg := config.DefaultConfig()
	conn := &mockConn{readData: strings.NewReader("")}
	client := NewClient(conn, cfg)

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
