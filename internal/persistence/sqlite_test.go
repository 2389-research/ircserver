package persistence

import (
	"context"

	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func TestUserPersistence(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.UpdateUser(ctx, "testuser", "testuser", "Test User", "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	var nickname, username, realname, ipAddress string
	var lastSeen time.Time
	err = store.db.QueryRowContext(ctx, "SELECT nickname, username, realname, ip_address, last_seen FROM users WHERE nickname = ?", "testuser").Scan(&nickname, &username, &realname, &ipAddress, &lastSeen)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}

	if nickname != "testuser" || username != "testuser" || realname != "Test User" || ipAddress != "127.0.0.1" {
		t.Errorf("User data mismatch: got (%s, %s, %s, %s), want (%s, %s, %s, %s)", nickname, username, realname, ipAddress, "testuser", "testuser", "Test User", "127.0.0.1")
	}
}

func TestChannelPersistence(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.UpdateChannel(ctx, "#testchannel", "Test Topic")
	if err != nil {
		t.Fatalf("Failed to update channel: %v", err)
	}

	var name, topic string
	var createdAt time.Time
	err = store.db.QueryRowContext(ctx, "SELECT name, topic, created_at FROM channels WHERE name = ?", "#testchannel").Scan(&name, &topic, &createdAt)
	if err != nil {
		t.Fatalf("Failed to query channel: %v", err)
	}

	if name != "#testchannel" || topic != "Test Topic" {
		t.Errorf("Channel data mismatch: got (%s, %s), want (%s, %s)", name, topic, "#testchannel", "Test Topic")
	}
}

func TestMessageLogging(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	err := store.LogMessage(ctx, "sender", "recipient", "PRIVMSG", "Hello, world!")
	if err != nil {
		t.Fatalf("Failed to log message: %v", err)
	}

	var timestamp time.Time
	var sender, recipient, msgType, content string
	err = store.db.QueryRowContext(ctx, "SELECT timestamp, sender, recipient, message_type, content FROM message_logs WHERE sender = ?", "sender").Scan(&timestamp, &sender, &recipient, &msgType, &content)
	if err != nil {
		t.Fatalf("Failed to query message log: %v", err)
	}

	if sender != "sender" || recipient != "recipient" || msgType != "PRIVMSG" || content != "Hello, world!" {
		t.Errorf("Message log data mismatch: got (%s, %s, %s, %s), want (%s, %s, %s, %s)", sender, recipient, msgType, content, "sender", "recipient", "PRIVMSG", "Hello, world!")
	}
}

func TestDatabaseErrors(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()

	// Test unique constraint violation on users
	err := store.UpdateUser(ctx, "duplicateuser", "user1", "User One", "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}
	err = store.UpdateUser(ctx, "duplicateuser", "user2", "User Two", "127.0.0.2")
	if err == nil {
		t.Fatal("Expected unique constraint violation, got nil")
	}

	// Test unique constraint violation on channels
	err = store.UpdateChannel(ctx, "#duplicatechannel", "Topic One")
	if err != nil {
		t.Fatalf("Failed to update channel: %v", err)
	}
	err = store.UpdateChannel(ctx, "#duplicatechannel", "Topic Two")
	if err == nil {
		t.Fatal("Expected unique constraint violation, got nil")
	}
}

func TestConcurrentDatabaseAccess(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nickname := "user" + string(i)
			err := store.UpdateUser(ctx, nickname, nickname, "User "+string(i), "127.0.0.1")
			if err != nil {
				t.Errorf("Failed to update user %d: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	var count int
	err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}

	if count != numGoroutines {
		t.Errorf("Expected %d users, got %d", numGoroutines, count)
	}
}
