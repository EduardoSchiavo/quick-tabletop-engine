package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	MaxSessions        int    `json:"maxSessions"`
	MaxUsersPerSession int    `json:"maxUsersPerSession"`
	SnapshotIntervalSec int   `json:"snapshotIntervalSec"`
	DatabaseURL        string `json:"databaseURL"`
}

func DefaultConfig() Config {
	return Config{
		MaxSessions:        5,
		MaxUsersPerSession: 10,
		SnapshotIntervalSec: 30,
		DatabaseURL:        "postgres://quicktt:quicktt@localhost:5432/quicktabletop?sslmode=disable",
	}
}

// Load reads a JSON config file at path. If the file is missing or invalid,
// it logs a warning and returns DefaultConfig(). Partial JSON is merged with defaults.
func Load(path string) Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("warning: could not read config file %q: %v — using defaults", path, err)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("warning: invalid JSON in config file %q: %v — using defaults", path, err)
		return DefaultConfig()
	}

	return cfg
}
