package store

import (
	"os"
	"testing"
)

// getTestDBURL returns the database URL for testing.
// Tests are skipped if no database is available.
func getTestDBURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping database tests")
	}
	return url
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	dbURL := getTestDBURL(t)

	s, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	stateJSON := []byte(`{"displayedTokens":{},"backgroundImgPath":"/tavern.jpg","showGrid":true,"gridUnit":96}`)

	if err := s.SaveSnapshot("session-1", stateJSON); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	snapshots, err := s.LoadAllSnapshots()
	if err != nil {
		t.Fatalf("failed to load snapshots: %v", err)
	}

	data, ok := snapshots["session-1"]
	if !ok {
		t.Fatal("session-1 not found in snapshots")
	}
	if string(data) != string(stateJSON) {
		t.Errorf("expected %s, got %s", stateJSON, data)
	}
}

func TestOverwriteSnapshot(t *testing.T) {
	dbURL := getTestDBURL(t)

	s, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	original := []byte(`{"showGrid":true}`)
	updated := []byte(`{"showGrid":false}`)

	if err := s.SaveSnapshot("session-overwrite", original); err != nil {
		t.Fatalf("failed to save original: %v", err)
	}
	if err := s.SaveSnapshot("session-overwrite", updated); err != nil {
		t.Fatalf("failed to save updated: %v", err)
	}

	snapshots, err := s.LoadAllSnapshots()
	if err != nil {
		t.Fatalf("failed to load snapshots: %v", err)
	}

	data := snapshots["session-overwrite"]
	if string(data) != string(updated) {
		t.Errorf("expected updated snapshot %s, got %s", updated, data)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	dbURL := getTestDBURL(t)

	s, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	stateJSON := []byte(`{"showGrid":true}`)
	if err := s.SaveSnapshot("session-delete", stateJSON); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	if err := s.DeleteSnapshot("session-delete"); err != nil {
		t.Fatalf("failed to delete snapshot: %v", err)
	}

	snapshots, err := s.LoadAllSnapshots()
	if err != nil {
		t.Fatalf("failed to load snapshots: %v", err)
	}

	if _, ok := snapshots["session-delete"]; ok {
		t.Error("session-delete should have been deleted")
	}
}

func TestLoadEmptySnapshots(t *testing.T) {
	dbURL := getTestDBURL(t)

	s, err := New(dbURL)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Clean up any existing data for a clean test
	// (we just check that LoadAllSnapshots doesn't error on an empty-ish table)
	snapshots, err := s.LoadAllSnapshots()
	if err != nil {
		t.Fatalf("failed to load snapshots from potentially empty table: %v", err)
	}

	// snapshots should be a valid (possibly non-empty) map, not nil
	if snapshots == nil {
		t.Error("expected non-nil map, got nil")
	}
}
