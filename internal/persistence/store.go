package persistence

import "context"

// Store defines the interface for persistence operations.
type Store interface {
	// Close closes the underlying database connection
	Close() error

	// LogMessage stores a message in the database
	LogMessage(ctx context.Context, sender, recipient, msgType, content string) error

	// UpdateUser stores or updates user information
	UpdateUser(ctx context.Context, nickname, username, realname, ipAddr string) error

	// UpdateChannel stores or updates channel information
	UpdateChannel(ctx context.Context, name, topic string) error
}
