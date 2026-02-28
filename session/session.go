package session

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"quick-tabletop-engine/config"
	"quick-tabletop-engine/game"
	"quick-tabletop-engine/store"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ServerMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Session struct {
	ID      string
	Clients map[*websocket.Conn]bool
	State   game.State
}

type Manager struct {
	sessions         map[string]*Session
	mu               sync.Mutex
	cfg              config.Config
	store            *store.Store
	snapshotInterval time.Duration
	stopSnapshot     chan struct{}
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		cfg:      cfg,
	}
}

func (m *Manager) Reset() {
	m.sessions = make(map[string]*Session)
}

// SetStore configures the persistence store and snapshot interval.
func (m *Manager) SetStore(s *store.Store, interval time.Duration) {
	m.store = s
	m.snapshotInterval = interval
}

// RestoreSessions loads snapshots from the store and creates sessions with restored state.
func (m *Manager) RestoreSessions() {
	if m.store == nil {
		return
	}

	snapshots, err := m.store.LoadAllSnapshots()
	if err != nil {
		log.Printf("warning: failed to load snapshots: %v", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for sessionID, stateJSON := range snapshots {
		var state game.State
		if err := json.Unmarshal(stateJSON, &state); err != nil {
			log.Printf("warning: failed to unmarshal state for session %s: %v", sessionID, err)
			continue
		}
		// Ensure maps are initialized
		if state.DisplayedTokens == nil {
			state.DisplayedTokens = make(map[string]game.TokenData)
		}
		if state.AreaTemplates == nil {
			state.AreaTemplates = make(map[string]game.AreaTemplate)
		}
		m.sessions[sessionID] = &Session{
			ID:      sessionID,
			Clients: make(map[*websocket.Conn]bool),
			State:   state,
		}
		log.Printf("restored session %s from snapshot", sessionID)
	}
}

// StartPeriodicSnapshots starts a goroutine that periodically saves all session states.
func (m *Manager) StartPeriodicSnapshots() {
	if m.store == nil {
		return
	}

	m.stopSnapshot = make(chan struct{})
	ticker := time.NewTicker(m.snapshotInterval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.saveAllSnapshots()
			case <-m.stopSnapshot:
				return
			}
		}
	}()
}

// StopPeriodicSnapshots stops the periodic snapshot goroutine and does one final save.
func (m *Manager) StopPeriodicSnapshots() {
	if m.store == nil || m.stopSnapshot == nil {
		return
	}

	close(m.stopSnapshot)
	m.saveAllSnapshots()
}

// saveAllSnapshots saves the state of every session to the store.
func (m *Manager) saveAllSnapshots() {
	m.mu.Lock()
	// Copy session IDs and states while holding the lock
	type snapshot struct {
		id    string
		state game.State
	}
	snaps := make([]snapshot, 0, len(m.sessions))
	for id, sess := range m.sessions {
		snaps = append(snaps, snapshot{id: id, state: sess.State})
	}
	m.mu.Unlock()

	for _, snap := range snaps {
		data, err := json.Marshal(snap.state)
		if err != nil {
			log.Printf("warning: failed to marshal state for session %s: %v", snap.id, err)
			continue
		}
		if err := m.store.SaveSnapshot(snap.id, data); err != nil {
			log.Printf("warning: failed to save snapshot for session %s: %v", snap.id, err)
		}
	}
}

func (m *Manager) CreateSession(c *fiber.Ctx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.cfg.MaxSessions {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "maximum number of sessions reached",
		})
	}

	id := uuid.NewString()

	m.sessions[id] = &Session{
		ID:      id,
		Clients: make(map[*websocket.Conn]bool),
		State:   game.NewState(),
	}

	log.Println("session created:", id)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"sessionId": id,
	})
}

func (m *Manager) GetSession(c *fiber.Ctx) error {
	id := c.Params("id")
	m.mu.Lock()
	_, ok := m.sessions[id]
	m.mu.Unlock()

	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "session not found",
		})
	}

	return c.JSON(fiber.Map{
		"sessionId": id,
	})
}

// WS handler
func (m *Manager) HandleWS(c *websocket.Conn) {
	sessionId := c.Params("sessionId")
	m.mu.Lock()
	session, ok := m.sessions[sessionId]
	if !ok {
		m.mu.Unlock()
		c.Close()
		return
	}

	// Check if session is full
	if len(session.Clients) >= m.cfg.MaxUsersPerSession {
		m.mu.Unlock()
		errMsg := ServerMessage{
			Type: "error",
			Payload: map[string]string{
				"error": "session is full",
			},
		}
		data, _ := json.Marshal(errMsg)
		c.WriteMessage(websocket.TextMessage, data)
		c.Close()
		return
	}

	session.Clients[c] = true
	log.Printf("client joined session %s (%d connected)\n", sessionId, len(session.Clients))

	// Send current state to the new client (late-joiner sync)
	sendState(c, session.State)
	m.mu.Unlock()

	defer func() {
		c.Close()
		m.mu.Lock()
		delete(session.Clients, c)
		remaining := len(session.Clients)
		m.mu.Unlock()

		if remaining == 0 {
			time.AfterFunc(2*time.Second, func() {
				m.mu.Lock()
				defer m.mu.Unlock()
				s, ok := m.sessions[sessionId]
				if ok && len(s.Clients) == 0 {
					delete(m.sessions, sessionId)
					log.Printf("session %s removed (no clients remaining)\n", sessionId)
				}
			})
		}
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message:", err)
			continue
		}

		m.mu.Lock()
		processCommand(clientMsg, &session.State)
		broadcastState(session)
		m.mu.Unlock()
	}
}

func processCommand(msg ClientMessage, state *game.State) {
	switch msg.Type {
	case "add_token":
		var p game.AddTokenPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.AddToken(p.ID, p.Token)
		}
	case "move_token":
		var p game.MoveTokenPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.MoveToken(p.ID, p.X, p.Y)
		}
	case "delete_token":
		var p game.DeleteTokenPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.DeleteToken(p.ID)
		}
	case "clear_tokens":
		state.ClearTokens()
	case "change_background":
		var p game.ChangeBackgroundPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.ChangeBackgroundImg(p.ImgPath)
		}
	case "toggle_grid":
		state.ToggleGrid()
	case "add_area_template":
		var p game.AddAreaTemplatePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			id := p.ID
			if id == "" {
				id = uuid.NewString()
			}
			state.AddAreaTemplate(id, p.Template)
		}
	case "move_area_template":
		var p game.MoveAreaTemplatePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.MoveAreaTemplate(p.ID, p.X, p.Y)
		}
	case "delete_area_template":
		var p game.DeleteAreaTemplatePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			state.DeleteAreaTemplate(p.ID)
		}
	case "clear_area_templates":
		state.ClearAreaTemplates()
	default:
		log.Println("unknown message type:", msg.Type)
	}
}

func broadcastState(session *Session) {
	msg := ServerMessage{
		Type:    "state_update",
		Payload: session.State,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("failed to marshal state:", err)
		return
	}
	for client := range session.Clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func sendState(c *websocket.Conn, state game.State) {
	msg := ServerMessage{
		Type:    "state_update",
		Payload: state,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("failed to marshal state:", err)
		return
	}
	c.WriteMessage(websocket.TextMessage, data)
}
