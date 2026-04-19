package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"quick-tabletop-engine/config"
	"quick-tabletop-engine/session"
	"quick-tabletop-engine/store"
)

var cfg = config.Load("config.json")
var sessionManager = session.NewManager(cfg)

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
	// Allow DATABASE_URL env var to override config file
	dbURL := cfg.DatabaseURL
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		dbURL = envURL
	}

	if dbURL != "" {
		s, err := store.New(dbURL)
		if err != nil {
			log.Printf("warning: failed to connect to database: %v — running without persistence", err)
		} else {
			interval := time.Duration(cfg.SnapshotIntervalSec) * time.Second
			sessionManager.SetStore(s, interval)
			sessionManager.RestoreSessions()
			sessionManager.StartPeriodicSnapshots()
			defer sessionManager.StopPeriodicSnapshots()
			defer s.Close()
		}
	}

	app := setupApp()
	log.Fatal(app.Listen(":3000"))
}
