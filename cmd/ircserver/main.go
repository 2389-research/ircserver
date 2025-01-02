package main

import (
	"flag"
	"log"

	"ircserver/internal/config"
	"ircserver/internal/persistence"
	"ircserver/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("FATAL: Configuration error: %v", err)
	}

	// Initialize database
	store, err := persistence.New(cfg.Storage.SQLitePath)
	if err != nil {
		log.Fatalf("FATAL: Database initialization error: %v", err)
	}
	defer store.Close()

	log.Printf("INFO: Starting IRC server %s with host=%s port=%s", 
		cfg.Server.Name, cfg.Server.Host, cfg.Server.Port)
	
	srv := server.New(cfg.Server.Host, cfg.Server.Port, store)
	srv.SetConfig(cfg)
	
	// Start web interface
	webServer, err := server.NewWebServer(srv)
	if err != nil {
		log.Fatalf("FATAL: Web server initialization error: %v", err)
	}
	
	// Start web server in a goroutine
	go func() {
		webAddr := ":" + cfg.Server.WebPort
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
