package game

import "testing"

func TestNewState(t *testing.T) {
	s := NewState()

	if len(s.DisplayedTokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(s.DisplayedTokens))
	}
	if s.BackgroundImgPath != "/assets/default/maps/tavern.jpg" {
		t.Errorf("expected tavern background, got %q", s.BackgroundImgPath)
	}
	if !s.ShowGrid {
		t.Error("expected showGrid to be true")
	}
	if s.GridUnit != 96 {
		t.Errorf("expected gridUnit 96, got %f", s.GridUnit)
	}
}

func TestAddToken(t *testing.T) {
	s := NewState()
	token := TokenData{Name: "Goblin", ImgPath: "/goblin.jpg", X: 96, Y: 96, TokenSize: 96}

	s.AddToken("t1", token)

	if len(s.DisplayedTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(s.DisplayedTokens))
	}
	got := s.DisplayedTokens["t1"]
	if got.Name != "Goblin" {
		t.Errorf("expected name Goblin, got %q", got.Name)
	}
	if got.X != 96 || got.Y != 96 {
		t.Errorf("expected position (96,96), got (%f,%f)", got.X, got.Y)
	}
}

func TestMoveToken(t *testing.T) {
	s := NewState()
	s.AddToken("t1", TokenData{Name: "Goblin", X: 96, Y: 96, TokenSize: 96})

	s.MoveToken("t1", 200, 300)

	got := s.DisplayedTokens["t1"]
	if got.X != 200 || got.Y != 300 {
		t.Errorf("expected position (200,300), got (%f,%f)", got.X, got.Y)
	}
}

func TestMoveTokenNonExistent(t *testing.T) {
	s := NewState()

	// Should not panic or create a new entry
	s.MoveToken("does-not-exist", 100, 100)

	if len(s.DisplayedTokens) != 0 {
		t.Error("moving a non-existent token should not create one")
	}
}

func TestDeleteToken(t *testing.T) {
	s := NewState()
	s.AddToken("t1", TokenData{Name: "Goblin"})
	s.AddToken("t2", TokenData{Name: "Orc"})

	s.DeleteToken("t1")

	if len(s.DisplayedTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(s.DisplayedTokens))
	}
	if _, ok := s.DisplayedTokens["t1"]; ok {
		t.Error("t1 should have been deleted")
	}
	if _, ok := s.DisplayedTokens["t2"]; !ok {
		t.Error("t2 should still exist")
	}
}

func TestClearTokens(t *testing.T) {
	s := NewState()
	s.AddToken("t1", TokenData{Name: "Goblin"})
	s.AddToken("t2", TokenData{Name: "Orc"})
	s.AddToken("t3", TokenData{Name: "Elf"})

	s.ClearTokens()

	if len(s.DisplayedTokens) != 0 {
		t.Errorf("expected 0 tokens after clear, got %d", len(s.DisplayedTokens))
	}
}

func TestChangeBackgroundImg(t *testing.T) {
	s := NewState()

	s.ChangeBackgroundImg("/assets/default/maps/forest.jpg")

	if s.BackgroundImgPath != "/assets/default/maps/forest.jpg" {
		t.Errorf("expected forest background, got %q", s.BackgroundImgPath)
	}
}

func TestToggleGrid(t *testing.T) {
	s := NewState()

	if !s.ShowGrid {
		t.Fatal("expected showGrid to start true")
	}

	s.ToggleGrid()
	if s.ShowGrid {
		t.Error("expected showGrid to be false after first toggle")
	}

	s.ToggleGrid()
	if !s.ShowGrid {
		t.Error("expected showGrid to be true after second toggle")
	}
}
