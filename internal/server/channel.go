package server

import (
	"log"
	"sync"
	"time"
)

// Channel represents an IRC channel
type Channel struct {
	Name    string
	Topic   string
	Created time.Time
	Clients map[string]*Client // Map of nickname -> client
	mu      sync.RWMutex
}

// NewChannel creates a new IRC channel
func NewChannel(name string) *Channel {
	ch := &Channel{
		Name:    name,
		Created: time.Now(),
		Clients: make(map[string]*Client),
	}
	log.Printf("INFO: New channel created: %s", name)
	return ch
}

// AddClient adds a client to the channel
func (ch *Channel) AddClient(client *Client) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.Clients[client.nick] = client
	log.Printf("INFO: Client %s joined channel %s", client.nick, ch.Name)
}

// RemoveClient removes a client from the channel
func (ch *Channel) RemoveClient(nickname string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.Clients, nickname)
	log.Printf("INFO: Client %s left channel %s", nickname, ch.Name)
}

// SetTopic sets the channel topic
func (ch *Channel) SetTopic(topic string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.Topic = topic
}

// GetTopic returns the channel topic
func (ch *Channel) GetTopic() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.Topic
}

// GetClients returns a list of all clients in the channel
func (ch *Channel) GetClients() []*Client {
	ch.mu.RLock()
	// Create fixed-size slice upfront
	clientMap := make(map[string]*Client, len(ch.Clients))
	for nick, client := range ch.Clients {
		clientMap[nick] = client
	}
	ch.mu.RUnlock()

	// Convert map to slice outside the lock
	clients := make([]*Client, 0, len(clientMap))
	for _, client := range clientMap {
		clients = append(clients, client)
	}
	return clients
}

// HasClient checks if a client is in the channel
func (ch *Channel) HasClient(nickname string) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	_, exists := ch.Clients[nickname]
	return exists
}
