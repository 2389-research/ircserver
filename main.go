package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	// Default to localhost:6667 if not specified
	host := getEnv("IRC_HOST", "localhost")
	port := getEnv("IRC_PORT", "6667")
	addr := fmt.Sprintf("%s:%s", host, port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("IRC server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("New connection from %s", remoteAddr)

	// Keep the connection open but don't process any IRC commands yet
	// Just wait until the client disconnects
	buffer := make([]byte, 1024)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Client %s disconnected: %v", remoteAddr, err)
			return
		}
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
