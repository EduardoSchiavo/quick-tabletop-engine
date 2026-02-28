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

	"quick-tabletop-engine/config"
	"quick-tabletop-engine/game"
	"quick-tabletop-engine/session"
)

func testConfig() config.Config {
	return config.Config{
		MaxSessions:        10,
		MaxUsersPerSession: 3,
		SnapshotIntervalSec: 30,
		DatabaseURL:        "",
	}
}

// startTestServer spins up the Fiber app on a random port and returns the base URL.
// It also resets global state so tests are isolated.
func startTestServer(t *testing.T) string {
	t.Helper()

	// Reset global state between tests.
	cfg = testConfig()
	sessionManager = session.NewManager(cfg)

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

// readStateUpdate reads a message, parses it as a ServerMessage with state_update type,
// and returns the game.State payload.
func readStateUpdate(t *testing.T, conn *websocket.Conn, timeout time.Duration) game.State {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var serverMsg session.ServerMessage
	if err := json.Unmarshal(msg, &serverMsg); err != nil {
		t.Fatalf("failed to unmarshal server message: %v", err)
	}
	if serverMsg.Type != "state_update" {
		t.Fatalf("expected type state_update, got %s", serverMsg.Type)
	}

	// Re-marshal payload and unmarshal into game.State
	payloadBytes, err := json.Marshal(serverMsg.Payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	var state game.State
	if err := json.Unmarshal(payloadBytes, &state); err != nil {
		t.Fatalf("failed to unmarshal game state: %v", err)
	}
	return state
}

// readServerMessage reads a raw ServerMessage from the WebSocket.
func readServerMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) session.ServerMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var serverMsg session.ServerMessage
	if err := json.Unmarshal(msg, &serverMsg); err != nil {
		t.Fatalf("failed to unmarshal server message: %v", err)
	}
	return serverMsg
}

// sendCommand sends a JSON command over the WebSocket.
func sendCommand(t *testing.T, conn *websocket.Conn, msgType string, payload interface{}) {
	t.Helper()
	var raw json.RawMessage
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
	}
	msg := session.ClientMessage{Type: msgType, Payload: raw}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal command: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}
}

func TestCreateSessionAndJoin(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)
	conn := connectWS(t, addr, sessionId)

	// Should receive initial state on connect
	state := readStateUpdate(t, conn, 2*time.Second)
	if state.BackgroundImgPath != "/assets/default/maps/tavern.jpg" {
		t.Errorf("expected tavern background, got %q", state.BackgroundImgPath)
	}
	if !state.ShowGrid {
		t.Error("expected showGrid to be true")
	}
	if len(state.DisplayedTokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(state.DisplayedTokens))
	}

	// Send add_token command and verify state_update
	sendCommand(t, conn, "add_token", game.AddTokenPayload{
		ID: "token-1",
		Token: game.TokenData{
			Name:      "Goblin",
			ImgPath:   "/goblin.jpg",
			X:         96,
			Y:         96,
			TokenSize: 96,
		},
	})

	state = readStateUpdate(t, conn, 2*time.Second)
	if len(state.DisplayedTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(state.DisplayedTokens))
	}
	token, ok := state.DisplayedTokens["token-1"]
	if !ok {
		t.Fatal("token-1 not found in state")
	}
	if token.Name != "Goblin" {
		t.Errorf("expected token name Goblin, got %q", token.Name)
	}
}

func TestJoinNonExistentSession(t *testing.T) {
	addr := startTestServer(t)

	url := fmt.Sprintf("ws://%s/ws/does-not-exist", addr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection to be closed for non-existent session")
	}
}

func TestBroadcastWithinSession(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := range conns {
		conns[i] = connectWS(t, addr, sessionId)
		// Drain initial state_update
		readStateUpdate(t, conns[i], 2*time.Second)
	}

	// Client 0 sends an add_token command
	sendCommand(t, conns[0], "add_token", game.AddTokenPayload{
		ID:    "broadcast-token",
		Token: game.TokenData{Name: "Orc", ImgPath: "/orc.jpg", X: 100, Y: 200, TokenSize: 96},
	})

	// All clients should receive the state_update
	var wg sync.WaitGroup
	for i, conn := range conns {
		wg.Add(1)
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			state := readStateUpdate(t, c, 2*time.Second)
			if len(state.DisplayedTokens) != 1 {
				t.Errorf("client %d: expected 1 token, got %d", idx, len(state.DisplayedTokens))
			}
		}(i, conn)
	}
	wg.Wait()
}

func TestBroadcastIsolationBetweenSessions(t *testing.T) {
	addr := startTestServer(t)

	session1 := createTestSession(t, addr)
	session2 := createTestSession(t, addr)

	conn1 := connectWS(t, addr, session1)
	conn2 := connectWS(t, addr, session2)

	// Drain initial states
	readStateUpdate(t, conn1, 2*time.Second)
	readStateUpdate(t, conn2, 2*time.Second)

	// Send a command in session 1
	sendCommand(t, conn1, "add_token", game.AddTokenPayload{
		ID:    "s1-token",
		Token: game.TokenData{Name: "Elf", ImgPath: "/elf.jpg", X: 50, Y: 50, TokenSize: 96},
	})

	// Client in session 1 should receive state_update
	state := readStateUpdate(t, conn1, 2*time.Second)
	if len(state.DisplayedTokens) != 1 {
		t.Errorf("session1: expected 1 token, got %d", len(state.DisplayedTokens))
	}

	// Client in session 2 should NOT receive anything
	conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn2.ReadMessage()
	if err == nil {
		t.Error("session2 client should not have received a message from session1")
	}
}

func TestNewClientReceivesCurrentState(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	// First client adds a token
	conn1 := connectWS(t, addr, sessionId)
	readStateUpdate(t, conn1, 2*time.Second) // drain initial state

	sendCommand(t, conn1, "add_token", game.AddTokenPayload{
		ID:    "existing-token",
		Token: game.TokenData{Name: "Dragon", ImgPath: "/dragon.jpg", X: 200, Y: 300, TokenSize: 96},
	})
	readStateUpdate(t, conn1, 2*time.Second) // drain broadcast

	// Second client joins and should immediately receive current state with the token
	conn2 := connectWS(t, addr, sessionId)
	state := readStateUpdate(t, conn2, 2*time.Second)

	if len(state.DisplayedTokens) != 1 {
		t.Fatalf("late joiner: expected 1 token, got %d", len(state.DisplayedTokens))
	}
	token, ok := state.DisplayedTokens["existing-token"]
	if !ok {
		t.Fatal("existing-token not found")
	}
	if token.Name != "Dragon" {
		t.Errorf("expected Dragon, got %q", token.Name)
	}
}

func TestToggleGrid(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	conn := connectWS(t, addr, sessionId)
	state := readStateUpdate(t, conn, 2*time.Second)

	if !state.ShowGrid {
		t.Fatal("expected showGrid to be true initially")
	}

	// Toggle off
	sendCommand(t, conn, "toggle_grid", nil)
	state = readStateUpdate(t, conn, 2*time.Second)
	if state.ShowGrid {
		t.Error("expected showGrid to be false after toggle")
	}

	// Toggle back on
	sendCommand(t, conn, "toggle_grid", nil)
	state = readStateUpdate(t, conn, 2*time.Second)
	if !state.ShowGrid {
		t.Error("expected showGrid to be true after second toggle")
	}
}

func TestSessionUserLimit(t *testing.T) {
	addr := startTestServer(t)
	sessionId := createTestSession(t, addr)

	// testConfig sets MaxUsersPerSession to 3
	// Connect 3 clients (should all succeed)
	conns := make([]*websocket.Conn, 3)
	for i := range conns {
		conns[i] = connectWS(t, addr, sessionId)
		readStateUpdate(t, conns[i], 2*time.Second) // drain initial state
	}

	// 4th client should get an error message and be disconnected
	url := fmt.Sprintf("ws://%s/ws/%s", addr, sessionId)
	extraConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("expected to connect (server sends error then closes): %v", err)
	}
	defer extraConn.Close()

	// Read the error message
	serverMsg := readServerMessage(t, extraConn, 2*time.Second)
	if serverMsg.Type != "error" {
		t.Fatalf("expected error message type, got %q", serverMsg.Type)
	}

	// Verify the error payload
	payloadBytes, _ := json.Marshal(serverMsg.Payload)
	var errPayload map[string]string
	json.Unmarshal(payloadBytes, &errPayload)
	if errPayload["error"] != "session is full" {
		t.Errorf("expected 'session is full' error, got %q", errPayload["error"])
	}

	// Connection should be closed by server — next read should fail
	extraConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = extraConn.ReadMessage()
	if err == nil {
		t.Error("expected connection to be closed after error")
	}
}
