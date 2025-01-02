package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func setupTestWebServer(t *testing.T) *WebServer {
	ircServer := New("localhost", "6667", nil, nil)
	ws := &WebServer{
		ircServer: ircServer,
		messages:  make([]MessageInfo, 0),
		mu:        sync.RWMutex{},
	}
	ircServer.SetWebServer(ws)
	return ws
}

func TestDashboardDataRetrieval(t *testing.T) {
	ws := setupTestWebServer(t)

	req, err := http.NewRequest("GET", "/api/data", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ws.handleAPIData)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
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
	ws := setupTestWebServer(t)

	msg := `{"target": "#test", "content": "Hello, world!"}`
	req, err := http.NewRequest("POST", "/api/send", strings.NewReader(msg))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ws.handleAPISend)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	ws.mu.RLock()
	defer ws.mu.RUnlock()
	// Give a small window for async operations to complete
	time.Sleep(100 * time.Millisecond)
	
	if len(ws.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(ws.messages))
		return
	}
	if ws.messages[0].Content != "Hello, world!" {
		t.Errorf("Expected message content 'Hello, world!', got '%s'", ws.messages[0].Content)
	}
	if ws.messages[0].To != "#test" {
		t.Errorf("Expected message target '#test', got '%s'", ws.messages[0].To)
	}
}

func TestConcurrentWebRequests(t *testing.T) {
	ws := setupTestWebServer(t)

	var wg sync.WaitGroup
	numRequests := 10

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest("GET", "/api/data", nil)
			if err != nil {
				t.Errorf("failed to create request: %v", err)
				return
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(ws.handleAPIData)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}
		}()
	}

	wg.Wait()
}

func TestWebInterfaceAuthentication(t *testing.T) {
	ws := setupTestWebServer(t)

	req, err := http.NewRequest("GET", "/api/data", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ws.handleAPIData)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Add authentication logic and test it here
	// For example, check if a specific header or token is present
}
