package main

import (
	"log"
	"os"

	"ircserver/internal/persistence"
	"ircserver/internal/server"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	// Default to localhost:6667 if not specified
	host := getEnv("IRC_HOST", "localhost")
	port := getEnv("IRC_PORT", "6667")
	dbPath := getEnv("IRC_DB_PATH", "irc.db")

	// Initialize database
	store, err := persistence.New(dbPath)
	if err != nil {
		log.Fatalf("FATAL: Database initialization error: %v", err)
	}
	defer store.Close()

	log.Printf("INFO: Starting IRC server with host=%s port=%s", host, port)
	srv := server.New(host, port, store)
	
	// Start web interface
	webServer, err := server.NewWebServer(srv)
	if err != nil {
		log.Fatalf("FATAL: Web server initialization error: %v", err)
	}
	
	// Start web server in a goroutine
	go func() {
		webAddr := getEnv("IRC_WEB_ADDR", ":8080")
		log.Printf("INFO: Starting web interface on %s", webAddr)
		if err := webServer.Start(webAddr); err != nil {
			log.Fatalf("FATAL: Web server error: %v", err)
		}
	}()
	
	// Start IRC server
	if err := srv.Start(); err != nil {
		log.Fatalf("FATAL: Server error: %v", err)
	}
}
