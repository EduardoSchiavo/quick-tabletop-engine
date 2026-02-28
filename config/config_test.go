package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxSessions != 5 {
		t.Errorf("expected MaxSessions 5, got %d", cfg.MaxSessions)
	}
	if cfg.MaxUsersPerSession != 10 {
		t.Errorf("expected MaxUsersPerSession 10, got %d", cfg.MaxUsersPerSession)
	}
	if cfg.SnapshotIntervalSec != 30 {
		t.Errorf("expected SnapshotIntervalSec 30, got %d", cfg.SnapshotIntervalSec)
	}
	if cfg.DatabaseURL != "postgres://quicktt:quicktt@localhost:5432/quicktabletop?sslmode=disable" {
		t.Errorf("unexpected DatabaseURL: %q", cfg.DatabaseURL)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{
		"maxSessions": 20,
		"maxUsersPerSession": 50,
		"snapshotIntervalSec": 60,
		"databaseURL": "postgres://user:pass@host:5432/db"
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load(path)

	if cfg.MaxSessions != 20 {
		t.Errorf("expected MaxSessions 20, got %d", cfg.MaxSessions)
	}
	if cfg.MaxUsersPerSession != 50 {
		t.Errorf("expected MaxUsersPerSession 50, got %d", cfg.MaxUsersPerSession)
	}
	if cfg.SnapshotIntervalSec != 60 {
		t.Errorf("expected SnapshotIntervalSec 60, got %d", cfg.SnapshotIntervalSec)
	}
	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Errorf("unexpected DatabaseURL: %q", cfg.DatabaseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg := Load("/nonexistent/path/config.json")
	defaults := DefaultConfig()

	if cfg != defaults {
		t.Errorf("expected defaults on missing file, got %+v", cfg)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("not valid json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load(path)
	defaults := DefaultConfig()

	if cfg != defaults {
		t.Errorf("expected defaults on invalid JSON, got %+v", cfg)
	}
}

func TestLoadPartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Only override maxSessions; everything else should keep defaults
	data := `{"maxSessions": 42}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load(path)

	if cfg.MaxSessions != 42 {
		t.Errorf("expected MaxSessions 42, got %d", cfg.MaxSessions)
	}
	// Remaining fields should be defaults
	if cfg.MaxUsersPerSession != 10 {
		t.Errorf("expected default MaxUsersPerSession 10, got %d", cfg.MaxUsersPerSession)
	}
	if cfg.SnapshotIntervalSec != 30 {
		t.Errorf("expected default SnapshotIntervalSec 30, got %d", cfg.SnapshotIntervalSec)
	}
	if cfg.DatabaseURL != "postgres://quicktt:quicktt@localhost:5432/quicktabletop?sslmode=disable" {
		t.Errorf("expected default DatabaseURL, got %q", cfg.DatabaseURL)
	}
}
