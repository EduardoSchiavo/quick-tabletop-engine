package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startTestServer spins up the Fiber app on a random port and returns the base URL.
// It also resets global state so tests are isolated.
func startTestServer(t *testing.T) string {
	t.Helper()

	// Reset global state between tests.
	mu.Lock()
	sessions = map[string]*Session{}
	mu.Unlock()

	app := setupApp()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = app.Listener(ln)
	}()

	t.Cleanup(func() {
		_ = app.Shutdown()
	})

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}

// createTestSession calls POST /session and returns the sessionId.
func createTestSession(t *testing.T, addr string) string {
	t.Helper()

	resp, err := http.Post(fmt.Sprintf("http://%s/session", addr), "application/json", nil)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	sessionId, ok := body["sessionId"]
	if !ok || sessionId == "" {
		t.Fatal("response missing sessionId")
	}
	return sessionId
}

// connectWS dials the WebSocket endpoint for a given session and returns the connection.
func connectWS(t *testing.T, addr, sessionId string) *websocket.Conn {
	t.Helper()

	url := fmt.Sprintf("ws://%s/ws/%s", addr, sessionId)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("failed to connect to ws: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// readWithTimeout reads a message with a deadline so tests don't hang.
func readWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	return string(msg)
}

func TestCreateSessionAndJoin(t *testing.T) {
	addr := startTestServer(t)

	// Create a session via the HTTP API.
	sessionId := createTestSession(t, addr)

	// Join the session over WebSocket.
	conn := connectWS(t, addr, sessionId)

	// Send a message and expect to receive it back (echo to self).
	expected := "hello"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(expected)); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	got := readWithTimeout(t, conn, 2*time.Second)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestJoinNonExistentSession(t *testing.T) {
	addr := startTestServer(t)

	url := fmt.Sprintf("ws://%s/ws/does-not-exist", addr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		// Connection refused or upgrade failed — both acceptable.
		return
	}
	defer conn.Close()

	// If the connection was accepted, the server should close it immediately.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection to be closed for non-existent session")
	}
}

func TestBroadcastWithinSession(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	// Connect three clients to the same session.
	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := range conns {
		conns[i] = connectWS(t, addr, sessionId)
	}

	// Client 0 sends a secret word.
	secret := "tabletop"
	if err := conns[0].WriteMessage(websocket.TextMessage, []byte(secret)); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// All clients (including the sender) should receive the broadcast.
	var wg sync.WaitGroup
	for i, conn := range conns {
		wg.Add(1)
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			got := readWithTimeout(t, c, 2*time.Second)
			if got != secret {
				t.Errorf("client %d: expected %q, got %q", idx, secret, got)
			}
		}(i, conn)
	}
	wg.Wait()
}

func TestBroadcastIsolationBetweenSessions(t *testing.T) {
	addr := startTestServer(t)

	// Create two independent sessions.
	session1 := createTestSession(t, addr)
	session2 := createTestSession(t, addr)

	conn1 := connectWS(t, addr, session1)
	conn2 := connectWS(t, addr, session2)

	// Send a message in session 1.
	if err := conn1.WriteMessage(websocket.TextMessage, []byte("only-for-session1")); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Client in session 1 should receive it.
	got := readWithTimeout(t, conn1, 2*time.Second)
	if got != "only-for-session1" {
		t.Errorf("session1 client: expected %q, got %q", "only-for-session1", got)
	}

	// Client in session 2 should NOT receive anything.
	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn2.ReadMessage()
	if err == nil {
		t.Error("session2 client should not have received a message from session1")
	}
}

func TestNewClientReceivesCurrentSecretWord(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	// First client sets a secret word.
	conn1 := connectWS(t, addr, sessionId)
	if err := conn1.WriteMessage(websocket.TextMessage, []byte("existing-secret")); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	// Read the broadcast back so we know the server has processed it.
	readWithTimeout(t, conn1, 2*time.Second)

	// A second client joins after the secret word was set.
	conn2 := connectWS(t, addr, sessionId)

	// The new client should immediately receive the current secret word.
	got := readWithTimeout(t, conn2, 2*time.Second)
	if got != "existing-secret" {
		t.Errorf("late joiner: expected %q, got %q", "existing-secret", got)
	}
}
