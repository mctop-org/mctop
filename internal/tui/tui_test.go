package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
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
		vim:       true,
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
	if m.cursor != 1 {
		t.Fatalf("down: want cursor 1, got %d", m.cursor)
	}
	m, _ = send(m, key("down")) // already at last of two tools
	if m.cursor != 1 {
		t.Fatalf("down clamp: want cursor 1, got %d", m.cursor)
	}
	m, _ = send(m, key("up"))
	if m.cursor != 0 {
		t.Fatalf("up: want cursor 0, got %d", m.cursor)
	}
}

func TestJumpTopBottom(t *testing.T) {
	m := testModel() // two tools
	m, _ = send(m, key("G"))
	if m.cursor != 1 {
		t.Fatalf("G should jump to last item, got %d", m.cursor)
	}
	m, _ = send(m, key("g"))
	if m.cursor != 0 {
		t.Fatalf("g should jump to top, got %d", m.cursor)
	}
}

func TestHelpToggle(t *testing.T) {
	m := testModel()
	m, _ = send(m, key("?"))
	if !m.showHelp {
		t.Fatal("? should open the help overlay")
	}
	m, _ = send(m, key("j"))
	if m.showHelp {
		t.Fatal("any key should close the help overlay")
	}
}

func TestResultGoesBack(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("h")},
		{Type: tea.KeyLeft},
		{Type: tea.KeyEsc},
	} {
		m := model{screen: result, width: 80, height: 24, spin: newSpinner(), vim: true}
		next, _ := m.Update(msg)
		if next.(model).screen != browse {
			t.Fatalf("key %v should return to browse", msg)
		}
	}
}

func TestResultViewToggle(t *testing.T) {
	m := model{screen: result, width: 80, height: 24, spin: newSpinner(), output: `{"a":1}`}
	m.vp = viewport.New(80, 10)
	m.vp.SetContent(m.resultBody())
	if m.jsonView {
		t.Fatal("results should default to the pretty view")
	}
	m, _ = send(m, key("t"))
	if !m.jsonView {
		t.Fatal("t should switch to the raw view")
	}
	m, _ = send(m, key("t"))
	if m.jsonView {
		t.Fatal("t should switch back to pretty")
	}
}

func TestResultViewToggleIgnoredForNonJSON(t *testing.T) {
	m := model{screen: result, width: 80, height: 24, spin: newSpinner(), output: "plain text"}
	m.vp = viewport.New(80, 10)
	m, _ = send(m, key("t"))
	if m.jsonView {
		t.Fatal("t should do nothing when the result is not JSON")
	}
}

func TestYankCopiesAndClears(t *testing.T) {
	m := model{screen: result, width: 80, height: 24, spin: newSpinner(), output: `{"a":1}`}
	m.vp = viewport.New(80, 10)
	m, _ = send(m, key("y"))
	if !strings.HasPrefix(m.yankSeq, "\x1b]52;c;") {
		t.Fatalf("y should stage an OSC52 clipboard sequence, got %q", m.yankSeq)
	}
	if !strings.Contains(m.View(), m.yankSeq) {
		t.Fatal("the staged sequence should ride along in the next frame")
	}
	if !strings.Contains(m.View(), "copied") {
		t.Fatal("the footer should confirm the copy")
	}
	m, _ = send(m, key("j"))
	if m.yankSeq != "" {
		t.Fatal("the next key should clear the yank so OSC52 emits once")
	}
}

func TestVimToggle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := testModel()
	m, _ = send(m, key("V"))
	if m.vim {
		t.Fatal("V should turn vim off")
	}
	m, _ = send(m, key("V"))
	if !m.vim {
		t.Fatal("V should turn vim back on")
	}
}

func TestVimOffDisablesMotions(t *testing.T) {
	m := testModel()
	m.vim = false
	m, _ = send(m, key("j"))
	if m.cursor != 0 {
		t.Fatalf("j should not move the cursor with vim off, got %d", m.cursor)
	}
	m, _ = send(m, key("down"))
	if m.cursor != 1 {
		t.Fatalf("arrow keys should still move with vim off, got %d", m.cursor)
	}
}

func TestResultBodyNeverExceedsWidth(t *testing.T) {
	// A nested object with a very long string value takes the indented path,
	// which does not wrap; fitResult must still keep every line within width.
	m := model{screen: result, width: 40, height: 24}
	m.vp = viewport.New(40, 10)
	m.output = `{"a":{"x":"` + strings.Repeat("z", 600) + `"}}`
	for _, ln := range strings.Split(m.resultBody(), "\n") {
		if w := len([]rune(stripANSI(ln))); w > 40 {
			t.Fatalf("line exceeds width 40 (%d): %q", w, stripANSI(ln))
		}
	}
}

func TestResultBodyCapsLineCount(t *testing.T) {
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 0; i < 7000; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('1')
	}
	sb.WriteByte(']')
	m := model{screen: result, width: 60, height: 24}
	m.vp = viewport.New(60, 10)
	m.output = sb.String()
	body := m.resultBody()
	if n := strings.Count(body, "\n") + 1; n > maxResultLines+2 {
		t.Fatalf("line count not capped: %d", n)
	}
	if !strings.Contains(body, "truncated") {
		t.Fatal("expected a truncation note")
	}
}

func TestLargePayloadSkipsStructuredLayout(t *testing.T) {
	m := model{screen: result, width: 60, height: 24}
	m.vp = viewport.New(60, 10)
	m.output = `{"data":"` + strings.Repeat("x", maxPrettyBytes) + `"}`
	if out := m.renderOutput(); !strings.HasPrefix(out, "{") {
		t.Fatal("a payload over the size cap should render raw, not aligned")
	}
}

func TestSearchFilters(t *testing.T) {
	m := testModel() // tools: alpha, beta
	m, _ = send(m, key("/"))
	if !m.searching {
		t.Fatal("/ should start searching")
	}
	for _, r := range "bet" {
		m, _ = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	vis := m.visibleItems()
	if len(vis) != 1 || m.tools[vis[0]].Name != "beta" {
		t.Fatalf("search 'bet' should match only beta, got %v", vis)
	}
	if got := m.selected(); got != 1 {
		t.Fatalf("selected should be index 1 (beta), got %d", got)
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
