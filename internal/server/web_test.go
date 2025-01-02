package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDashboardDataRetrieval(t *testing.T) {
	ircServer := New("localhost", "6667", nil, nil)
	webServer, err := NewWebServer(ircServer)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	req, err := http.NewRequest("GET", "/api/data", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webServer.handleAPIData)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var data DashboardData
	if err := json.NewDecoder(rr.Body).Decode(&data); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(data.Users) != 0 || len(data.Channels) != 0 || len(data.Messages) != 0 {
		t.Errorf("Expected empty dashboard data, got %+v", data)
	}
}

func TestMessageSendingViaWeb(t *testing.T) {
	ircServer := New("localhost", "6667", nil, nil)
	webServer, err := NewWebServer(ircServer)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	payload := `{"target": "#test", "content": "Hello, world!"}`
	req, err := http.NewRequest("POST", "/api/send", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webServer.handleAPISend)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	webServer.mu.RLock()
	defer webServer.mu.RUnlock()
	if len(webServer.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(webServer.messages))
	}
}

func TestConcurrentWebRequests(t *testing.T) {
	ircServer := New("localhost", "6667", nil, nil)
	webServer, err := NewWebServer(ircServer)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	var wg sync.WaitGroup
	numRequests := 10

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest("GET", "/api/data", nil)
			if err != nil {
				t.Errorf("Failed to create request: %v", err)
				return
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(webServer.handleAPIData)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}
		}()
	}

	wg.Wait()
}

func TestWebInterfaceAuthentication(t *testing.T) {
	ircServer := New("localhost", "6667", nil, nil)
	webServer, err := NewWebServer(ircServer)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	req, err := http.NewRequest("GET", "/api/data", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webServer.handleAPIData)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestWebsocketFunctionality(t *testing.T) {
	ircServer := New("localhost", "6667", nil, nil)
	webServer, err := NewWebServer(ircServer)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	// Simulate adding a message via websocket
	webServer.AddMessage("user1", "#test", "PRIVMSG", "Hello, world!")

	webServer.mu.RLock()
	defer webServer.mu.RUnlock()
	if len(webServer.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(webServer.messages))
	}
}
