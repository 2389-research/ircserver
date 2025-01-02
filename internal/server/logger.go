package server

import (
	"context"
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
func (l *Logger) LogEvent(ctx context.Context, eventType EventType, client *Client, target, details string) error {
	// Log to console
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] %s: %s -> %s (%s)", 
		timestamp, eventType, client.String(), target, details)
	log.Printf("INFO: %s", logMsg)

	// Store in database with retry logic
	if l.store != nil {
		var err error
		for retries := 3; retries > 0; retries-- {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err = l.store.LogMessage(ctx, client.String(), target, string(eventType), details); err == nil {
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return fmt.Errorf("failed to log event after retries: %w", err)
	}
	return nil
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
		ctx := context.Background()
		if err := l.store.LogMessage(ctx, from.String(), target, msgType, content); err != nil {
			l.LogError("Failed to log message", err)
		}
	}
}

// LogError logs error events
func (l *Logger) LogError(msg string, err error) {
	// Log to console
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] ERROR: %s: %v",
		timestamp, msg, err)
	log.Printf("%s", logMsg)

	// Store in database
	if l.store != nil {
		ctx := context.Background()
		if err := l.store.LogMessage(ctx, "SERVER", "ERROR", "ERROR", fmt.Sprintf("%s: %v", msg, err)); err != nil {
			log.Printf("Failed to log error: %v", err)
		}
	}
}
