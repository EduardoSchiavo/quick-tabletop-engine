package main

import (
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"quick-tabletop-engine/session"
)

// hardcoded 5 session limit
var sessionManager = session.NewManager(5)

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

	app.Post("/session", sessionManager.CreateSession)
	app.Get("/session/:id", sessionManager.GetSession)

	app.Get("/ws/:sessionId", websocket.New(sessionManager.HandleWS))

	return app
}

func main() {
	app := setupApp()
	log.Fatal(app.Listen(":3000"))
}

