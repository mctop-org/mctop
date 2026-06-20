package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aloki-alok/mctop/internal/mcp"
)

const callTimeout = 60 * time.Second

// formInput pairs a tool argument with its text field.
type formInput struct {
	arg   Arg
	input textinput.Model
}

// callResultMsg carries the outcome of a tool call back into Update.
type callResultMsg struct {
	output  string
	err     error
	elapsed string
}

// openForm switches to the argument form for a tool, building one text field per
// argument and focusing the first.
func (m model) openForm(tool *sdk.Tool) model {
	args := toolArgs(tool)
	inputs := make([]formInput, len(args))
	for i, a := range args {
		ti := textinput.New()
		ti.Placeholder = a.Type
		ti.Prompt = ""
		inputs[i] = formInput{arg: a, input: ti}
	}
	if len(inputs) > 0 {
		inputs[0].input.Focus()
	}
	m.formTool = tool
	m.inputs = inputs
	m.focus = 0
	m.screen = form
	m.output, m.resultErr, m.elapsed = "", nil, ""
	return m
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.screen = browse
			return m, nil
		case "enter":
			return m.dispatch(m.formTool.Name, m.runCall())
		case "tab", "down":
			m.refocus(m.focus + 1)
			return m, nil
		case "shift+tab", "up":
			m.refocus(m.focus - 1)
			return m, nil
		}
	}
	if len(m.inputs) > 0 {
		var cmd tea.Cmd
		m.inputs[m.focus].input, cmd = m.inputs[m.focus].input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// refocus moves the focused field, wrapping around the list.
func (m *model) refocus(to int) {
	if len(m.inputs) == 0 {
		return
	}
	m.inputs[m.focus].input.Blur()
	m.focus = (to + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focus].input.Focus()
}

func (m model) runCall() tea.Cmd {
	tool := m.formTool.Name
	args := collectArgs(m.inputs)
	client := m.client
	parent := m.ctx
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, callTimeout)
		defer cancel()
		start := time.Now()
		res, err := client.Call(ctx, tool, args)
		elapsed := time.Since(start).Round(time.Millisecond).String()
		if err != nil {
			return callResultMsg{err: err, elapsed: elapsed}
		}
		out := mcp.RenderContent(res.Content)
		if res.IsError {
			return callResultMsg{err: fmt.Errorf("tool returned an error"), output: out, elapsed: elapsed}
		}
		return callResultMsg{output: out, elapsed: elapsed}
	}
}

func (m model) readResource(uri string) tea.Cmd {
	client, parent := m.client, m.ctx
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, callTimeout)
		defer cancel()
		start := time.Now()
		res, err := client.ReadResource(ctx, uri)
		elapsed := time.Since(start).Round(time.Millisecond).String()
		if err != nil {
			return callResultMsg{err: err, elapsed: elapsed}
		}
		return callResultMsg{output: mcp.RenderResource(res), elapsed: elapsed}
	}
}

func (m model) getPrompt(name string) tea.Cmd {
	client, parent := m.client, m.ctx
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, callTimeout)
		defer cancel()
		start := time.Now()
		res, err := client.GetPrompt(ctx, name, nil)
		elapsed := time.Since(start).Round(time.Millisecond).String()
		if err != nil {
			return callResultMsg{err: err, elapsed: elapsed}
		}
		return callResultMsg{output: mcp.RenderPrompt(res), elapsed: elapsed}
	}
}

// collectArgs reads the filled fields into call arguments, skipping blanks and
// parsing each value as JSON so numbers and booleans are typed, falling back to
// a string.
func collectArgs(inputs []formInput) map[string]any {
	args := make(map[string]any)
	for _, fi := range inputs {
		raw := strings.TrimSpace(fi.input.Value())
		if raw == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			v = raw
		}
		args[fi.arg.Name] = v
	}
	return args
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = true
	case "esc", "h", "left", "backspace":
		m.screen = browse
	case "e":
		if m.formTool != nil {
			m.screen = form
		}
	case "r":
		if m.lastCmd != nil {
			m.running = true
			return m, tea.Batch(m.lastCmd, m.spin.Tick)
		}
	case "t":
		if m.outputIsJSON() {
			m.rawView = !m.rawView
			off := m.vp.YOffset
			m.vp.SetContent(m.resultBody())
			m.vp.SetYOffset(off)
		}
	case "y":
		if m.output != "" {
			m.yankSeq = ansi.SetSystemClipboard(m.output)
		}
	case "j", "down":
		m.vp.LineDown(1)
	case "k", "up":
		m.vp.LineUp(1)
	case "ctrl+d":
		m.vp.HalfViewDown()
	case "ctrl+u":
		m.vp.HalfViewUp()
	case "g", "home":
		m.vp.GotoTop()
	case "G", "end":
		m.vp.GotoBottom()
	}
	return m, nil
}

func (m model) viewForm() string {
	bh := m.bodyHeight()
	body := lipgloss.NewStyle().Height(bh).MaxHeight(bh).Padding(1, 3).Render(m.formBody())
	footer := m.rule() + "\n" + dim.Render("  tab field  ·  enter run  ·  esc back")
	return m.layout(m.header(m.formTool.Name, dim.Render("fill arguments")), body, footer)
}

func (m model) formBody() string {
	if len(m.inputs) == 0 {
		return dim.Render("this tool takes no arguments") + "\n\n" + accent.Render("enter") + dim.Render(" to run")
	}
	var b strings.Builder
	for i, fi := range m.inputs {
		pointer := "  "
		if i == m.focus {
			pointer = barS.Render("▌") + " "
		}
		plain := fi.arg.Name
		styled := bold.Render(fi.arg.Name)
		if fi.arg.Required {
			plain += "*"
			styled += accent.Render("*")
		}
		pad := 16 - len([]rune(plain))
		if pad < 1 {
			pad = 1
		}
		b.WriteString(pointer + styled + strings.Repeat(" ", pad) + fi.input.View())
		if fi.arg.Type != "" {
			b.WriteString("  " + dim.Render(fi.arg.Type))
		}
		b.WriteString("\n\n")
	}
	b.WriteString(accent.Render("enter") + dim.Render(" to run"))
	return b.String()
}

func (m model) resultBody() string {
	var b strings.Builder
	if m.resultErr != nil {
		b.WriteString(red.Render("  " + m.resultErr.Error()))
		if m.output != "" {
			b.WriteString("\n")
		}
	}
	if m.output != "" {
		b.WriteString(indent(m.renderOutput()))
	}
	return b.String()
}

// renderOutput is the result payload as shown: pretty-colored JSON by default,
// or the verbatim text wrapped to the viewport width in raw view and for any
// non-JSON output, so nothing is clipped off the right edge.
func (m model) renderOutput() string {
	if !m.rawView {
		if pretty, ok := prettyJSON(m.output); ok {
			return pretty
		}
	}
	return wrapPlain(m.output, m.vp.Width-2)
}

// wrapPlain hard-wraps unstyled text to w columns so a long single line stays
// fully scrollable. It is only used on text without ANSI styling, so wrapping by
// rune count is exact.
func wrapPlain(s string, w int) string {
	if w < 1 {
		return s
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		r := []rune(line)
		for len(r) > w {
			out = append(out, string(r[:w]))
			r = r[w:]
		}
		out = append(out, string(r))
	}
	return strings.Join(out, "\n")
}

// outputIsJSON reports whether the result can be pretty-printed, which gates the
// raw/pretty toggle.
func (m model) outputIsJSON() bool {
	return m.output != "" && json.Valid([]byte(m.output))
}

func (m model) viewResult() string {
	bh := m.bodyHeight()
	if m.running {
		body := lipgloss.NewStyle().Height(bh).MaxHeight(bh).Padding(1, 3).Render(m.spin.View() + dim.Render("running"))
		footer := m.rule() + "\n" + dim.Render("  esc cancel")
		return m.layout(m.header(m.resultTitle, dim.Render("calling")), body, footer)
	}
	status := green.Render("✓ ") + dim.Render(m.elapsed)
	if m.resultErr != nil {
		status = red.Render("✗ ") + dim.Render(m.elapsed)
	}
	keys := "  ↑↓ scroll  ·  esc back  ·  r re-run"
	if m.formTool != nil {
		keys += "  ·  e edit"
	}
	if m.outputIsJSON() {
		to := "raw"
		if m.rawView {
			to = "pretty"
		}
		keys += "  ·  t " + to
	}
	keys += "  ·  y copy  ·  ? keys"
	pct := ""
	if m.vp.TotalLineCount() > m.vp.Height {
		pct = dim.Render(fmt.Sprintf("%d%%  ", int(m.vp.ScrollPercent()*100)))
	}
	right := pct
	if m.yankSeq != "" {
		right = green.Render("copied  ") + pct
	}
	footer := m.rule() + "\n" + m.spread(dim.Render(keys), right)
	return m.layout(m.header(m.resultTitle+" → result", status), m.vp.View(), footer)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
