package persistence

import (
	"context"
	"strconv"
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

	// Test that we can update existing users
	err := store.UpdateUser(ctx, "testuser", "user1", "User One", "127.0.0.1")
	if err != nil {
		t.Fatalf("Failed to create initial user: %v", err)
	}
	
	// Update should succeed
	err = store.UpdateUser(ctx, "testuser", "user1-updated", "User One Updated", "127.0.0.2")
	if err != nil {
		t.Fatalf("Failed to update existing user: %v", err)
	}

	// Verify the update worked
	var username, realname string
	err = store.db.QueryRowContext(ctx, "SELECT username, realname FROM users WHERE nickname = ?", "testuser").Scan(&username, &realname)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}
	if username != "user1-updated" || realname != "User One Updated" {
		t.Errorf("User update failed: got (%s, %s), want (%s, %s)", 
			username, realname, "user1-updated", "User One Updated")
	}

	// Similar test for channels
	err = store.UpdateChannel(ctx, "#testchannel", "Topic One")
	if err != nil {
		t.Fatalf("Failed to create initial channel: %v", err)
	}

	err = store.UpdateChannel(ctx, "#testchannel", "Updated Topic")
	if err != nil {
		t.Fatalf("Failed to update existing channel: %v", err)
	}

	var topic string
	err = store.db.QueryRowContext(ctx, "SELECT topic FROM channels WHERE name = ?", "#testchannel").Scan(&topic)
	if err != nil {
		t.Fatalf("Failed to query updated channel: %v", err)
	}
	if topic != "Updated Topic" {
		t.Errorf("Channel update failed: got %s, want %s", topic, "Updated Topic")
	}
}

func TestConcurrentDatabaseAccess(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	ctx := context.Background()
	
	// Explicitly create tables and verify they exist
	if err := createTables(store.db); err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Verify tables exist by checking the schema
	var tableName string
	err := store.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		t.Fatalf("Failed to verify users table exists: %v", err)
	}

	// Initialize with test data
	if err := store.UpdateUser(ctx, "init", "init", "Initial User", "127.0.0.1"); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nickname := "user" + strconv.Itoa(i)
			err := store.UpdateUser(ctx, nickname, nickname, "User "+strconv.Itoa(i), "127.0.0.1")
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
