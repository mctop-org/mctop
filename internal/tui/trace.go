package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mctop/mctop/internal/mcp"
)

// traceBody renders the recorded protocol frames for the trace overlay: each
// frame is a labelled line (direction, method or kind, time) above its
// pretty-printed JSON, so a session reads top to bottom like a network log.
func (m model) traceBody() string {
	frames := m.client.Trace()
	if len(frames) == 0 {
		return lipgloss.NewStyle().Padding(1, 3).Render(dim.Render("No protocol frames yet. Run a call, then reopen this view."))
	}
	var b strings.Builder
	for i, f := range frames {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(traceLine(f) + "\n")
		body := f.Data
		if pretty, ok := indentJSON(f.Data); ok {
			body = pretty
		}
		b.WriteString(dim.Render(indent(body)) + "\n")
	}
	return lipgloss.NewStyle().Padding(1, 3).Render(strings.TrimRight(b.String(), "\n"))
}

// traceLine is the one-line header above a frame's JSON: a colored direction
// arrow, the method or kind, and the time it crossed.
func traceLine(f mcp.Frame) string {
	arrow, label, style := "→", "sent", accent
	switch f.Dir {
	case mcp.Received:
		arrow, label, style = "←", "received", green
	case mcp.Failed:
		arrow, label, style = "×", "error", red
	}
	head := style.Render(arrow + " " + label)
	if s := frameSummary(f.Data); s != "" {
		head += dim.Render("  " + s)
	}
	return head + dim.Render("  "+f.At.Format("15:04:05.000"))
}

// frameSummary pulls a short tag out of a JSON-RPC frame: the method for a
// request or notification, or a reply's shape when there is no method.
func frameSummary(data string) string {
	var msg struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal([]byte(data), &msg) != nil {
		return ""
	}
	switch {
	case msg.Method != "":
		return msg.Method
	case msg.Error != nil:
		return "error response"
	case len(msg.ID) > 0:
		return "response"
	}
	return ""
}

// viewTrace draws the trace overlay: the recorded frames in a scrollable
// viewport, framed by the standard header and footer.
func (m model) viewTrace() string {
	right := dim.Render(fmt.Sprintf("%d frames", len(m.client.Trace())))
	if m.traceVP.TotalLineCount() > m.traceVP.Height {
		right += dim.Render(fmt.Sprintf("  ·  %d%%", int(m.traceVP.ScrollPercent()*100)))
	}
	footer := m.rule() + "\n" + dim.Render("  ↑↓ scroll  ·  esc close")
	return m.layout(m.header("trace", right), m.traceVP.View(), footer)
}
