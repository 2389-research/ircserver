package server

import (
	"log"
	"sync"
	"time"
)

// UserMode represents the mode of a user in a channel
type UserMode int

const (
	UserModeNormal UserMode = iota
	UserModeVoice
	UserModeOperator
)

// ChannelMember represents a client and their mode in a channel
type ChannelMember struct {
	Client *Client
	Mode   UserMode
}

// Channel represents an IRC channel.
type Channel struct {
	Name     string
	Topic    string
	Created  time.Time
	Members  map[string]*ChannelMember // Map of nickname -> member
	modes    map[string]bool          // Channel modes
	mu       sync.RWMutex             // Protects all mutable state
	password string                   // Optional channel password
}

// channelState represents an immutable snapshot of channel state
type channelState struct {
	name     string
	topic    string
	members  map[string]*ChannelMember
	modes    map[string]bool
	password string
}

// getState creates a deep copy of channel state under read lock
func (ch *Channel) getState() channelState {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	members := make(map[string]*ChannelMember, len(ch.Members))
	for k, v := range ch.Members {
		members[k] = &ChannelMember{
			Client: v.Client,
			Mode:   v.Mode,
		}
	}

	modes := make(map[string]bool, len(ch.modes))
	for k, v := range ch.modes {
		modes[k] = v
	}

	return channelState{
		name:     ch.Name,
		topic:    ch.Topic,
		members:  members,
		modes:    modes,
		password: ch.password,
	}
}

// channelSnapshot represents an immutable copy of channel state
type channelSnapshot struct {
	members map[string]*ChannelMember
	topic   string
}

// snapshot creates a copy of relevant channel state under read lock
func (ch *Channel) snapshot() channelSnapshot {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	members := make(map[string]*ChannelMember, len(ch.Members))
	for k, v := range ch.Members {
		members[k] = v
	}
	
	return channelSnapshot{
		members: members,
		topic:   ch.Topic,
	}
}

// NewChannel creates a new IRC channel.
func NewChannel(name string) *Channel {
	ch := &Channel{
		Name:     name,
		Created:  time.Now(),
		Members:  make(map[string]*ChannelMember),
		modes:    make(map[string]bool),
	}
	log.Printf("INFO: New channel created: %s", name)
	return ch
}

// AddClient adds a client to the channel with specified mode.
func (ch *Channel) AddClient(client *Client, mode UserMode) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.Members[client.nick] = &ChannelMember{
		Client: client,
		Mode:   mode,
	}
	log.Printf("INFO: Client %s joined channel %s with mode %v", client.nick, ch.Name, mode)
}

// RemoveClient removes a client from the channel.
func (ch *Channel) RemoveClient(nickname string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.Members, nickname)
	log.Printf("INFO: Client %s left channel %s", nickname, ch.Name)

	// If channel is empty, it should be cleaned up by the server
	if len(ch.Members) == 0 {
		log.Printf("INFO: Channel %s is empty and ready for cleanup", ch.Name)
	}
}

// SetTopic sets the channel topic.
func (ch *Channel) SetTopic(topic string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.Topic = topic
}

// GetTopic returns the channel topic.
func (ch *Channel) GetTopic() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.Topic
}

// GetMembers returns a list of all members in the channel.
func (ch *Channel) GetMembers() []*ChannelMember {
	ch.mu.RLock()
	memberMap := make(map[string]*ChannelMember, len(ch.Members))
	for nick, member := range ch.Members {
		memberMap[nick] = member
	}
	ch.mu.RUnlock()

	members := make([]*ChannelMember, 0, len(memberMap))
	for _, member := range memberMap {
		members = append(members, member)
	}
	return members
}

// HasMember checks if a client is in the channel.
func (ch *Channel) HasMember(nickname string) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	_, exists := ch.Members[nickname]
	return exists
}

// GetMemberMode returns the mode of a channel member.
func (ch *Channel) GetMemberMode(nickname string) (UserMode, bool) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if member, exists := ch.Members[nickname]; exists {
		return member.Mode, true
	}
	return UserModeNormal, false
}

// SetMemberMode sets the mode of a channel member.
func (ch *Channel) SetMemberMode(nickname string, mode UserMode) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if member, exists := ch.Members[nickname]; exists {
		member.Mode = mode
		return true
	}
	return false
}

// SetPassword sets the channel password.
func (ch *Channel) SetPassword(password string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.password = password
	if password != "" {
		ch.modes["k"] = true
	} else {
		delete(ch.modes, "k")
	}
}

// CheckPassword verifies the channel password.
func (ch *Channel) CheckPassword(password string) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.password == "" || ch.password == password
}

// IsEmpty returns true if the channel has no members.
func (ch *Channel) IsEmpty() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.Members) == 0
}
