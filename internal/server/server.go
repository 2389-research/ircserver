package server

import (
	"fmt"
	"log"
	"net"
)

// Server represents an IRC server instance
type Server struct {
	host string
	port string
}

// New creates a new IRC server instance
func New(host, port string) *Server {
	return &Server{
		host: host,
		port: port,
	}
}

// Start begins listening for connections
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("IRC server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("New connection from %s", remoteAddr)

	buffer := make([]byte, 1024)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Client %s disconnected: %v", remoteAddr, err)
			return
		}
	}
}
