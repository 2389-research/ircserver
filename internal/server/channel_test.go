package server

import (
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
