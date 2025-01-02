package main

import (
	"log"
	"os"

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

	srv := server.New(host, port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
