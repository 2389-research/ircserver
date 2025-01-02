package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sync"
	"time"
)

type WebServer struct {
	ircServer *Server
	templates *template.Template
	messages  []MessageInfo
	mu        sync.RWMutex
}

type DashboardData struct {
	Users    []UserInfo    `json:"users"`
	Channels []ChannelInfo `json:"channels"`
	Messages []MessageInfo `json:"messages"`
}

type UserInfo struct {
	Nickname string   `json:"nickname"`
	Username string   `json:"username"`
	Channels []string `json:"channels"`
	LastSeen string   `json:"lastSeen"`
}

type ChannelInfo struct {
	Name      string   `json:"name"`
	Topic     string   `json:"topic"`
	UserCount int      `json:"userCount"`
	Users     []string `json:"users"`
}

type MessageInfo struct {
	Time    string `json:"time"`
	From    string `json:"from"`
	To      string `json:"to"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func NewWebServer(ircServer *Server) (*WebServer, error) {
	tmpl, err := template.ParseFiles("web/templates/dashboard.html")
	if err != nil {
		return nil, err
	}

	ws := &WebServer{
		ircServer: ircServer,
		templates: tmpl,
		messages:  make([]MessageInfo, 0),
	}

	ircServer.SetWebServer(ws)
	return ws, nil
}

func (ws *WebServer) Start(addr string) error {
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Register routes
	http.HandleFunc("/", ws.handleDashboard)
	http.HandleFunc("/api/data", ws.handleAPIData)
	http.HandleFunc("/api/send", ws.handleAPISend)

	return http.ListenAndServe(addr, nil)
}

func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if err := ws.templates.ExecuteTemplate(w, "dashboard.html", nil); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("ERROR: Failed to execute template: %v", err)
		return
	}
}

func (ws *WebServer) handleAPIData(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	data := ws.collectDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		log.Printf("ERROR: Failed to encode JSON: %v", err)
		return
	}
}

func (ws *WebServer) AddMessage(from, to, msgType, content string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	msg := MessageInfo{
		Time:    time.Now().Format(time.RFC3339),
		From:    from,
		To:      to,
		Type:    msgType,
		Content: content,
	}

	// Keep last 100 messages
	if len(ws.messages) >= 100 {
		ws.messages = ws.messages[1:]
	}
	ws.messages = append(ws.messages, msg)
}

func (ws *WebServer) handleAPISend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg struct {
		Target  string `json:"target"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create a virtual admin client for web messages
	adminClient := &Client{
		nick:     "WebAdmin",
		username: "webadmin",
		realname: "Web Interface Administrator",
	}

	ws.ircServer.deliverMessage(adminClient, msg.Target, "PRIVMSG", msg.Content)
	ws.AddMessage("WebAdmin", msg.Target, "PRIVMSG", msg.Content)

	w.WriteHeader(http.StatusOK)
}

// Shutdown gracefully shuts down the web server.
func (ws *WebServer) Shutdown() error {
	// TODO: Implement proper shutdown with context and timeout
	return nil
}

func (ws *WebServer) collectDashboardData() DashboardData {
	// Collect messages with minimal lock time
	ws.mu.RLock()
	messages := make([]MessageInfo, len(ws.messages))
	copy(messages, ws.messages)
	ws.mu.RUnlock()

	data := DashboardData{
		Users:    make([]UserInfo, 0),
		Channels: make([]ChannelInfo, 0),
		Messages: messages,
	}

	// Collect snapshot of server state
	ws.ircServer.mu.RLock()
	clientsCopy := make(map[string]*Client, len(ws.ircServer.clients))
	channelsCopy := make(map[string]*Channel, len(ws.ircServer.channels))

	for nick, client := range ws.ircServer.clients {
		clientsCopy[nick] = client
	}
	for name, channel := range ws.ircServer.channels {
		channelsCopy[name] = channel
	}
	ws.ircServer.mu.RUnlock()

	// Process clients without holding server lock
	for _, client := range clientsCopy {
		client.mu.Lock()
		channels := make([]string, 0, len(client.channels))
		for ch := range client.channels {
			channels = append(channels, ch)
		}

		data.Users = append(data.Users, UserInfo{
			Nickname: client.nick,
			Username: client.username,
			Channels: channels,
			LastSeen: time.Now().Format(time.RFC3339),
		})
		client.mu.Unlock()
	}

	// Process channels without holding server lock
	for name, channel := range channelsCopy {
		channel.mu.RLock()
		users := make([]string, 0, len(channel.Clients))
		for _, client := range channel.Clients {
			users = append(users, client.nick)
		}

		data.Channels = append(data.Channels, ChannelInfo{
			Name:      name,
			Topic:     channel.Topic,
			UserCount: len(users),
			Users:     users,
		})
		channel.mu.RUnlock()
	}

	return data
}
