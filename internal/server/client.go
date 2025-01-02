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
	msgCount   int           // Messages sent in current window
	lastMsg    time.Time     // Start of current rate limit window
	msgQueue   chan string   // Queue for rate-limited messages
	lastPong   time.Time     // Last time we received a PONG
	pingTimer  *time.Timer   // Timer for PING timeout
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
		lastMsg:    time.Now(),
		msgQueue:   make(chan string, 100), // Buffer up to 100 messages
	}
	
	// Start message queue processor
	go client.processMessageQueue()

	// Start idle timeout monitor
	go client.monitorIdle()
	go client.pingLoop()
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

	// For immediate responses, write directly
	if _, err := c.writer.WriteString(message + "\r\n"); err != nil {
		return err
	}
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
func (c *Client) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			// Send PING
			if err := c.Send(fmt.Sprintf("PING :%d", time.Now().Unix())); err != nil {
				log.Printf("ERROR: Failed to send PING to client %s: %v", c.String(), err)
				c.conn.Close()
				return
			}

			// Set/reset ping timeout timer
			if c.pingTimer != nil {
				c.pingTimer.Stop()
			}
			c.pingTimer = time.AfterFunc(10*time.Second, func() {
				log.Printf("INFO: Client %s timed out - no PONG response", c.String())
				c.conn.Close()
			})
		}
	}
}

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

		if cmd == "PONG" {
			c.mu.Lock()
			c.lastPong = time.Now()
			if c.pingTimer != nil {
				c.pingTimer.Stop()
			}
			c.mu.Unlock()
			continue
		} else if cmd == "NICK" {
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
				if err := c.Send(fmt.Sprintf("432 * %s :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _", nick)); err != nil {
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
			c.realname = strings.TrimPrefix(strings.Join(parts[4:], " "), ":")
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

func (c *Client) processMessageQueue() {
	ticker := time.NewTicker(time.Second / 10) // Process queue more frequently
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case msg := <-c.msgQueue:
			c.mu.Lock()
			now := time.Now()
			// Reset rate limit if window has expired
			if now.Sub(c.lastMsg) > time.Second*2 {
				c.msgCount = 0
				c.lastMsg = now
			}
			
			// Check rate limit
			if c.msgCount < 10 { // Max 10 messages per 2 seconds
				if _, err := c.writer.WriteString(msg + "\r\n"); err == nil {
					err = c.writer.Flush()
					if err == nil {
						c.msgCount++
					} else {
						log.Printf("ERROR: Failed to flush message to client %s: %v", c.String(), err)
					}
				} else {
					log.Printf("ERROR: Failed to write message to client %s: %v", c.String(), err)
				}
			}
			c.mu.Unlock()
		case <-ticker.C:
			c.mu.Lock()
			if time.Since(c.lastMsg) > time.Second*2 {
				c.msgCount = 0
				c.lastMsg = time.Now()
			}
			c.mu.Unlock()
		}
	}
}
