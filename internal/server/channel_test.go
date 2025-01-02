package server

import (
	"sync"
	"testing"
	"time"
)

func TestNewChannel(t *testing.T) {
	ch := NewChannel("#test")
	if ch.Name != "#test" {
		t.Errorf("Expected channel name '#test', got %s", ch.Name)
	}
	if ch.Members == nil {
		t.Error("Members map not initialized")
	}
	if ch.modes == nil {
		t.Error("Modes map not initialized")
	}
}

func TestChannelMemberOperations(t *testing.T) {
	ch := NewChannel("#test")
	client := &Client{nick: "testuser"}

	// Test adding member
	ch.AddClient(client, UserModeNormal)
	if !ch.HasMember("testuser") {
		t.Error("Expected member to be in channel")
	}

	// Test getting member mode
	if mode, exists := ch.GetMemberMode("testuser"); !exists || mode != UserModeNormal {
		t.Error("Expected member to have normal mode")
	}

	// Test setting member mode
	ch.SetMemberMode("testuser", UserModeOperator)
	if mode, _ := ch.GetMemberMode("testuser"); mode != UserModeOperator {
		t.Error("Expected member to be operator")
	}

	// Test removing member
	ch.RemoveClient("testuser")
	if ch.HasMember("testuser") {
		t.Error("Expected member to be removed")
	}
}

func TestChannelPassword(t *testing.T) {
	ch := NewChannel("#test")

	// Test setting password
	ch.SetPassword("secret")
	if !ch.CheckPassword("secret") {
		t.Error("Expected password check to succeed")
	}
	if ch.CheckPassword("wrong") {
		t.Error("Expected wrong password check to fail")
	}

	// Test removing password
	ch.SetPassword("")
	if !ch.CheckPassword("") {
		t.Error("Expected empty password check to succeed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ch := NewChannel("#test")
	done := make(chan bool)

	// Simulate concurrent access
	for i := 0; i < 10; i++ {
		go func(id int) {
			client := &Client{nick: string(rune('A' + id))}
			ch.AddClient(client, UserModeNormal)
			time.Sleep(time.Millisecond)
			ch.SetMemberMode(client.nick, UserModeOperator)
			time.Sleep(time.Millisecond)
			ch.RemoveClient(client.nick)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if !ch.IsEmpty() {
		t.Error("Expected channel to be empty after all operations")
	}
}

func TestGetMembers(t *testing.T) {
	ch := NewChannel("#test")
	clients := []*Client{
		{nick: "user1"},
		{nick: "user2"},
		{nick: "user3"},
	}

	// Add clients with different modes
	ch.AddClient(clients[0], UserModeNormal)
	ch.AddClient(clients[1], UserModeVoice)
	ch.AddClient(clients[2], UserModeOperator)

	members := ch.GetMembers()
	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}

	// Verify modes
	modes := make(map[string]UserMode)
	for _, member := range members {
		modes[member.Client.nick] = member.Mode
	}

	expectedModes := map[string]UserMode{
		"user1": UserModeNormal,
		"user2": UserModeVoice,
		"user3": UserModeOperator,
	}

	for nick, expectedMode := range expectedModes {
		if mode, exists := modes[nick]; !exists || mode != expectedMode {
			t.Errorf("Expected mode %v for user %s, got %v", expectedMode, nick, mode)
		}
	}
}

// TestConcurrentChannelJoins tests concurrent channel joins to validate safe operation
func TestConcurrentChannelJoins(t *testing.T) {
	const numClients = 100
	const numChannels = 10

	channels := make([]*Channel, numChannels)
	for i := 0; i < numChannels; i++ {
		channels[i] = NewChannel(fmt.Sprintf("#channel%d", i))
	}

	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{nick: fmt.Sprintf("user%d", i)}
	}

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			for j := 0; j < numChannels; j++ {
				channels[j].AddClient(client, UserModeNormal)
			}
		}(clients[i])
	}

	wg.Wait()

	// Verify all clients are in all channels
	for i := 0; i < numChannels; i++ {
		members := channels[i].GetMembers()
		if len(members) != numClients {
			t.Errorf("Expected %d members in channel %s, got %d", numClients, channels[i].Name, len(members))
		}
	}
}

// TestConcurrentChannelLeaves tests concurrent channel leaves to validate safe operation
func TestConcurrentChannelLeaves(t *testing.T) {
	const numClients = 100
	const numChannels = 10

	channels := make([]*Channel, numChannels)
	for i := 0; i < numChannels; i++ {
		channels[i] = NewChannel(fmt.Sprintf("#channel%d", i))
	}

	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{nick: fmt.Sprintf("user%d", i)}
	}

	// Join all clients to all channels
	for i := 0; i < numClients; i++ {
		for j := 0; j < numChannels; j++ {
			channels[j].AddClient(clients[i], UserModeNormal)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			for j := 0; j < numChannels; j++ {
				channels[j].RemoveClient(client.nick)
			}
		}(clients[i])
	}

	wg.Wait()

	// Verify all channels are empty
	for i := 0; i < numChannels; i++ {
		if !channels[i].IsEmpty() {
			t.Errorf("Expected channel %s to be empty, but it has members", channels[i].Name)
		}
	}
}

// TestConcurrentMessageDeliveries tests concurrent message deliveries to validate safe operation
func TestConcurrentMessageDeliveries(t *testing.T) {
	const numClients = 100
	const numChannels = 10
	const numMessages = 1000

	channels := make([]*Channel, numChannels)
	for i := 0; i < numChannels; i++ {
		channels[i] = NewChannel(fmt.Sprintf("#channel%d", i))
	}

	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{nick: fmt.Sprintf("user%d", i)}
	}

	// Join all clients to all channels
	for i := 0; i < numClients; i++ {
		for j := 0; j < numChannels; j++ {
			channels[j].AddClient(clients[i], UserModeNormal)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(msgNum int) {
			defer wg.Done()
			client := clients[msgNum%numClients]
			channel := channels[msgNum%numChannels]
			msg := fmt.Sprintf("Message %d", msgNum)
			channel.deliverMessage(client, "PRIVMSG", msg)
		}(i)
	}

	wg.Wait()

	// Verify message delivery
	for i := 0; i < numClients; i++ {
		client := clients[i]
		for j := 0; j < numChannels; j++ {
			channel := channels[j]
			if !strings.Contains(client.conn.(*mockConn).writeData.String(), channel.Name) {
				t.Errorf("Expected messages to be delivered to channel %s for client %s", channel.Name, client.nick)
			}
		}
	}
}
