package persistence

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store handles all database operations
type Store struct {
	db *sql.DB
}

// New creates a new database connection and initializes tables
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
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

// LogMessage stores a message in the database
func (s *Store) LogMessage(sender, recipient, msgType, content string) error {
	query := `INSERT INTO message_logs (timestamp, sender, recipient, message_type, content)
			 VALUES (?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, time.Now(), sender, recipient, msgType, content)
	if err != nil {
		log.Printf("ERROR: Failed to log message: %v", err)
	}
	return err
}

// UpdateUser stores or updates user information
func (s *Store) UpdateUser(nickname, username, realname, ipAddr string) error {
	query := `INSERT OR REPLACE INTO users (nickname, username, realname, ip_address, last_seen)
			 VALUES (?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, nickname, username, realname, ipAddr, time.Now())
	if err != nil {
		log.Printf("ERROR: Failed to update user: %v", err)
	}
	return err
}

// UpdateChannel stores or updates channel information
func (s *Store) UpdateChannel(name, topic string) error {
	query := `INSERT OR REPLACE INTO channels (name, topic, created_at)
			 VALUES (?, ?, ?)`
	
	_, err := s.db.Exec(query, name, topic, time.Now())
	if err != nil {
		log.Printf("ERROR: Failed to update channel: %v", err)
	}
	return err
}
