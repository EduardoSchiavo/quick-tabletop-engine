package session

import (
	"encoding/json"
	"log"
	"sync"

	"quick-tabletop-engine/game"

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
	sessions  map[string]*Session
	mu       sync.Mutex
	maxSessions int
}

func NewManager(maxSessions int) *Manager{
	return &Manager{
		sessions: make(map[string]*Session),
		maxSessions: maxSessions,
	}
}

func (m *Manager) Reset(){
	m.sessions = make(map[string]*Session)}


func (m *Manager) CreateSession(c *fiber.Ctx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxSessions {
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


//WS handler
func (m *Manager) HandleWS(c *websocket.Conn){
		sessionId := c.Params("sessionId")
		m.mu.Lock()
		session, ok := m.sessions[sessionId]
		if !ok {
			m.mu.Unlock()
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
			m.mu.Unlock()
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
