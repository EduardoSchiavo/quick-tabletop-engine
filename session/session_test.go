package session

import (
	"encoding/json"
	"testing"

	"quick-tabletop-engine/game"
)

func TestNewManager(t *testing.T) {
	m := NewManager(10)

	if m.maxSessions != 10 {
		t.Errorf("expected maxSessions 10, got %d", m.maxSessions)
	}
	if len(m.sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(m.sessions))
	}
}

func TestManagerReset(t *testing.T) {
	m := NewManager(10)
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
