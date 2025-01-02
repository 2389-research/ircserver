package persistence

// Store defines the interface for persistence operations
type Store interface {
	// Close closes the underlying database connection
	Close() error
	
	// LogMessage stores a message in the database
	LogMessage(sender, recipient, msgType, content string) error
	
	// UpdateUser stores or updates user information
	UpdateUser(nickname, username, realname, ipAddr string) error
	
	// UpdateChannel stores or updates channel information
	UpdateChannel(name, topic string) error
}
