package server

import (
	"bufio"
	"log"
	"net"
	"sync"
)

// Client represents a connected IRC client
type Client struct {
	conn     net.Conn
	nick     string
	username string
	realname string
	channels map[string]bool
	writer   *bufio.Writer
	mu       sync.Mutex
}

// NewClient creates a new IRC client instance
func NewClient(conn net.Conn) *Client {
	client := &Client{
		conn:     conn,
		channels: make(map[string]bool),
		writer:   bufio.NewWriter(conn),
	}
	log.Printf("INFO: New client connection from %s", conn.RemoteAddr().String())
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

// String returns a string representation of the client
func (c *Client) String() string {
	if c.nick == "" {
		return "unknown"
	}
	return c.nick
}
