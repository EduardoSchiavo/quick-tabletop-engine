package main

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
)

type TokenData struct {
	Name      string  `json:"name"`
	ImgPath   string  `json:"imgPath"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	TokenSize float64 `json:"tokenSize"`
}

type GameState struct {
	DisplayedTokens  map[string]TokenData `json:"displayedTokens"`
	BackgroundImgPath string              `json:"backgroundImgPath"`
	ShowGrid          bool                `json:"showGrid"`
	GridUnit          float64             `json:"gridUnit"`
}

type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ServerMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type AddTokenPayload struct {
	ID    string    `json:"id"`
	Token TokenData `json:"token"`
}

type MoveTokenPayload struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type DeleteTokenPayload struct {
	ID string `json:"id"`
}

type ChangeBackgroundPayload struct {
	ImgPath string `json:"imgPath"`
}

type Session struct {
	ID      string
	Clients map[*websocket.Conn]bool
	State   GameState
}

func newGameState() GameState {
	return GameState{
		DisplayedTokens:   make(map[string]TokenData),
		BackgroundImgPath: "/assets/default/maps/tavern.jpg",
		ShowGrid:          true,
		GridUnit:          96,
	}
}

var (
	sessions = map[string]*Session{}
	mu       sync.Mutex
)

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

func sendState(c *websocket.Conn, state GameState) {
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

func setupApp() *fiber.App {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,OPTIONS",
		AllowHeaders: "Content-Type",
	}))

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/session", createSession)
	app.Get("/session/:id", getSession)

	app.Get("/ws/:sessionId", websocket.New(func(c *websocket.Conn) {
		sessionId := c.Params("sessionId")
		mu.Lock()
		session, ok := sessions[sessionId]
		if !ok {
			mu.Unlock()
			c.Close()
			return
		}

		session.Clients[c] = true
		log.Printf("client joined session %s (%d connected)\n", sessionId, len(session.Clients))

		// Send current state to the new client (late-joiner sync)
		sendState(c, session.State)
		mu.Unlock()

		defer func() {
			c.Close()
			mu.Lock()
			delete(session.Clients, c)
			mu.Unlock()
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

			mu.Lock()
			switch clientMsg.Type {
			case "add_token":
				var p AddTokenPayload
				if err := json.Unmarshal(clientMsg.Payload, &p); err == nil {
					session.State.DisplayedTokens[p.ID] = p.Token
				}
			case "move_token":
				var p MoveTokenPayload
				if err := json.Unmarshal(clientMsg.Payload, &p); err == nil {
					if token, ok := session.State.DisplayedTokens[p.ID]; ok {
						token.X = p.X
						token.Y = p.Y
						session.State.DisplayedTokens[p.ID] = token
					}
				}
			case "delete_token":
				var p DeleteTokenPayload
				if err := json.Unmarshal(clientMsg.Payload, &p); err == nil {
					delete(session.State.DisplayedTokens, p.ID)
				}
			case "clear_tokens":
				session.State.DisplayedTokens = make(map[string]TokenData)
			case "change_background":
				var p ChangeBackgroundPayload
				if err := json.Unmarshal(clientMsg.Payload, &p); err == nil {
					session.State.BackgroundImgPath = p.ImgPath
				}
			case "toggle_grid":
				session.State.ShowGrid = !session.State.ShowGrid
			default:
				log.Println("unknown message type:", clientMsg.Type)
			}
			broadcastState(session)
			mu.Unlock()
		}
	}))

	return app
}

func main() {
	app := setupApp()
	log.Fatal(app.Listen(":3000"))
}

func getSession(c *fiber.Ctx) error {
	id := c.Params("id")
	mu.Lock()
	_, ok := sessions[id]
	mu.Unlock()

	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "session not found",
		})
	}

	return c.JSON(fiber.Map{
		"sessionId": id,
	})
}

const maxSessions = 100

func createSession(c *fiber.Ctx) error {
	mu.Lock()
	defer mu.Unlock()

	if len(sessions) >= maxSessions {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "maximum number of sessions reached",
		})
	}

	id := uuid.NewString()

	sessions[id] = &Session{
		ID:      id,
		Clients: make(map[*websocket.Conn]bool),
		State:   newGameState(),
	}

	log.Println("session created:", id)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"sessionId": id,
	})
}
