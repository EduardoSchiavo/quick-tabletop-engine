package main

import (
	"log"
    "sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/contrib/websocket"
)


var ( 
    clients = make(map[*websocket.Conn]bool)
    secretWord string
    mu sync.Mutex
)



func main() {
	app := fiber.New()

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
        //register connection
        mu.Lock()
        clients[c] = true


		// c.Locals is added to the *websocket.Conn
		log.Println(c.Locals("allowed"))  // true
		log.Println(c.Params("id"))       // 123
		log.Println(c.Query("v"))         // 1.0
		log.Println(c.Cookies("session")) // ""

        if secretWord != "" {
            c.WriteMessage(websocket.TextMessage, []byte(secretWord))
        }
        mu.Unlock()

        defer func() {
            c.Close()
			mu.Lock()
			delete(clients, c)
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
			secretWord = newWord
			for client := range clients {
				client.WriteMessage(websocket.TextMessage, []byte(secretWord))
			}
			mu.Unlock()
		}
		// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index

	}))

	log.Fatal(app.Listen(":3000"))
	// Access the websocket server: ws://localhost:3000/ws/123?v=1.0
	// https://www.websocket.org/echo.html
}

