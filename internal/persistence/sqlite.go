package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)


// Ensure SQLiteStore implements Store interface.
var _ Store = (*SQLiteStore)(nil)

// SQLiteStore handles all database operations.
type SQLiteStore struct {
	db *sql.DB
}

// New creates a new database connection and initializes tables.
func New(dbPath string) (*SQLiteStore, error) {
	store := &SQLiteStore{}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	store.db = db
	return store, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nickname TEXT UNIQUE,
			username TEXT,
			realname TEXT,
			ip_address TEXT,
			last_seen DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			topic TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS message_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME,
			sender TEXT,
			recipient TEXT,
			message_type TEXT,
			content TEXT
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}
	return nil
}

// LogMessage stores a message in the database.
func (s *SQLiteStore) LogMessage(ctx context.Context, sender, recipient, msgType, content string) error {
	query := `INSERT INTO message_logs (timestamp, sender, recipient, message_type, content)
			 VALUES (?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query, time.Now(), sender, recipient, msgType, content)
	if err != nil {
		return fmt.Errorf("failed to log message: %w", err)
	}
	return nil
}

// UpdateUser stores or updates user information.
func (s *SQLiteStore) UpdateUser(ctx context.Context, nickname, username, realname, ipAddr string) error {
	query := `INSERT OR REPLACE INTO users (nickname, username, realname, ip_address, last_seen)
			 VALUES (?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query, nickname, username, realname, ipAddr, time.Now())
	if err != nil {
		log.Printf("ERROR: Failed to update user: %v", err)
	}
	return err
}

// UpdateChannel stores or updates channel information.
func (s *SQLiteStore) UpdateChannel(ctx context.Context, name, topic string) error {
	query := `INSERT OR REPLACE INTO channels (name, topic, created_at)
			 VALUES (?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query, name, topic, time.Now())
	if err != nil {
		log.Printf("ERROR: Failed to update channel: %v", err)
	}
	return err
}

// QueryRow executes a query that returns a single row and scans the result into dest.
// This method is primarily intended for testing.
func (s *SQLiteStore) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}
