package server

import (
	"fmt"
	"log"
	"time"

	"ircserver/internal/persistence"
)

// EventType represents different types of IRC events
type EventType string

const (
	EventConnect    EventType = "CONNECT"
	EventDisconnect EventType = "DISCONNECT"
	EventJoin       EventType = "JOIN"
	EventPart       EventType = "PART"
	EventMessage    EventType = "MESSAGE"
	EventNick       EventType = "NICK"
	EventTopic      EventType = "TOPIC"
)

// Logger handles all IRC event logging
type Logger struct {
	store persistence.Store
}

// NewLogger creates a new Logger instance
func NewLogger(store persistence.Store) *Logger {
	return &Logger{
		store: store,
	}
}

// LogEvent logs a general IRC event
func (l *Logger) LogEvent(eventType EventType, client *Client, target, details string) {
	// Log to console
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] %s: %s -> %s (%s)", 
		timestamp, eventType, client.String(), target, details)
	log.Printf("INFO: %s", logMsg)

	// Store in database
	if l.store != nil {
		l.store.LogMessage(client.String(), target, string(eventType), details)
	}
}

// LogMessage logs chat messages (PRIVMSG/NOTICE)
func (l *Logger) LogMessage(from *Client, target, msgType, content string) {
	// Log to console
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] %s from %s to %s: %s",
		timestamp, msgType, from.String(), target, content)
	log.Printf("INFO: %s", logMsg)

	// Store in database
	if l.store != nil {
		l.store.LogMessage(from.String(), target, msgType, content)
	}
}

// LogError logs error events
func (l *Logger) LogError(context string, err error) {
	// Log to console
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] ERROR: %s: %v",
		timestamp, context, err)
	log.Printf("%s", logMsg)

	// Store in database
	if l.store != nil {
		l.store.LogMessage("SERVER", "ERROR", "ERROR", fmt.Sprintf("%s: %v", context, err))
	}
}