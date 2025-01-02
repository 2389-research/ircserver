package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

// Server represents an IRC server instance
type Server struct {
	host     string
	port     string
	clients  map[string]*Client
	channels map[string]map[string]bool
	mu       sync.RWMutex
}

// New creates a new IRC server instance
func New(host, port string) *Server {
	return &Server{
		host:     host,
		port:     port,
		clients:  make(map[string]*Client),
		channels: make(map[string]map[string]bool),
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

	client := NewClient(conn)
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Client %s disconnected: %v", client, err)
			s.removeClient(client)
			return
		}

		message = strings.TrimSpace(message)
		s.handleMessage(client, message)
	}
}

func (s *Server) handleMessage(client *Client, message string) {
	if message == "" {
		return
	}

	parts := strings.SplitN(message, " ", 2)
	command := strings.ToUpper(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch command {
	case "NICK":
		s.handleNick(client, args)
	case "USER":
		s.handleUser(client, args)
	case "QUIT":
		s.handleQuit(client, args)
	case "JOIN":
		s.handleJoin(client, args)
	case "PART":
		s.handlePart(client, args)
	case "PRIVMSG":
		s.handlePrivMsg(client, args)
	case "NOTICE":
		s.handleNotice(client, args)
	case "PING":
		s.handlePing(client, args)
	default:
		log.Printf("Unknown command from %s: %s", client, command)
	}
}

func (s *Server) handleNick(client *Client, args string) {
	newNick := strings.TrimSpace(args)
	if newNick == "" {
		client.Send(":server 431 :No nickname given")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if nickname is already in use
	if _, exists := s.clients[newNick]; exists {
		client.Send(":server 433 * " + newNick + " :Nickname is already in use")
		return
	}

	// If client already had a nickname, update the clients map
	if client.nick != "" {
		delete(s.clients, client.nick)
	}

	client.nick = newNick
	s.clients[newNick] = client
	client.Send(fmt.Sprintf(":%s NICK %s", client, newNick))
}

func (s *Server) handleUser(client *Client, args string) {
	parts := strings.SplitN(args, " ", 4)
	if len(parts) < 4 {
		client.Send(":server 461 USER :Not enough parameters")
		return
	}

	client.username = parts[0]
	client.realname = strings.TrimPrefix(parts[3], ":")
	
	// Send welcome messages
	client.Send(fmt.Sprintf(":server 001 %s :Welcome to the IRC Network %s!%s@%s",
		client.nick, client.nick, client.username, client.conn.RemoteAddr().String()))
}

func (s *Server) handleQuit(client *Client, args string) {
	quitMsg := "Quit"
	if args != "" {
		quitMsg = args
	}

	s.removeClient(client)
	client.Send(fmt.Sprintf("ERROR :Closing Link: %s (%s)", client, quitMsg))
	client.conn.Close()
}

func (s *Server) handleJoin(client *Client, args string) {
	channels := strings.Split(args, ",")
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}

		s.mu.Lock()
		if s.channels[channel] == nil {
			s.channels[channel] = make(map[string]bool)
		}
		s.channels[channel][client.nick] = true
		client.channels[channel] = true
		s.mu.Unlock()

		// Notify all clients in the channel
		s.broadcastToChannel(channel, fmt.Sprintf(":%s JOIN %s", client, channel))
	}
}

func (s *Server) handlePart(client *Client, args string) {
	if args == "" {
		client.Send(":server 461 PART :Not enough parameters")
		return
	}

	channels := strings.Split(args, ",")
	for _, channel := range channels {
		channelName := strings.TrimSpace(channel)
		s.mu.Lock()
		if s.channels[channelName] != nil {
			delete(s.channels[channelName], client.nick)
			delete(client.channels, channelName)
			s.mu.Unlock()
			s.broadcastToChannel(channelName, fmt.Sprintf(":%s PART %s", client, channelName))
		} else {
			s.mu.Unlock()
			client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, channelName))
		}
	}
}

func (s *Server) handlePrivMsg(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		client.Send(":server 461 PRIVMSG :Not enough parameters")
		return
	}

	target := parts[0]
	message := strings.TrimPrefix(parts[1], ":")
	s.deliverMessage(client, target, "PRIVMSG", message)
}

func (s *Server) handleNotice(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return // NOTICE doesn't send error replies
	}

	target := parts[0]
	message := strings.TrimPrefix(parts[1], ":")
	s.deliverMessage(client, target, "NOTICE", message)
}

func (s *Server) handlePing(client *Client, args string) {
	client.Send(fmt.Sprintf("PONG :%s", args))
}

func (s *Server) deliverMessage(from *Client, target, msgType, message string) {
	if strings.HasPrefix(target, "#") {
		s.mu.RLock()
		if channel, exists := s.channels[target]; exists {
			s.mu.RUnlock()
			s.broadcastToChannel(target, fmt.Sprintf(":%s %s %s :%s", from, msgType, target, message))
		} else {
			s.mu.RUnlock()
			from.Send(fmt.Sprintf(":server 403 %s %s :No such channel", from.nick, target))
		}
	} else {
		s.mu.RLock()
		if to, exists := s.clients[target]; exists {
			s.mu.RUnlock()
			to.Send(fmt.Sprintf(":%s %s %s :%s", from, msgType, target, message))
		} else {
			s.mu.RUnlock()
			from.Send(fmt.Sprintf(":server 401 %s %s :No such nick/channel", from.nick, target))
		}
	}
}

func (s *Server) broadcastToChannel(channel string, message string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if channelClients, exists := s.channels[channel]; exists {
		for nick := range channelClients {
			if client, ok := s.clients[nick]; ok {
				client.Send(message)
			}
		}
	}
}

func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from channels
	for channel := range client.channels {
		if s.channels[channel] != nil {
			delete(s.channels[channel], client.nick)
			// If channel is empty, remove it
			if len(s.channels[channel]) == 0 {
				delete(s.channels, channel)
			}
		}
	}

	// Remove from clients list
	if client.nick != "" {
		delete(s.clients, client.nick)
	}
}
