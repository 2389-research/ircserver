package main

import (
	"flag"
	"fmt"
	"ircserver/internal/config"
	"ircserver/internal/persistence"
	"ircserver/internal/server"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// getEnv retrieves an environment variable value or returns a fallback value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// CLI flags
	configPath := flag.String("config", "config.yaml", "path to config file")
	host := flag.String("host", "", "override server host from config")
	port := flag.String("port", "", "override server port from config")
	webPort := flag.String("web-port", "", "override web interface port from config")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("IRC Server v1.0.0")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("FATAL: Configuration error: %v", err)
	}

	// Override config with CLI flags if provided
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *port != "" {
		cfg.Server.Port = *port
	}
	if *webPort != "" {
		cfg.Server.WebPort = *webPort
	}

	// Initialize database
	store, err := persistence.New(cfg.Storage.SQLitePath)
	if err != nil {
		log.Fatalf("FATAL: Database initialization error: %v", err)
	}
	defer store.Close()

	log.Printf("INFO: Starting IRC server %s with host=%s port=%s",
		cfg.Server.Name, cfg.Server.Host, cfg.Server.Port)

	srv := server.New(cfg.Server.Host, cfg.Server.Port, store, cfg)

	// Start web interface
	webServer, err := server.NewWebServer(srv)
	if err != nil {
		log.Fatalf("FATAL: Web server initialization error: %v", err)
	}
	srv.SetWebServer(webServer)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start web server in a goroutine
	go func() {
		webAddr := ":" + cfg.Server.WebPort
		log.Printf("INFO: Starting web interface on %s", webAddr)
		if err := webServer.Start(webAddr); err != nil {
			log.Printf("ERROR: Web server error: %v", err)
		}
	}()

	// Start IRC server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("ERROR: Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("INFO: Received signal %v, initiating shutdown...", sig)

	// Graceful shutdown
	if err := srv.Shutdown(); err != nil {
		log.Printf("ERROR: Shutdown error: %v", err)
	}

	log.Println("INFO: Server shutdown complete")
}
