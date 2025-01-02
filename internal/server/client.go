package server

import (
	"bufio"
	"fmt"
	"ircserver/internal/config"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Client represents a connected IRC client.
type Client struct {
	conn       net.Conn
	nick       string
	username   string
	realname   string
	channels   map[string]bool
	writer     *bufio.Writer
	mu         sync.Mutex
	lastActive time.Time
	done       chan struct{}
	config     *config.Config
}

// NewClient creates a new IRC client instance.
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
	// Start keepalive monitor
	go client.startKeepalive()

	addr := conn.RemoteAddr()
	addrStr := "unknown"
	if addr != nil {
		addrStr = addr.String()
	}
	log.Printf("INFO: New client connection from %s", addrStr)
	return client
}

// Send sends a message to the client.
func (c *Client) Send(message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	maxRetries := 3
	if c.config.IRC.MaxRetries > 0 {
		maxRetries = c.config.IRC.MaxRetries
	}
	retryDelay := time.Second
	if c.config.IRC.RetryDelay > 0 {
		retryDelay = c.config.IRC.RetryDelay
	}

	for retries := maxRetries; retries > 0; retries-- {
		_, err = c.writer.WriteString(message + "\r\n")
		if err == nil {
			log.Printf("DEBUG: Sent to client %s: %s", c.String(), message)
			return c.writer.Flush()
		}
		log.Printf("ERROR: Failed to send message to client %s: %v", c.String(), err)
		time.Sleep(retryDelay)
	}
	return err
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

// startKeepalive sends keepalive messages at regular intervals.
func (c *Client) startKeepalive() {
	ticker := time.NewTicker(c.config.IRC.KeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.Send("PING :keepalive"); err != nil {
				log.Printf("ERROR: Failed to send keepalive message: %v", err)
				c.conn.Close()
				return
			}
		}
	}
}

// UpdateActivity updates the last active timestamp.
func (c *Client) UpdateActivity() {
	c.mu.Lock()
	c.lastActive = time.Now()
	c.mu.Unlock()
}

// Close cleanly shuts down the client.
func (c *Client) Close() {
	close(c.done)
	c.conn.Close()
}

// handleConnection processes the client connection.
func (c *Client) handleConnection() error {
	reader := bufio.NewReader(c.conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("ERROR: Read error for client %s: %v", c.String(), err)
			return err
		}

		c.UpdateActivity()
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		if cmd == "NICK" {
			var nick string
			if len(parts) < 2 {
				nick = ""
			} else {
				nick = strings.TrimSpace(strings.Join(parts[1:], " "))
			}

			if nick == "" {
				if err := c.Send("431 * :No nickname given"); err != nil {
					return err
				}
				continue
			}
			if !isValidNick(nick) {
				if err := c.Send("432 * :Erroneous nickname"); err != nil {
					return err
				}
				continue
			}
			oldNick := c.nick
			c.nick = nick
			if oldNick == "" {
				oldNick = "*"
			}
			if err := c.Send(fmt.Sprintf(":%s NICK %s", oldNick, nick)); err != nil {
				return err
			}
			continue
		}

		if strings.HasPrefix(line, "USER ") {
			parts := strings.Split(line, " ")
			if len(parts) < 5 {
				return &IRCError{Code: "461", Message: "Not enough parameters"}
			}
			c.username = parts[1]
			c.realname = strings.TrimPrefix(strings.join(parts[4:], " "), ":")
			return nil
		}

		// Unknown command
		log.Printf("WARN: Unknown command from %s: %s", c.String(), cmd)
		if err := c.Send(fmt.Sprintf("421 %s %s :Unknown command", c.String(), cmd)); err != nil {
			return err
		}
	}
}

// String returns a string representation of the client.
func (c *Client) String() string {
	if c.nick == "" {
		return "unknown"
	}
	return c.nick
}
