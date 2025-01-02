package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"ircserver/internal/config"
	"ircserver/internal/persistence"
)

// Server represents an IRC server instance
type Server struct {
	host       string
	port       string
	clients    map[string]*Client
	channels   map[string]*Channel
	store      persistence.Store
	logger     *Logger
	webServer  *WebServer
	config     *config.Config
	listener   net.Listener
	shutdown   chan struct{}
	mu         sync.RWMutex
}

// New creates a new IRC server instance
func New(host, port string, store persistence.Store) *Server {
	return &Server{
		host:      host,
		port:      port,
		clients:   make(map[string]*Client),
		channels:  make(map[string]*Channel),
		store:     store,
		logger:    NewLogger(store),
		webServer: nil,
		shutdown:  make(chan struct{}),
	}
}

// Start begins listening for connections
// Shutdown gracefully shuts down the server
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
		client.Send("ERROR :Server shutting down")
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

			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set initial connection timeouts
	conn.SetReadDeadline(time.Now().Add(s.config.IRC.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(s.config.IRC.WriteTimeout))

	client := NewClient(conn, s.config)
	defer client.Close()
	reader := bufio.NewReader(conn)

	// Register client in a thread-safe way
	s.mu.Lock()
	s.clients[client.String()] = client
	s.mu.Unlock()

	s.logger.LogEvent(EventConnect, client, "SERVER", 
		fmt.Sprintf("from %s", conn.RemoteAddr()))

	defer func() {
		s.removeClient(client)
		log.Printf("INFO: Client %s disconnected", client)
	}()

	for {
		// Reset read deadline before each read
		conn.SetReadDeadline(time.Now().Add(s.config.IRC.ReadTimeout))
		
		message, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("INFO: Read timeout for client %s", client)
			} else {
				log.Printf("ERROR: Read error for client %s: %v", client, err)
			}
			return
		}

		// Enforce message size limit
		if len(message) > s.config.IRC.MaxBufferSize {
			log.Printf("WARN: Oversized message from client %s: %d bytes", client, len(message))
			client.Send(":server ERROR :Message too long")
			continue
		}

		client.UpdateActivity()

		message = strings.TrimSpace(message)
		s.handleMessage(client, message)
	}
}

func (s *Server) handleMessage(ctx context.Context, client *Client, message string) error {
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
	case "WHO":
		s.handleWho(client, args)
	case "TOPIC":
		s.handleTopic(client, args)
	default:
		log.Printf("WARN: Unknown command from %s: %s", client, command)
	}
}

func (s *Server) handleNick(client *Client, args string) error {
	newNick := strings.TrimSpace(args)
	if newNick == "" {
		err := NewError(ErrNoNicknameGiven, "No nickname given", nil)
		client.Send(fmt.Sprintf(":server %s", err.Error()))
		return err
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

	// Store user info in database
	if err := s.store.UpdateUser(client.nick, client.username, client.realname, 
		client.conn.RemoteAddr().String()); err != nil {
		log.Printf("ERROR: Failed to store user info: %v", err)
	}

	// Send welcome messages
	welcomeMsg := fmt.Sprintf(":server 001 %s :Welcome to the IRC Network %s!%s@%s",
		client.nick, client.nick, client.username, client.conn.RemoteAddr().String())
	log.Printf("INFO: New client registered - Nick: %s, Username: %s, Address: %s", 
		client.nick, client.username, client.conn.RemoteAddr().String())
	client.Send(welcomeMsg)
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
	for _, channelName := range channels {
		channelName = strings.TrimSpace(channelName)
		if !strings.HasPrefix(channelName, "#") {
			channelName = "#" + channelName
		}

		s.mu.Lock()
		channel, exists := s.channels[channelName]
		if !exists {
			channel = NewChannel(channelName)
			s.channels[channelName] = channel
		}
		channel.AddClient(client)
		client.channels[channelName] = true
		
		s.logger.LogEvent(EventJoin, client, channelName, "")
		
		if err := s.store.UpdateChannel(channelName, channel.GetTopic()); err != nil {
			s.logger.LogError("Failed to store channel info", err)
		}
		
		s.mu.Unlock()

		// Send JOIN message to all clients in the channel
		s.broadcastToChannel(channelName, fmt.Sprintf(":%s JOIN %s", client, channelName))
		
		// Send channel topic if it exists
		if topic := channel.GetTopic(); topic != "" {
			client.Send(fmt.Sprintf(":server 332 %s %s :%s", client.nick, channelName, topic))
		}

		// Send list of users in channel
		names := []string{}
		for _, c := range channel.GetClients() {
			names = append(names, c.nick)
		}
		client.Send(fmt.Sprintf(":server 353 %s = %s :%s", client.nick, channelName, strings.Join(names, " ")))
		client.Send(fmt.Sprintf(":server 366 %s %s :End of /NAMES list", client.nick, channelName))
	}
}

func (s *Server) handlePart(client *Client, args string) {
	if args == "" {
		client.Send(":server 461 PART :Not enough parameters")
		return
	}

	channels := strings.Split(args, ",")
	for _, channelName := range channels {
		channelName = strings.TrimSpace(channelName)
		s.mu.Lock()
		channel, exists := s.channels[channelName]
		if exists {
			channel.RemoveClient(client.nick)
			delete(client.channels, channelName)
			
			// Remove channel if empty
			if len(channel.GetClients()) == 0 {
				delete(s.channels, channelName)
			}
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
	
	s.logger.LogMessage(client, target, "PRIVMSG", message)
	
	s.deliverMessage(client, target, "PRIVMSG", message)
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
	client.Send(fmt.Sprintf("PONG :%s", args))
}

func (s *Server) handleTopic(client *Client, args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 1 {
		client.Send(":server 461 TOPIC :Not enough parameters")
		return
	}

	channelName := parts[0]
	s.mu.RLock()
	channel, exists := s.channels[channelName]
	s.mu.RUnlock()

	if !exists {
		client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, channelName))
		return
	}

	// If no topic is provided, show the current topic
	if len(parts) == 1 {
		topic := channel.GetTopic()
		if topic == "" {
			client.Send(fmt.Sprintf(":server 331 %s %s :No topic is set", client.nick, channelName))
		} else {
			client.Send(fmt.Sprintf(":server 332 %s %s :%s", client.nick, channelName, topic))
		}
		return
	}

	// Set new topic
	newTopic := strings.TrimPrefix(parts[1], ":")
	channel.SetTopic(newTopic)
	
	// Broadcast the topic change to all channel members
	s.broadcastToChannel(channelName, fmt.Sprintf(":%s TOPIC %s :%s", client, channelName, newTopic))
	
	// Log the topic change
	s.logger.LogEvent(EventTopic, client, channelName, newTopic)
}

func (s *Server) handleWho(client *Client, args string) {
	target := strings.TrimSpace(args)
	if target == "" {
		client.Send(":server 461 WHO :Not enough parameters")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.HasPrefix(target, "#") {
		// WHO for channel
		channel, exists := s.channels[target]
		if !exists {
			client.Send(fmt.Sprintf(":server 403 %s %s :No such channel", client.nick, target))
			return
		}

		for _, member := range channel.GetClients() {
			// <channel> <username> <host> <server> <nick> <H|G>[*][@|+] :<hopcount> <real name>
			client.Send(fmt.Sprintf(":server 352 %s %s %s %s server %s H :0 %s",
				client.nick, target, member.username,
				member.conn.RemoteAddr().String(),
				member.nick, member.realname))
		}
	} else {
		// WHO for user
		if targetClient, exists := s.clients[target]; exists {
			client.Send(fmt.Sprintf(":server 352 %s * %s %s server %s H :0 %s",
				client.nick, targetClient.username,
				targetClient.conn.RemoteAddr().String(),
				targetClient.nick, targetClient.realname))
		}
	}

	client.Send(fmt.Sprintf(":server 315 %s %s :End of WHO list", client.nick, target))
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

func (s *Server) broadcastToChannel(channelName string, message string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	channel, exists := s.channels[channelName]
	if !exists {
		return
	}

	channel.mu.RLock()
	clients := make([]*Client, 0, len(channel.Clients))
	for _, client := range channel.Clients {
		clients = append(clients, client)
	}
	channel.mu.RUnlock()

	// Send messages after releasing locks to prevent deadlocks
	for _, client := range clients {
		client.Send(message)
	}
}

// SetWebServer sets the web server reference
func (s *Server) SetWebServer(ws *WebServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webServer = ws
}

// SetConfig sets the server configuration
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
		s.broadcastToChannel(channelName, fmt.Sprintf(":%s QUIT :Client exiting", client))
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
