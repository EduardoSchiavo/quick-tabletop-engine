package session

import (
	"encoding/json"
	"testing"

	"quick-tabletop-engine/config"
	"quick-tabletop-engine/game"
)

func testConfig() config.Config {
	return config.Config{
		MaxSessions:        10,
		MaxUsersPerSession: 10,
		SnapshotIntervalSec: 30,
		DatabaseURL:        "",
	}
}

func TestNewManager(t *testing.T) {
	cfg := testConfig()
	m := NewManager(cfg)

	if m.cfg.MaxSessions != 10 {
		t.Errorf("expected maxSessions 10, got %d", m.cfg.MaxSessions)
	}
	if len(m.sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(m.sessions))
	}
}

func TestManagerReset(t *testing.T) {
	m := NewManager(testConfig())
	m.sessions["fake"] = &Session{ID: "fake", State: game.NewState()}

	m.Reset()

	if len(m.sessions) != 0 {
		t.Errorf("expected 0 sessions after reset, got %d", len(m.sessions))
	}
}

func makeCommand(t *testing.T, msgType string, payload interface{}) ClientMessage {
	t.Helper()
	var raw json.RawMessage
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
	}
	return ClientMessage{Type: msgType, Payload: raw}
}

func TestProcessCommandAddToken(t *testing.T) {
	state := game.NewState()
	cmd := makeCommand(t, "add_token", game.AddTokenPayload{
		ID:    "t1",
		Token: game.TokenData{Name: "Goblin", ImgPath: "/goblin.jpg", X: 96, Y: 96, TokenSize: 96},
	})

	processCommand(cmd, &state)

	if len(state.DisplayedTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(state.DisplayedTokens))
	}
	if state.DisplayedTokens["t1"].Name != "Goblin" {
		t.Errorf("expected Goblin, got %q", state.DisplayedTokens["t1"].Name)
	}
}

func TestProcessCommandMoveToken(t *testing.T) {
	state := game.NewState()
	state.AddToken("t1", game.TokenData{Name: "Goblin", X: 96, Y: 96, TokenSize: 96})

	cmd := makeCommand(t, "move_token", game.MoveTokenPayload{ID: "t1", X: 200, Y: 300})
	processCommand(cmd, &state)

	token := state.DisplayedTokens["t1"]
	if token.X != 200 || token.Y != 300 {
		t.Errorf("expected (200,300), got (%f,%f)", token.X, token.Y)
	}
}

func TestProcessCommandDeleteToken(t *testing.T) {
	state := game.NewState()
	state.AddToken("t1", game.TokenData{Name: "Goblin"})

	cmd := makeCommand(t, "delete_token", game.DeleteTokenPayload{ID: "t1"})
	processCommand(cmd, &state)

	if len(state.DisplayedTokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(state.DisplayedTokens))
	}
}

func TestProcessCommandClearTokens(t *testing.T) {
	state := game.NewState()
	state.AddToken("t1", game.TokenData{Name: "Goblin"})
	state.AddToken("t2", game.TokenData{Name: "Orc"})

	cmd := makeCommand(t, "clear_tokens", nil)
	processCommand(cmd, &state)

	if len(state.DisplayedTokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(state.DisplayedTokens))
	}
}

func TestProcessCommandChangeBackground(t *testing.T) {
	state := game.NewState()

	cmd := makeCommand(t, "change_background", game.ChangeBackgroundPayload{ImgPath: "/forest.jpg"})
	processCommand(cmd, &state)

	if state.BackgroundImgPath != "/forest.jpg" {
		t.Errorf("expected /forest.jpg, got %q", state.BackgroundImgPath)
	}
}

func TestProcessCommandToggleGrid(t *testing.T) {
	state := game.NewState()

	cmd := makeCommand(t, "toggle_grid", nil)
	processCommand(cmd, &state)

	if state.ShowGrid {
		t.Error("expected showGrid false after toggle")
	}
}

func TestProcessCommandUnknownType(t *testing.T) {
	state := game.NewState()

	cmd := makeCommand(t, "unknown_command", nil)
	processCommand(cmd, &state)

	// Should not panic, state should be unchanged
	if len(state.DisplayedTokens) != 0 {
		t.Error("state should be unchanged for unknown command")
	}
}

func TestProcessCommandAddAreaTemplate(t *testing.T) {
	state := game.NewState()
	cmd := makeCommand(t, "add_area_template", game.AddAreaTemplatePayload{
		ID: "at1",
		Template: game.AreaTemplate{
			Shape:   "circle",
			X:       192,
			Y:       288,
			Size:    3,
			Color:   "#ff0000",
			Opacity: 0.5,
		},
	})

	processCommand(cmd, &state)

	if len(state.AreaTemplates) != 1 {
		t.Fatalf("expected 1 area template, got %d", len(state.AreaTemplates))
	}
	got := state.AreaTemplates["at1"]
	if got.Shape != "circle" {
		t.Errorf("expected shape circle, got %q", got.Shape)
	}
	if got.Color != "#ff0000" {
		t.Errorf("expected color #ff0000, got %q", got.Color)
	}
}

func TestProcessCommandAddAreaTemplateAutoID(t *testing.T) {
	state := game.NewState()
	// Send without an ID — processCommand should generate a UUID
	cmd := makeCommand(t, "add_area_template", game.AddAreaTemplatePayload{
		Template: game.AreaTemplate{
			Shape:   "square",
			X:       96,
			Y:       96,
			Size:    2,
			Color:   "#00ff00",
			Opacity: 0.3,
		},
	})

	processCommand(cmd, &state)

	if len(state.AreaTemplates) != 1 {
		t.Fatalf("expected 1 area template, got %d", len(state.AreaTemplates))
	}
	// Verify the generated ID is not empty
	for id, tmpl := range state.AreaTemplates {
		if id == "" {
			t.Error("expected non-empty auto-generated ID")
		}
		if tmpl.Shape != "square" {
			t.Errorf("expected shape square, got %q", tmpl.Shape)
		}
	}
}

func TestProcessCommandMoveAreaTemplate(t *testing.T) {
	state := game.NewState()
	state.AddAreaTemplate("at1", game.AreaTemplate{Shape: "circle", X: 96, Y: 96, Size: 2, Color: "#ff0000", Opacity: 0.5})

	cmd := makeCommand(t, "move_area_template", game.MoveAreaTemplatePayload{ID: "at1", X: 384, Y: 480})
	processCommand(cmd, &state)

	got := state.AreaTemplates["at1"]
	if got.X != 384 || got.Y != 480 {
		t.Errorf("expected position (384,480), got (%f,%f)", got.X, got.Y)
	}
}

func TestProcessCommandDeleteAreaTemplate(t *testing.T) {
	state := game.NewState()
	state.AddAreaTemplate("at1", game.AreaTemplate{Shape: "circle", X: 96, Y: 96, Size: 2, Color: "#ff0000", Opacity: 0.5})

	cmd := makeCommand(t, "delete_area_template", game.DeleteAreaTemplatePayload{ID: "at1"})
	processCommand(cmd, &state)

	if len(state.AreaTemplates) != 0 {
		t.Errorf("expected 0 area templates, got %d", len(state.AreaTemplates))
	}
}

func TestProcessCommandClearAreaTemplates(t *testing.T) {
	state := game.NewState()
	state.AddAreaTemplate("at1", game.AreaTemplate{Shape: "circle", X: 96, Y: 96, Size: 2, Color: "#ff0000", Opacity: 0.5})
	state.AddAreaTemplate("at2", game.AreaTemplate{Shape: "square", X: 192, Y: 192, Size: 3, Color: "#00ff00", Opacity: 0.3})

	cmd := makeCommand(t, "clear_area_templates", nil)
	processCommand(cmd, &state)

	if len(state.AreaTemplates) != 0 {
		t.Errorf("expected 0 area templates after clear, got %d", len(state.AreaTemplates))
	}
}
