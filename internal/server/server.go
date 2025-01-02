package server

import (
	"bufio"
	"context"
	"fmt"
	"ircserver/internal/config"
	"ircserver/internal/persistence"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)
// Server represents an IRC server instance.
type Server struct {
	host      string
	port      string
	clients   map[string]*Client
	channels  map[string]*Channel
	store     persistence.Store
	logger    *Logger
	webServer *WebServer
	config    *config.Config
	listener  net.Listener
	shutdown  chan struct{}
	mu        sync.RWMutex
}

// New creates a new IRC server instance.
func New(host, port string, store persistence.Store, cfg *config.Config) *Server {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Server{
		host:      host,
		port:      port,
		clients:   make(map[string]*Client),
		channels:  make(map[string]*Channel),
		store:     store,
		logger:    NewLogger(store),
		webServer: nil,
		config:    cfg,
		shutdown:  make(chan struct{}),
	}
}

// Start begins listening for connections
// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	close(s.shutdown)

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("ERROR: Failed to close listener: %v", err)
		}
	}

	// Notify all clients
	s.mu.RLock()
	for _, client := range s.clients {
		if err := client.Send("ERROR :Server shutting down"); err != nil {
			log.Printf("ERROR: Failed to send shutdown message to client: %v", err)
		}
		client.conn.Close()
	}
	s.mu.RUnlock()

	// Shutdown web server if running
	if s.webServer != nil {
		if err := s.webServer.Shutdown(); err != nil {
			log.Printf("ERROR: Failed to shutdown web server: %v", err)
		}
	}

	return nil
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	log.Printf("INFO: IRC server started and listening on %s", addr)

	for {
		select {
		case <-s.shutdown:
			return nil
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				log.Printf("ERROR: Failed to accept connection: %v", err)
				continue
			}

			go func(conn net.Conn) {
				if err := s.handleConnection(conn); err != nil {
					log.Printf("ERROR: Connection handler error: %v", err)
				}
			}(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set initial connection timeouts
	if err := conn.SetReadDeadline(time.Now().Add(s.config.IRC.ReadTimeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(s.config.IRC.WriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	client := NewClient(conn, s.config)
	defer client.Close()
	reader := bufio.NewReader(conn)

	// Register client in a thread-safe way
	s.mu.Lock()
	s.clients[client.String()] = client
	s.mu.Unlock()

	if err := s.logger.LogEvent(ctx, EventConnect, client, "SERVER",
		fmt.Sprintf("from %s", conn.RemoteAddr())); err != nil {
		log.Printf("ERROR: Failed to log connect event: %v", err)
	}

	defer func() {
		s.removeClient(client)
		log.Printf("INFO: Client %s disconnected", client)
	}()

	for {
		// Reset read deadline before each read
		if err := conn.SetReadDeadline(time.Now().Add(s.config.IRC.ReadTimeout)); err != nil {
			log.Printf("ERROR: Failed to set read deadline: %v", err)
			return fmt.Errorf("failed to set read deadline: %w", err)
		}

		message, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("INFO: Read timeout for client %s", client)
			} else {
				log.Printf("ERROR: Read error for client %s: %v", client, err)
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Enforce message size limit
		if len(message) > s.config.IRC.MaxBufferSize {
			log.Printf("WARN: Oversized message from client %s: %d bytes", client, len(message))
			if err := client.Send(":server ERROR :Message too long"); err != nil {
				log.Printf("ERROR: Failed to send message size error: %v", err)
			}
			continue
		}

		client.UpdateActivity()

		message = strings.TrimSpace(message)
		if err := s.handleMessage(ctx, client, message); err != nil {
			log.Printf("ERROR: Failed to handle message from %s: %v", client, err)
			return err
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, client *Client, message string) error {
	if message == "" {
		return nil
	}

	parts := strings.SplitN(message, " ", 2)
	command := strings.ToUpper(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	switch command {
	case "NICK":
		if err := s.handleNick(client, args); err != nil {
			log.Printf("ERROR: Failed to handle NICK command: %v", err)
		}
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
	case "WHO":
		s.handleWho(client, args)
	case "TOPIC":
		s.handleTopic(client, args)
	default:
		log.Printf("WARN: Unknown command from %s: %s", client, command)
	}
	return nil
}

func (s *Server) handleNick(client *Client, args string) error {
	newNick := strings.TrimSpace(args)
	if newNick == "" {
		err := NewError(ErrNoNicknameGiven, "No nickname given", nil)
		if err := client.Send(fmt.Sprintf(":server %s", err.Error())); err != nil {
			log.Printf("ERROR: Failed to send error message: %v", err)
		}
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if nickname is already in use
	// Check nickname validity first
	if !isValidNick(newNick) {
		if err := client.Send(fmt.Sprintf(":server 432 * %s :Erroneous nickname - must be 1-9 chars, start with letter, and contain only letters, numbers, - or _", newNick)); err != nil {
			log.Printf("ERROR: Failed to send invalid nickname error: %v", err)
		}
		return fmt.Errorf("invalid nickname format: %s", newNick)
	}

	// Then check for collisions
	if _, exists := s.clients[newNick]; exists {
		if err := client.Send(fmt.Sprintf(":server 433 * %s :Nickname is already in use", newNick)); err != nil {
			log.Printf("ERROR: Failed to send nickname in use error: %v", err)
		}
		return fmt.Errorf("nickname %s already in use", newNick)
	}

	// If client already had a nickname, update the clients map
	if client.nick != "" {
		delete(s.clients, client.nick)
	}

	client.nick = newNick
	s.clients[newNick] = client
	if err := client.Send(fmt.Sprintf(":%s NICK %s", client, newNick)); err != nil {
		log.Printf("ERROR: Failed to send nick change confirmation: %v", err)
	}
	return nil
}

func (s *Server) handleUser(client *Client, args string) {
	parts := strings.SplitN(args, " ", 4)
	if len(parts) < 4 {
		if err := client.Send(":server 461 USER :Not enough parameters"); err != nil {
			log.Printf("ERROR: Failed to send parameters error: %v", err)
		}
		return
	}

	client.username = parts[0]
	client.realname = strings.TrimPrefix(parts[3], ":")

	// Store user info in database
	ctx := context.Background()
	if err := s.store.UpdateUser(ctx, client.nick, client.username, client.realname,
		client.conn.RemoteAddr().String()); err != nil {
		log.Printf("ERROR: Failed to store user info: %v", err)
	}

	// Send welcome messages
	welcomeMsg := fmt.Sprintf(":server 001 %s :Welcome to the IRC Network %s!%s@%s",
		client.nick, client.nick, client.username, client.conn.RemoteAddr().String())
	log.Printf("INFO: New client registered - Nick: %s, Username: %s, Address: %s",
		client.nick, client.username, client.conn.RemoteAddr().String())
	if err := client.Send(welcomeMsg); err != nil {
		log.Printf("ERROR: Failed to send welcome message: %v", err)
	}
}

func (s *Server) handleQuit(client *Client, args string) {
	quitMsg := "Quit"
	if args != "" {
		quitMsg = strings.TrimPrefix(args, ":")
	}

	// Log the QUIT event before removing the client
	ctx := context.Background()
	if err := s.logger.LogEvent(ctx, EventQuit, client, "SERVER", quitMsg); err != nil {
		log.Printf("ERROR: Failed to log quit event: %v", err)
	}

	// Send quit message to all channels the client is in
	quitNotice := fmt.Sprintf(":%s QUIT :%s", client, quitMsg)
	for channelName := range client.channels {
		if err := s.broadcastToChannel(channelName, quitNotice); err != nil {
			log.Printf("ERROR: Failed to broadcast quit message to channel %s: %v", channelName, err)
		}
	}

	// Send final message to the quitting client
	if err := client.Send(fmt.Sprintf("ERROR :Closing Link: %s (%s)", client, quitMsg)); err != nil {
		log.Printf("ERROR: Failed to send quit message: %v", err)
	}

	s.removeClient(client)
	client.conn.Close()
}

func (s *Server) handleJoin(client *Client, args string) {
	// First validate all channel names
	channelNames := strings.Split(args, ",")
	for _, name := range channelNames {
		name = strings.TrimSpace(name)
		if !isValidChannelName(name) || strings.Contains(name, ",") {
			if err := client.Send(fmt.Sprintf(":server 403 %s %s :Invalid channel name", client.nick, name)); err != nil {
				log.Printf("ERROR: Failed to send invalid channel name error: %v", err)
			}
			return // Exit early if any channel name is invalid
		}
	}

	// Now process each valid channel
	for _, channelName := range channelNames {
		channelName = strings.TrimSpace(channelName)
		s.mu.Lock()
		channel, exists := s.channels[channelName]
		if !exists {
			channel = NewChannel(channelName)
			s.channels[channelName] = channel
		}
		channel.AddClient(client)
		client.channels[channelName] = true

		ctx := context.Background()
		if err := s.logger.LogEvent(ctx, EventJoin, client, channelName, ""); err != nil {
			log.Printf("ERROR: Failed to log join event: %v", err)
		}

		if err := s.store.UpdateChannel(ctx, channelName, channel.GetTopic()); err != nil {
			s.logger.LogError("Failed to store channel info", err)
		}

		s.mu.Unlock()

		// Send JOIN message to all clients in the channel
		joinMsg := fmt.Sprintf(":%s JOIN %s", client, channelName)
		if err := s.broadcastToChannel(channelName, joinMsg); err != nil {
			log.Printf("ERROR: Failed to broadcast join message: %v", err)
		}

		// Always send topic reply - either the topic or no topic message
		topic := channel.GetTopic()
		if topic != "" {
			if err := client.Send(fmt.Sprintf(":server 332 %s %s :%s", client.nick, channelName, topic)); err != nil {
				log.Printf("ERROR: Failed to send channel topic: %v", err)
			}
		} else {
			if err := client.Send(fmt.Sprintf(":server 331 %s %s :No topic is set", client.nick, channelName)); err != nil {
				log.Printf("ERROR: Failed to send no topic message: %v", err)
			}
		}

		// Send list of users in channel
		names := []string{}
		for _, c := range channel.GetClients() {
			names = append(names, c.nick)
		}
		if err := client.Send(fmt.Sprintf(":server 353 %s = %s :%s", client.nick, channelName, strings.Join(names, " "))); err != nil {
			log.Printf("ERROR: Failed to send channel names list: %v", err)
		}
		if err := client.Send(fmt.Sprintf(":server 366 %s %s :End of /NAMES list", client.nick, channelName)); err != nil {
			log.Printf("ERROR: Failed to send end of names list: %v", err)
		}
	}
}

func (s *Server) handlePart(client *Client, args string) {
	if args == "" {
		if err := client.Send(":server 461 PART :Not enough parameters"); err != nil {
			log.Printf("ERROR: Failed to send parameters error: %v", err)
		}
		return
	}

	channels := strings.Split(args, ",")
	for _, channelName := range channels {
		channelName = strings.TrimSpace(channelName)
		
		s.mu.Lock()
		channel, exists := s.channels[channelName]
		if !exists {
			s.mu.Unlock()
			if err := client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, channelName)); err != nil {
				log.Printf("ERROR: Failed to send no such channel error: %v", err)
			}
			continue
		}

		// Check if client is actually in the channel
		if !channel.HasClient(client.nick) {
			s.mu.Unlock()
			if err := client.Send(fmt.Sprintf(":server 442 %s %s :You're not on that channel", client.nick, channelName)); err != nil {
				log.Printf("ERROR: Failed to send not on channel error: %v", err)
			}
			continue
		}

		// Send PART message to all clients in the channel (including the leaving client)
		partMsg := fmt.Sprintf(":%s PART %s", client, channelName)
		if err := s.broadcastToChannel(channelName, partMsg); err != nil {
			log.Printf("ERROR: Failed to broadcast PART message: %v", err)
		}

		// Remove client from channel
		channel.RemoveClient(client.nick)
		delete(client.channels, channelName)

		// Log the PART event
		ctx := context.Background()
		if err := s.logger.LogEvent(ctx, EventPart, client, channelName, ""); err != nil {
			log.Printf("ERROR: Failed to log part event: %v", err)
		}

		// Check if channel is now empty after removing the client
		if len(channel.Clients) == 0 {
			delete(s.channels, channelName)
			if err := s.logger.LogEvent(ctx, EventChannelDelete, client, channelName, "Channel removed - last user left"); err != nil {
				log.Printf("ERROR: Failed to log channel deletion: %v", err)
			}
		}
		
		s.mu.Unlock()
	}
}

func (s *Server) handlePrivMsg(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 || parts[0] == "" {
		if err := client.Send(fmt.Sprintf(":server 411 %s :No recipient given", client.nick)); err != nil {
			log.Printf("ERROR: Failed to send no recipient error: %v", err)
		}
		return
	}

	target := parts[0]
	message := strings.TrimPrefix(parts[1], ":")
	
	if message == "" {
		if err := client.Send(fmt.Sprintf(":server 412 %s :No text to send", client.nick)); err != nil {
			log.Printf("ERROR: Failed to send no text error: %v", err)
		}
		return
	}

	// Validate target format
	if target == "" {
		if err := client.Send(":server 411 :No recipient given"); err != nil {
			log.Printf("ERROR: Failed to send no recipient error: %v", err)
		}
		return
	}

	// Validate message
	if message == "" {
		if err := client.Send(":server 412 :No text to send"); err != nil {
			log.Printf("ERROR: Failed to send no text error: %v", err)
		}
		return
	}

	s.logger.LogMessage(client, target, "PRIVMSG", message)

	// Handle multiple targets separated by commas
	targets := strings.Split(target, ",")
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" {
			s.deliverMessage(client, t, "PRIVMSG", message)
		}
	}
}

func (s *Server) handleNotice(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return // NOTICE doesn't send error replies
	}

	target := parts[0]
	message := strings.TrimPrefix(parts[1], ":")

	s.logger.LogMessage(client, target, "NOTICE", message)

	s.deliverMessage(client, target, "NOTICE", message)
}

func (s *Server) handlePing(client *Client, args string) {
	if err := client.Send(fmt.Sprintf("PONG :%s", args)); err != nil {
		log.Printf("ERROR: Failed to send PONG response: %v", err)
	}
}

func (s *Server) handleTopic(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 1 {
		if err := client.Send(":server 461 TOPIC :Not enough parameters"); err != nil {
			log.Printf("ERROR: Failed to send parameters error: %v", err)
		}
		return
	}

	channelName := parts[0]
	s.mu.RLock()
	channel, exists := s.channels[channelName]
	s.mu.RUnlock()

	if !exists {
		if err := client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, channelName)); err != nil {
			log.Printf("ERROR: Failed to send no such channel error: %v", err)
		}
		return
	}

	// If no topic is provided, show the current topic
	if len(parts) == 1 {
		topic := channel.GetTopic()
		if topic == "" {
			if err := client.Send(fmt.Sprintf(":server 331 %s %s :No topic is set", client.nick, channelName)); err != nil {
				log.Printf("ERROR: Failed to send no topic message: %v", err)
			}
		} else {
			if err := client.Send(fmt.Sprintf(":server 332 %s %s :%s", client.nick, channelName, topic)); err != nil {
				log.Printf("ERROR: Failed to send topic message: %v", err) 
			}
		}
		return
	}

	// Set new topic
	newTopic := strings.TrimPrefix(parts[1], ":")
	channel.SetTopic(newTopic)

	// Broadcast the topic change to all channel members
	topicMsg := fmt.Sprintf(":%s TOPIC %s :%s", client, channelName, newTopic)
	if err := s.broadcastToChannel(channelName, topicMsg); err != nil {
		log.Printf("ERROR: Failed to broadcast topic change: %v", err)
	}

	// Log the topic change
	ctx := context.Background()
	if err := s.logger.LogEvent(ctx, EventTopic, client, channelName, newTopic); err != nil {
		log.Printf("ERROR: Failed to log topic event: %v", err)
	}
}

func (s *Server) handleWho(client *Client, args string) {
	target := strings.TrimSpace(args)
	if target == "" {
		if err := client.Send(":server 461 WHO :Not enough parameters"); err != nil {
			log.Printf("ERROR: Failed to send parameters error: %v", err)
		}
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.HasPrefix(target, "#") {
		// WHO for channel
		channel, exists := s.channels[target]
		if !exists {
			if err := client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, target)); err != nil {
				log.Printf("ERROR: Failed to send no channel error: %v", err)
			}
			return
		}

		for _, member := range channel.GetClients() {
			// <channel> <username> <host> <server> <nick> <H|G>[*][@|+] :<hopcount> <real name>
			if err := client.Send(fmt.Sprintf(":server 352 %s %s %s %s server %s H :0 %s",
				client.nick, target, member.username,
				member.conn.RemoteAddr().String(),
				member.nick, member.realname)); err != nil {
				log.Printf("ERROR: Failed to send WHO response: %v", err)
			}
		}
	} else {
		// WHO for user
		if targetClient, exists := s.clients[target]; exists {
			if err := client.Send(fmt.Sprintf(":server 352 %s * %s %s server %s H :0 %s",
				client.nick, targetClient.username,
				targetClient.conn.RemoteAddr().String(),
				targetClient.nick, targetClient.realname)); err != nil {
				log.Printf("ERROR: Failed to send WHO response: %v", err) 
			}
		}
	}

	if err := client.Send(fmt.Sprintf(":server 315 %s %s :End of WHO list", client.nick, target)); err != nil {
		log.Printf("ERROR: Failed to send end of WHO list: %v", err)
	}
}

func (s *Server) deliverMessage(from *Client, target, msgType, message string) {
	// Track message in web interface if available
	s.mu.RLock()
	if s.webServer != nil {
		s.webServer.AddMessage(from.String(), target, msgType, message)
	}
	s.mu.RUnlock()

	if strings.HasPrefix(target, "#") {
		s.mu.RLock()
		if _, exists := s.channels[target]; exists {
			s.mu.RUnlock()
			if err := s.broadcastToChannel(target, fmt.Sprintf(":%s %s %s :%s", from, msgType, target, message)); err != nil {
				log.Printf("ERROR: Failed to broadcast channel message: %v", err)
			}
		} else {
			s.mu.RUnlock()
			if err := from.Send(fmt.Sprintf(":server 403 %s %s :No such channel", from.nick, target)); err != nil {
				log.Printf("ERROR: Failed to send no such channel error: %v", err)
			}
		}
	} else {
		s.mu.RLock()
		if to, exists := s.clients[target]; exists {
			s.mu.RUnlock()
			if err := to.Send(fmt.Sprintf(":%s %s %s :%s", from, msgType, target, message)); err != nil {
				log.Printf("ERROR: Failed to deliver message to %s: %v", target, err)
			}
		} else {
			s.mu.RUnlock()
			if err := from.Send(fmt.Sprintf(":server 401 %s %s :No such nick/channel", from.nick, target)); err != nil {
				log.Printf("ERROR: Failed to send no such nick error: %v", err)
			}
		}
	}
}

func (s *Server) broadcastToChannel(channelName string, message string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channel, exists := s.channels[channelName]
	if !exists {
		return fmt.Errorf("channel %s does not exist", channelName)
	}

	channel.mu.RLock()
	clients := make([]*Client, 0, len(channel.Clients))
	for _, client := range channel.Clients {
		clients = append(clients, client)
	}
	channel.mu.RUnlock()

	// Send messages after releasing locks to prevent deadlocks
	var firstErr error
	for _, client := range clients {
		if err := client.Send(message); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to broadcast to %s: %w", client.String(), err)
			}
			log.Printf("ERROR: Failed to send message to client %s: %v", client.String(), err)
		}
	}
	return firstErr
}

// SetWebServer sets the web server reference.
func (s *Server) SetWebServer(ws *WebServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webServer = ws
}

// SetConfig sets the server configuration.
func (s *Server) SetConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

func (s *Server) removeClient(client *Client) {
	// First collect all channels the client is in
	s.mu.RLock()
	channels := make([]string, 0, len(client.channels))
	for channelName := range client.channels {
		channels = append(channels, channelName)
	}
	s.mu.RUnlock()

	// Broadcast departure to each channel
	for _, channelName := range channels {
		quitMsg := fmt.Sprintf(":%s QUIT :Client exiting", client)
		if err := s.broadcastToChannel(channelName, quitMsg); err != nil {
			log.Printf("ERROR: Failed to broadcast quit message: %v", err)
		}
	}

	// Now remove the client with write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from channels
	for _, channelName := range channels {
		if channel := s.channels[channelName]; channel != nil {
			channel.RemoveClient(client.nick)
			// If channel is empty, remove it
			if len(channel.GetClients()) == 0 {
				delete(s.channels, channelName)
			}
		}
	}

	// Remove from clients list
	if client.nick != "" {
		delete(s.clients, client.nick)
	}
}
