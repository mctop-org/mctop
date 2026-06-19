package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func testModel() model {
	return model{
		server:    "test",
		tools:     []*sdk.Tool{{Name: "alpha"}, {Name: "beta"}},
		resources: []*sdk.Resource{{URI: "file://x"}},
		prompts:   []*sdk.Prompt{{Name: "p"}},
		width:     80,
		height:    24,
	}
}

func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	return tea.KeyMsg{Type: map[string]tea.KeyType{"tab": tea.KeyTab, "down": tea.KeyDown, "up": tea.KeyUp}[s]}
}

func send(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

func TestCursorMovesAndClamps(t *testing.T) {
	m := testModel()
	m, _ = send(m, key("down"))
	if m.cursor[secTools] != 1 {
		t.Fatalf("down: want cursor 1, got %d", m.cursor[secTools])
	}
	m, _ = send(m, key("down")) // already at last of two tools
	if m.cursor[secTools] != 1 {
		t.Fatalf("down clamp: want cursor 1, got %d", m.cursor[secTools])
	}
	m, _ = send(m, key("up"))
	if m.cursor[secTools] != 0 {
		t.Fatalf("up: want cursor 0, got %d", m.cursor[secTools])
	}
}

func TestTabSwitchesSection(t *testing.T) {
	m := testModel()
	m, _ = send(m, key("tab"))
	if m.section != secResources {
		t.Fatalf("tab: want resources, got %v", m.section)
	}
}

func TestQuitReturnsCommand(t *testing.T) {
	m := testModel()
	if _, cmd := send(m, key("q")); cmd == nil {
		t.Fatal("q should return a quit command")
	}
}

func TestViewShowsToolNames(t *testing.T) {
	out := testModel().View()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "TOOLS (2)") {
		t.Fatalf("view missing expected content:\n%s", out)
	}
}
