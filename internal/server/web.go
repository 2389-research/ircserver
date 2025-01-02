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
	mu        sync.RWMutex
}

type DashboardData struct {
	Users    []UserInfo    `json:"users"`
	Channels []ChannelInfo `json:"channels"`
	Messages []MessageInfo `json:"messages"`
}

type UserInfo struct {
	Nickname  string   `json:"nickname"`
	Username  string   `json:"username"`
	Channels  []string `json:"channels"`
	LastSeen  string   `json:"lastSeen"`
}

type ChannelInfo struct {
	Name     string   `json:"name"`
	Topic    string   `json:"topic"`
	UserCount int     `json:"userCount"`
	Users    []string `json:"users"`
}

type MessageInfo struct {
	Time     string `json:"time"`
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Content  string `json:"content"`
}

func NewWebServer(ircServer *Server) (*WebServer, error) {
	tmpl, err := template.ParseFiles("web/templates/dashboard.html")
	if err != nil {
		return nil, err
	}

	return &WebServer{
		ircServer: ircServer,
		templates: tmpl,
	}, nil
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
	ws.templates.ExecuteTemplate(w, "dashboard.html", nil)
}

func (ws *WebServer) handleAPIData(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	data := ws.collectDashboardData()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
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
		nick: "WebAdmin",
		username: "webadmin",
		realname: "Web Interface Administrator",
	}

	ws.ircServer.deliverMessage(adminClient, msg.Target, "PRIVMSG", msg.Content)
	
	w.WriteHeader(http.StatusOK)
}

func (ws *WebServer) collectDashboardData() DashboardData {
	data := DashboardData{
		Users:    make([]UserInfo, 0),
		Channels: make([]ChannelInfo, 0),
	}

	// Collect users
	for _, client := range ws.ircServer.clients {
		channels := make([]string, 0)
		for ch := range client.channels {
			channels = append(channels, ch)
		}
		
		data.Users = append(data.Users, UserInfo{
			Nickname: client.nick,
			Username: client.username,
			Channels: channels,
			LastSeen: time.Now().Format(time.RFC3339),
		})
	}

	// Collect channels
	for name, channel := range ws.ircServer.channels {
		users := make([]string, 0)
		for _, client := range channel.GetClients() {
			users = append(users, client.nick)
		}
		
		data.Channels = append(data.Channels, ChannelInfo{
			Name:     name,
			Topic:    channel.Topic,
			UserCount: len(users),
			Users:    users,
		})
	}

	return data
}
