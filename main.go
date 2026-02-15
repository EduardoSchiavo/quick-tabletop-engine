package main

import (
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
)

// TODO: move elsewhere
type Session struct {
	ID         string
	Clients    map[*websocket.Conn]bool
	SecretWord string
}

var (
	//    clients = make(map[*websocket.Conn]bool)
	//    secretWord string

	sessions = map[string]*Session{}
	mu       sync.Mutex
)

func setupApp() *fiber.App {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,OPTIONS",
		AllowHeaders: "Content-Type",
	}))

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
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

		if session.SecretWord != "" {
			c.WriteMessage(websocket.TextMessage, []byte(session.SecretWord))
		}
		mu.Unlock()

		defer func() {
			c.Close()
			mu.Lock()
			delete(session.Clients, c)
			//cleanup empty sessions
			if len(session.Clients) == 0 {
				delete(sessions, sessionId)
				log.Println("session closed since no clients were active:", sessionId)
			}
			mu.Unlock()
		}()

		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}

			newWord := string(msg)
			log.Println("new secret word:", newWord)

			// Update state + broadcast
			mu.Lock()
			session.SecretWord = newWord
			for client := range session.Clients {
				client.WriteMessage(websocket.TextMessage, []byte(session.SecretWord))
			}
			mu.Unlock()
		}
		// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index

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

// TODO: move elsewhere
const maxSessions = 5

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
	}

	log.Println("session created:", id)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"sessionId": id,
	})
}
