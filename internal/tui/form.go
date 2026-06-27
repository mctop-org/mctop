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

	"github.com/mctop/mctop/internal/mcp"
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
	doc     bool // a resource or prompt body, rendered as markdown rather than laid out as data
}

// openForm switches to the argument form for a tool, building one text field per
// argument and focusing the first.
func (m model) openForm(tool *sdk.Tool) model {
	args := toolArgs(tool)
	inputs := make([]formInput, len(args))
	for i, a := range args {
		ti := textinput.New()
		ti.Placeholder = a.hint()
		ti.Prompt = ""
		inputs[i] = formInput{arg: a, input: ti}
	}
	if len(inputs) > 0 {
		inputs[0].input.Focus()
	}
	m.formTool = tool
	m.inputs = inputs
	m.focus = 0
	m.formMsg = ""
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
			if i, missing := m.firstMissingRequired(); missing {
				m.formMsg = m.inputs[i].arg.Name + " is required"
				m.refocus(i)
				return m, nil
			}
			m.formMsg = ""
			return m.dispatch(m.formTool.Name, m.runCall())
		case "tab", "down":
			m.formMsg = ""
			m.refocus(m.focus + 1)
			return m, nil
		case "shift+tab", "up":
			m.formMsg = ""
			m.refocus(m.focus - 1)
			return m, nil
		}
		// A keystroke that edits the focused field clears a stale message.
		m.formMsg = ""
	}
	if len(m.inputs) > 0 {
		var cmd tea.Cmd
		m.inputs[m.focus].input, cmd = m.inputs[m.focus].input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// firstMissingRequired finds the first required field left blank, so the form
// can point at it instead of dispatching a call the server will only reject. A
// whitespace-only value counts as blank, matching collectArgs.
func (m model) firstMissingRequired() (int, bool) {
	for i, fi := range m.inputs {
		if fi.arg.Required && strings.TrimSpace(fi.input.Value()) == "" {
			return i, true
		}
	}
	return 0, false
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
		out := mcp.RenderResult(res)
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
		return callResultMsg{output: mcp.RenderResource(res), elapsed: elapsed, doc: true}
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
		return callResultMsg{output: mcp.RenderPrompt(res), elapsed: elapsed, doc: true}
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
	// A table result with no row expanded yet: the up/down family selects a row
	// instead of scrolling, and enter opens the selected row's detail view.
	listNav := m.rows != nil && !m.jsonView && !m.rowOpen

	// Vim motions for scrolling and going back, live only when vim mode is on.
	if m.vim {
		switch key.String() {
		case "j":
			if listNav {
				m.moveRow(1)
			} else {
				m.vp.LineDown(1)
			}
			return m, nil
		case "k":
			if listNav {
				m.moveRow(-1)
			} else {
				m.vp.LineUp(1)
			}
			return m, nil
		case "g":
			if listNav {
				m.setRow(0)
			} else {
				m.vp.GotoTop()
			}
			return m, nil
		case "G":
			if listNav {
				m.setRow(len(m.rows) - 1)
			} else {
				m.vp.GotoBottom()
			}
			return m, nil
		case "l":
			if listNav {
				m.expandRow()
				return m, nil
			}
		case "h":
			if m.rowOpen {
				m.collapseRow()
			} else {
				m.screen = browse
			}
			return m, nil
		}
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = true
	case "V":
		m.toggleVim()
	case "esc", "left", "backspace":
		if m.rowOpen {
			m.collapseRow()
		} else {
			m.screen = browse
		}
	case "enter", "right":
		if listNav {
			m.expandRow()
		}
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
		if m.outputIsJSON() || m.doc {
			m.jsonView = !m.jsonView
			m.rowOpen = false
			off := m.vp.YOffset
			m.vp.SetContent(m.resultBody())
			m.vp.SetYOffset(off)
		}
	case "y":
		if m.output != "" {
			m.yankSeq = ansi.SetSystemClipboard(m.output)
		}
	case "down":
		if listNav {
			m.moveRow(1)
		} else {
			m.vp.LineDown(1)
		}
	case "up":
		if listNav {
			m.moveRow(-1)
		} else {
			m.vp.LineUp(1)
		}
	case "ctrl+d":
		if listNav {
			m.moveRow(m.rowPage())
		} else {
			m.vp.HalfViewDown()
		}
	case "ctrl+u":
		if listNav {
			m.moveRow(-m.rowPage())
		} else {
			m.vp.HalfViewUp()
		}
	case "home":
		if listNav {
			m.setRow(0)
		} else {
			m.vp.GotoTop()
		}
	case "end":
		if listNav {
			m.setRow(len(m.rows) - 1)
		} else {
			m.vp.GotoBottom()
		}
	}
	return m, nil
}

// moveRow and setRow change the highlighted record and keep it on screen.
func (m *model) moveRow(delta int) { m.setRow(m.rowCursor + delta) }

func (m *model) setRow(i int) {
	m.rowCursor = clamp(i, 0, len(m.rows)-1)
	m.vp.SetContent(m.resultBody())
	m.ensureRowVisible()
}

// expandRow opens the selected record's full, untruncated detail view; collapseRow
// returns to the table with the cursor preserved.
func (m *model) expandRow() {
	m.rowOpen = true
	m.vp.SetContent(m.resultBody())
	m.vp.GotoTop()
}

func (m *model) collapseRow() {
	m.rowOpen = false
	m.vp.SetContent(m.resultBody())
	m.ensureRowVisible()
}

func (m *model) rowPage() int {
	if p := m.vp.Height - 2; p > 1 {
		return p
	}
	return 1
}

// rowsTopOffset is the line the first record sits on: any envelope summary, then
// the table's header and divider (a table result carries no error prefix).
func (m model) rowsTopOffset() int {
	return strings.Count(m.rowPrefix(m.vp.Width-2), "\n") + 2
}

// ensureRowVisible scrolls the viewport so the highlighted row stays in view.
func (m *model) ensureRowVisible() {
	if m.vp.Height < 1 {
		return
	}
	line := m.rowsTopOffset() + m.rowCursor
	switch top := m.vp.YOffset; {
	case line < top:
		m.vp.SetYOffset(line)
	case line >= top+m.vp.Height:
		m.vp.SetYOffset(line - m.vp.Height + 1)
	}
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
	if m.formMsg != "" {
		b.WriteString(red.Render(m.formMsg) + "\n\n")
	}
	b.WriteString(accent.Render("enter") + dim.Render(" to run"))
	return b.String()
}

const (
	maxPrettyBytes = 256 * 1024 // above this, skip the structured layout and show raw
	maxResultLines = 5000       // hard cap so a huge result cannot choke the screen
)

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
	return m.fitResult(b.String())
}

// renderOutput is the result payload as shown: the insight view by default, the
// colored indented JSON when the JSON view is toggled, and plain wrapped text for
// non-JSON output or payloads too large to lay out cheaply.
func (m model) renderOutput() string {
	w := m.vp.Width - 2
	if len(m.output) > maxPrettyBytes {
		return wrapPlain(m.output, w)
	}
	v, err := decodeOrdered(m.output)
	if err != nil {
		// Not JSON: a resource or prompt body renders as markdown (toggle to the
		// raw source with t); any other plain text just wraps.
		if m.doc && !m.jsonView {
			return renderMarkdown(m.output, w-2)
		}
		return wrapPlain(m.output, w)
	}
	if m.jsonView {
		if s, ok := indentJSON(m.output); ok {
			return s
		}
		return wrapPlain(m.output, w)
	}
	if m.rows != nil {
		if m.rowOpen {
			return humanValue(m.rows[m.rowCursor], w, 0)
		}
		if t, ok := renderObjectTable(m.rows, w, m.rowCursor); ok {
			return m.rowPrefix(w) + t
		}
	}
	return humanValue(v, w, 0)
}

// rowPrefix is what precedes the record table in the list view: for an envelope
// result, the wrapping object's other fields as a summary, then the table's
// titled heading. It is empty for a bare array.
func (m model) rowPrefix(w int) string {
	if m.rowEnvelope == nil {
		return ""
	}
	var b strings.Builder
	if rest := m.rowEnvelope.without(m.rowsKey); len(rest.keys) > 0 {
		b.WriteString(humanObject(rest, w, 0))
		b.WriteString("\n\n")
	}
	b.WriteString(accent.Bold(true).Render(humanLabel(m.rowsKey)) +
		dim.Render(fmt.Sprintf("  (%d)", len(m.rows))) + "\n")
	return b.String()
}

// fitResult is the safety net that keeps a result from breaking the terminal: it
// truncates every line to the pane width (ANSI-aware, so colors stay intact) and
// caps the total line count, pointing at raw and copy for the remainder.
func (m model) fitResult(s string) string {
	w := m.vp.Width
	if w < 1 {
		w = m.width
	}
	lines := strings.Split(s, "\n")
	truncated := false
	if len(lines) > maxResultLines {
		lines = lines[:maxResultLines]
		truncated = true
	}
	if w > 0 {
		for i, ln := range lines {
			if ansi.StringWidth(ln) > w {
				lines[i] = ansi.Truncate(ln, w, "…")
			}
		}
	}
	out := strings.Join(lines, "\n")
	if truncated {
		out += "\n" + dim.Render("  … output truncated. press t for raw, y to copy all.")
	}
	return out
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
	listNav := m.rows != nil && !m.jsonView && !m.rowOpen
	var keys string
	switch {
	case listNav:
		keys = "  ↑↓ select  ·  enter expand  ·  esc back  ·  r re-run"
	case m.rowOpen:
		keys = "  ↑↓ scroll  ·  esc back to list  ·  r re-run"
	default:
		keys = "  ↑↓ scroll  ·  esc back  ·  r re-run"
	}
	if m.formTool != nil {
		keys += "  ·  e edit"
	}
	switch {
	case m.doc && !m.outputIsJSON():
		to := "raw"
		if m.jsonView {
			to = "rendered"
		}
		keys += "  ·  t " + to
	case m.outputIsJSON():
		to := "json"
		if m.jsonView {
			to = "insight"
		}
		keys += "  ·  t " + to
	}
	keys += "  ·  y copy  ·  ? keys"
	pct := ""
	if m.vp.TotalLineCount() > m.vp.Height {
		pct = dim.Render(fmt.Sprintf("%d%%  ", int(m.vp.ScrollPercent()*100)))
	}
	right := pct
	if listNav {
		right = dim.Render(fmt.Sprintf("%d/%d  ", m.rowCursor+1, len(m.rows))) + pct
	}
	if m.yankSeq != "" {
		right = green.Render("copied  ") + pct
	}
	title := m.resultTitle + " → result"
	if m.rowOpen {
		title = m.resultTitle + fmt.Sprintf(" → row %d", m.rowCursor+1)
	}
	footer := m.rule() + "\n" + m.spread(dim.Render(keys), right)
	return m.layout(m.header(title, status), m.vp.View(), footer)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
