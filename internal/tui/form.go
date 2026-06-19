package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	case "esc":
		m.screen = browse
	case "e":
		if m.formTool != nil {
			m.screen = form
		}
	case "r":
		if m.lastCmd != nil {
			m.running = true
			return m, m.lastCmd
		}
	}
	return m, nil
}

func (m model) viewForm() string {
	var b strings.Builder
	b.WriteString(m.header(m.formTool.Name, dim.Render("call")) + "\n")
	if len(m.inputs) == 0 {
		b.WriteString("\n" + dim.Render("  no arguments") + "\n")
	}
	for i, fi := range m.inputs {
		name := fi.arg.Name
		if fi.arg.Required {
			name += "*"
		}
		pointer := "  "
		label := fmt.Sprintf("%-16s", name)
		if i == m.focus {
			pointer = cursorS.Render("▸ ")
			label = bold.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s %s  %s\n", pointer, label, fi.input.View(), dim.Render(fi.arg.Type)))
	}
	b.WriteString("\n  " + accent.Render("[ enter to run ]") + "\n")
	b.WriteString(m.rule() + "\n" + dim.Render("  ↑↓ field   enter run   esc back"))
	return b.String()
}

func (m model) viewResult() string {
	if m.running {
		return m.header(m.resultTitle, dim.Render("running…")) + "\n\n" + dim.Render("  running...")
	}
	status := accent.Render("✓ ") + dim.Render(m.elapsed)
	if m.resultErr != nil {
		status = red.Render("✗ ") + dim.Render(m.elapsed)
	}
	var b strings.Builder
	b.WriteString(m.header(m.resultTitle+" → result", status) + "\n")
	if m.resultErr != nil {
		b.WriteString(red.Render("  "+m.resultErr.Error()) + "\n")
	}
	if m.output != "" {
		b.WriteString(indent(m.output) + "\n")
	}
	keys := "  r re-run   esc back   q quit"
	if m.formTool != nil {
		keys = "  r re-run   e edit args   esc back   q quit"
	}
	b.WriteString(m.rule() + "\n" + dim.Render(keys))
	return b.String()
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
