package server

import (
	"bufio"
	"log"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	"ircserver/internal/config"
)

// Client represents a connected IRC client
type Client struct {
	conn        net.Conn
	nick        string
	username    string
	realname    string
	channels    map[string]bool
	writer      *bufio.Writer
	mu          sync.Mutex
	lastActive  time.Time
	done        chan struct{}
	config      *config.Config
}

// NewClient creates a new IRC client instance
func NewClient(conn net.Conn, cfg *config.Config) *Client {
	client := &Client{
		conn:       conn,
		channels:   make(map[string]bool),
		writer:     bufio.NewWriter(conn),
		lastActive: time.Now(),
		done:       make(chan struct{}),
		config:     cfg,
	}
	
	// Start idle timeout monitor
	go client.monitorIdle()
	addr := conn.RemoteAddr()
	addrStr := "unknown"
	if addr != nil {
		addrStr = addr.String()
	}
	log.Printf("INFO: New client connection from %s", addrStr)
	return client
}

// Send sends a message to the client
func (c *Client) Send(message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	_, err := c.writer.WriteString(message + "\r\n")
	if err != nil {
		log.Printf("ERROR: Failed to send message to client %s: %v", c.String(), err)
		return err
	}
	log.Printf("DEBUG: Sent to client %s: %s", c.String(), message)
	return c.writer.Flush()
}

func (c *Client) monitorIdle() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			idle := time.Since(c.lastActive)
			c.mu.Unlock()

			if idle > c.config.IRC.IdleTimeout {
				log.Printf("INFO: Client %s timed out after %v of inactivity", c.String(), idle)
				c.conn.Close()
				return
			}
		}
	}
}

// UpdateActivity updates the last active timestamp
func (c *Client) UpdateActivity() {
	c.mu.Lock()
	c.lastActive = time.Now()
	c.mu.Unlock()
}

// Close cleanly shuts down the client
func (c *Client) Close() {
	close(c.done)
	c.conn.Close()
}

// handleConnection processes the client connection
func (c *Client) handleConnection() error {
	reader := bufio.NewReader(c.conn)
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		
		c.UpdateActivity()
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "NICK ") {
			nick := strings.TrimPrefix(line, "NICK ")
			if nick == "" {
				return &IRCError{Code: "431", Message: "No nickname given"}
			}
			if !isValidNick(nick) {
				return &IRCError{Code: "432", Message: "Erroneous nickname"}
			}
			c.nick = nick
		}
		
		if strings.HasPrefix(line, "USER ") {
			parts := strings.Split(line, " ")
			if len(parts) < 5 {
				return &IRCError{Code: "461", Message: "Not enough parameters"}
			}
			c.username = parts[1]
			c.realname = strings.TrimPrefix(strings.Join(parts[4:], " "), ":")
			return nil
		}
	}
}

func isValidNick(nick string) bool {
	for _, r := range nick {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// String returns a string representation of the client
func (c *Client) String() string {
	if c.nick == "" {
		return "unknown"
	}
	return c.nick
}
