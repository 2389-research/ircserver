package main

import (
	"crypto/tls"
	"log"
	"os"
	"strconv"

	"ircserver/internal/server"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	// Server configuration
	host := getEnv("IRC_HOST", "localhost")
	port := getEnv("IRC_PORT", "6667")
	useTLS := getEnv("IRC_USE_TLS", "true") == "true"
	certFile := getEnv("IRC_CERT_FILE", "server.crt")
	keyFile := getEnv("IRC_KEY_FILE", "server.key")

	var tlsConfig *tls.Config
	if useTLS {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatalf("FATAL: Failed to load TLS certificates: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:  tls.VersionTLS12,
		}
	}

	log.Printf("INFO: Starting IRC server with host=%s port=%s tls=%v", host, port, useTLS)
	srv := server.New(host, port, tlsConfig)
	if err := srv.Start(); err != nil {
		log.Fatalf("FATAL: Server error: %v", err)
	}
}
