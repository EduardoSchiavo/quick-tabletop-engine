package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store manages PostgreSQL persistence for session snapshots.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to the database and creates the session_snapshots table if it does not exist.
func New(connStr string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS session_snapshots (
			session_id TEXT PRIMARY KEY,
			state_json JSONB NOT NULL,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
	`
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &Store{pool: pool}, nil
}

// SaveSnapshot upserts the session state as a JSON snapshot.
func (s *Store) SaveSnapshot(sessionID string, stateJSON []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO session_snapshots (session_id, state_json, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (session_id) DO UPDATE
		SET state_json = EXCLUDED.state_json, updated_at = NOW();
	`
	_, err := s.pool.Exec(ctx, query, sessionID, stateJSON)
	return err
}

// LoadAllSnapshots returns all stored session snapshots as a map of sessionID to JSON bytes.
func (s *Store) LoadAllSnapshots() (map[string][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx, "SELECT session_id, state_json FROM session_snapshots")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snapshots := make(map[string][]byte)
	for rows.Next() {
		var sessionID string
		var stateJSON []byte
		if err := rows.Scan(&sessionID, &stateJSON); err != nil {
			return nil, err
		}
		snapshots[sessionID] = stateJSON
	}

	return snapshots, rows.Err()
}

// DeleteSnapshot removes the snapshot for a given session.
func (s *Store) DeleteSnapshot(sessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.pool.Exec(ctx, "DELETE FROM session_snapshots WHERE session_id = $1", sessionID)
	return err
}

// Close shuts down the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}
