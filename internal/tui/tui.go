// Package tui is mctop's interactive terminal client: connect to a server and
// move through it step by step, browse what it exposes, fill a tool's arguments,
// run it, and read the result, without leaving the keyboard.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aloki-alok/mctop/internal/config"
	"github.com/aloki-alok/mctop/internal/mcp"
)

// chromeHeight is the lines the header (2) and footer (2) occupy, reserved from
// the body so panes fill the rest of the terminal and the footer pins to the
// bottom.
const chromeHeight = 4

type section int

const (
	secTools section = iota
	secResources
	secPrompts
)

func (s section) String() string {
	return [...]string{"tools", "resources", "prompts"}[s]
}

// screen is which view is active: the browser, a tool's argument form, or a
// call's result.
type screen int

const (
	browse screen = iota
	form
	result
)

var (
	colAccent = lipgloss.Color("141") // iris
	colBorder = lipgloss.Color("238") // hairline divider
	colGreen  = lipgloss.Color("114")
	colRed    = lipgloss.Color("203")

	accent  = lipgloss.NewStyle().Foreground(colAccent)
	bold    = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Faint(true)
	cursorS = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	red     = lipgloss.NewStyle().Foreground(colRed)
	green   = lipgloss.NewStyle().Foreground(colGreen)
	barS    = lipgloss.NewStyle().Foreground(colAccent)
)

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = accent
	return s
}

// model holds the connected session and the state of whichever screen is
// active: the browse cursor, the open form, and the last result.
type model struct {
	ctx       context.Context
	server    string
	client    *mcp.Client
	tools     []*sdk.Tool
	resources []*sdk.Resource
	prompts   []*sdk.Prompt

	screen    screen
	section   section
	cursor    int // position within the active section's visible (filtered) items
	query     string
	searching bool
	showHelp  bool
	showTrace bool // the protocol trace overlay is open over whichever screen is active
	vim       bool // when on, the hjkl-family motions are active alongside the arrows

	width, height int

	// form and result state, populated when those screens are active.
	formTool    *sdk.Tool
	inputs      []formInput
	focus       int
	running     bool
	resultTitle string
	lastCmd     tea.Cmd
	output      string
	resultErr   error
	elapsed     string
	jsonView    bool      // result screen: show the raw form (JSON for data, source for a doc) instead of the rendered view
	doc         bool      // result screen: the output is a resource or prompt body, rendered as markdown
	rows        []*object // result screen: the records when the payload is a table, enabling row navigation
	rowEnvelope *object   // the wrapping object when the records are nested under one field, nil for a bare array
	rowsKey     string    // the envelope field that holds the records, for the table's title
	rowCursor   int       // selected record in the table
	rowOpen     bool      // a single record is expanded into its own detail view
	yankSeq     string    // OSC52 sequence to emit on the next render, cleared on the next key
	vp          viewport.Model
	traceVP     viewport.Model // scrolls the protocol trace overlay, kept apart from the result viewport
	spin        spinner.Model
}

// dispatch starts an action (tool call, resource read, prompt render), showing
// the result screen in a running state and remembering the command so r can
// re-run it.
func (m model) dispatch(title string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.running = true
	m.resultTitle = title
	m.lastCmd = cmd
	m.screen = result
	// Run the action and animate the spinner alongside it.
	return m, tea.Batch(cmd, m.spin.Tick)
}

// New builds the model from an already-connected client and its loaded surface.
func New(ctx context.Context, server string, client *mcp.Client, tools []*sdk.Tool, resources []*sdk.Resource, prompts []*sdk.Prompt) tea.Model {
	return model{ctx: ctx, server: server, client: client, tools: tools, resources: resources, prompts: prompts, vim: config.Load().Vim, vp: viewport.New(0, 0), traceVP: viewport.New(0, 0), spin: newSpinner()}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - chromeHeight
		m.traceVP.Width = msg.Width
		m.traceVP.Height = msg.Height - chromeHeight
		return m, nil
	case callResultMsg:
		m.running = false
		m.output, m.resultErr, m.elapsed = msg.output, msg.err, msg.elapsed
		m.jsonView = false
		m.doc = msg.doc
		m.rows, m.rowEnvelope, m.rowsKey, m.rowCursor, m.rowOpen = nil, nil, "", 0, false
		// A successful result whose records are an array of objects, bare or
		// wrapped in an envelope, becomes row-navigable.
		if msg.err == nil && len(msg.output) <= maxPrettyBytes {
			if v, e := decodeOrdered(msg.output); e == nil {
				m.rows, m.rowEnvelope, m.rowsKey = detectRows(v)
			}
		}
		m.screen = result
		m.vp.SetContent(m.resultBody())
		m.vp.GotoTop()
		return m, nil
	case spinner.TickMsg:
		// Keep the spinner turning only while an action is in flight.
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	// A yank rides along in one render frame; the next key clears it so the
	// OSC52 sequence is emitted to the terminal exactly once.
	if _, ok := msg.(tea.KeyMsg); ok {
		m.yankSeq = ""
	}

	// The help overlay swallows the next keypress to dismiss itself.
	if key, ok := msg.(tea.KeyMsg); ok && m.showHelp {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.showHelp = false
		return m, nil
	}

	// The trace overlay scrolls while open and closes on its own keys; any other
	// key is fed to its viewport so the protocol log can be paged through.
	if key, ok := msg.(tea.KeyMsg); ok && m.showTrace {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q", "T":
			m.showTrace = false
			return m, nil
		}
		var cmd tea.Cmd
		m.traceVP, cmd = m.traceVP.Update(msg)
		return m, cmd
	}

	// T opens the protocol trace over the browse or result screen. It is skipped
	// while typing in the search box or a form so the key stays available there.
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "T" && !m.searching && m.screen != form {
		m.showTrace = true
		m.traceVP.SetContent(m.traceBody())
		m.traceVP.GotoBottom()
		return m, nil
	}

	switch m.screen {
	case form:
		return m.updateForm(msg)
	case result:
		return m.updateResult(msg)
	default:
		return m.updateBrowse(msg)
	}
}

func (m model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		return m.updateSearch(key)
	}
	last := len(m.visibleItems()) - 1
	// Vim motions are only live when vim mode is on; the arrow-key equivalents
	// below always work.
	if m.vim {
		switch key.String() {
		case "k":
			m.cursor = clamp(m.cursor-1, 0, last)
			return m, nil
		case "j":
			m.cursor = clamp(m.cursor+1, 0, last)
			return m, nil
		case "g":
			m.cursor = 0
			return m, nil
		case "G":
			m.cursor = clamp(last, 0, last)
			return m, nil
		case "L":
			m.section, m.cursor, m.query = (m.section+1)%3, 0, ""
			return m, nil
		case "H":
			m.section, m.cursor, m.query = (m.section+2)%3, 0, ""
			return m, nil
		case "l":
			return m.open()
		}
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = true
	case "V":
		m.toggleVim()
	case "/":
		m.searching = true
	case "esc":
		m.query, m.cursor = "", 0
	case "tab", "]":
		m.section, m.cursor, m.query = (m.section+1)%3, 0, ""
	case "shift+tab", "[":
		m.section, m.cursor, m.query = (m.section+2)%3, 0, ""
	case "up":
		m.cursor = clamp(m.cursor-1, 0, last)
	case "down":
		m.cursor = clamp(m.cursor+1, 0, last)
	case "ctrl+u":
		m.cursor = clamp(m.cursor-10, 0, last)
	case "ctrl+d":
		m.cursor = clamp(m.cursor+10, 0, last)
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = clamp(last, 0, last)
	case "enter", "right":
		return m.open()
	}
	return m, nil
}

// toggleVim flips vim mode and persists the choice. The receiver is a pointer so
// the caller's model sees the new state; persistence is best-effort.
func (m *model) toggleVim() {
	m.vim = !m.vim
	_ = config.Save(config.Config{Vim: m.vim})
}

func clamp(v, lo, hi int) int {
	switch {
	case hi < lo:
		return lo
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}

// updateSearch handles keys while the search box is active: typing refines the
// query, arrows move within the matches, enter opens the match, esc cancels.
func (m model) updateSearch(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.searching, m.query, m.cursor = false, "", 0
	case "enter":
		m.searching = false
		return m.open()
	case "backspace":
		if m.query != "" {
			m.query, m.cursor = m.query[:len(m.query)-1], 0
		}
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.visibleItems())-1 {
			m.cursor++
		}
	default:
		if len(key.Runes) > 0 {
			m.query, m.cursor = m.query+string(key.Runes), 0
		}
	}
	return m, nil
}

// open acts on the selected item: a tool opens its argument form, a resource is
// read, and a prompt is rendered.
func (m model) open() (tea.Model, tea.Cmd) {
	i := m.selected()
	if i < 0 {
		return m, nil
	}
	switch m.section {
	case secTools:
		return m.openForm(m.tools[i]), textinput.Blink
	case secResources:
		m.formTool = nil
		r := m.resources[i]
		return m.dispatch(r.URI, m.readResource(r.URI))
	default:
		m.formTool = nil
		p := m.prompts[i]
		return m.dispatch(p.Name, m.getPrompt(p.Name))
	}
}

// visibleItems is the indices of the active section's items matching the search
// query, or all of them when the query is empty.
func (m model) visibleItems() []int {
	q := strings.ToLower(m.query)
	var idx []int
	for i := 0; i < m.count(m.section); i++ {
		if q == "" || strings.Contains(strings.ToLower(m.itemLabel(m.section, i)), q) {
			idx = append(idx, i)
		}
	}
	return idx
}

// selected returns the real item index under the cursor, or -1 when nothing is
// visible.
func (m model) selected() int {
	vis := m.visibleItems()
	if m.cursor >= len(vis) {
		return -1
	}
	return vis[m.cursor]
}

func (m model) count(s section) int {
	switch s {
	case secTools:
		return len(m.tools)
	case secResources:
		return len(m.resources)
	default:
		return len(m.prompts)
	}
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	// A pending yank is prefixed to the frame so the OSC52 sequence is written
	// through bubbletea's own output rather than racing it on stdout.
	return m.yankSeq + m.screenView()
}

func (m model) screenView() string {
	if m.showHelp {
		return m.helpView()
	}
	if m.showTrace {
		return m.viewTrace()
	}
	switch m.screen {
	case form:
		return m.viewForm()
	case result:
		return m.viewResult()
	default:
		return m.viewBrowse()
	}
}

// helpView lists every keybind, the same in any screen.
func (m model) helpView() string {
	binds := [][2]string{
		{"j / k   ↓ / ↑", "move up and down"},
		{"g / G", "jump to top / bottom"},
		{"ctrl+d / ctrl+u", "half page down / up"},
		{"enter / l / →", "open item · expand a result row"},
		{"esc / h / ←", "go back · collapse a row"},
		{"tab / shift+tab", "next / previous section"},
		{"/", "search the current list"},
		{"r", "re-run a result"},
		{"t", "toggle insight / JSON result"},
		{"y", "copy the result to the clipboard"},
		{"e", "edit a call's arguments"},
		{"V", "turn vim motions on or off"},
		{"T", "show the protocol trace"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}
	intro := "Arrow keys always work; vim motions are on. Press V to turn them off."
	if !m.vim {
		intro = "Arrow keys move; vim motions are off. Press V to turn them on."
	}
	var b strings.Builder
	b.WriteString(dim.Render(intro) + "\n\n")
	for _, kb := range binds {
		b.WriteString(accent.Render(fmt.Sprintf("  %-18s", kb[0])) + dim.Render(kb[1]) + "\n")
	}
	body := lipgloss.NewStyle().Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Padding(1, 3).Render(b.String())
	return m.layout(m.header("keys", dim.Render("help")), body, m.rule()+"\n"+dim.Render("  any key to close"))
}

// bodyHeight is the rows between the header and footer.
func (m model) bodyHeight() int {
	if h := m.height - chromeHeight; h > 0 {
		return h
	}
	return 1
}

// layout stacks the two-line header, the body, and the two-line footer so the
// frame fills the terminal and the footer rests on the last row.
func (m model) layout(header, body, footer string) string {
	return header + "\n" + body + "\n" + footer
}

func (m model) viewBrowse() string {
	bh := m.bodyHeight()
	left := m.leftPane(bh)
	right := m.rightPane(bh, lipgloss.Width(left))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return m.layout(m.headerView(), body, m.footerView())
}

func (m model) headerView() string {
	return m.header(m.server, green.Render("●")+dim.Render(" connected"))
}

func (m model) header(title, right string) string {
	left := "  " + accent.Bold(true).Render("mctop") + dim.Render("  ·  "+title)
	return m.spread(left, right+"  ") + "\n" + m.rule()
}

// spread places left and right text on one row, filling the middle.
func (m model) spread(left, right string) string {
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) rule() string { return dim.Render(strings.Repeat("─", m.width)) }

func (m model) leftWidth() int {
	w := m.width / 3
	switch {
	case w < 22:
		w = 22
	case w > 36:
		w = 36
	}
	if w > m.width-12 {
		w = m.width - 12
	}
	if w < 8 {
		w = 8
	}
	return w
}

// leftPane is the section list, bounded to the body height with a hairline
// divider on its right edge.
func (m model) leftPane(height int) string {
	return lipgloss.NewStyle().
		Width(m.leftWidth()).
		Height(height).
		MaxHeight(height).
		Padding(0, 2, 0, 1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(colBorder).
		Render(m.sectionList(height))
}

// sectionList renders the three section headers and, under the active one, a
// window of its items sized to fit the available rows.
func (m model) sectionList(height int) string {
	reserved := 3 // one line per section header
	if m.searching || m.query != "" {
		reserved++
	}
	itemRows := height - reserved
	if itemRows < 1 {
		itemRows = 1
	}

	var lines []string
	for s := secTools; s <= secPrompts; s++ {
		lines = append(lines, sectionHeader(s, m.count(s), s == m.section))
		if s != m.section {
			continue
		}
		if m.searching || m.query != "" {
			lines = append(lines, accent.Render("/"+m.query)+dim.Render("▏"))
		}
		lines = append(lines, m.itemRows(itemRows)...)
	}
	return strings.Join(lines, "\n")
}

func sectionHeader(s section, count int, active bool) string {
	name := strings.ToUpper(s.String())
	cnt := dim.Render(fmt.Sprintf(" (%d)", count))
	if active {
		return accent.Bold(true).Render(name) + cnt
	}
	return dim.Render(name) + cnt
}

func (m model) itemRows(maxRows int) []string {
	vis := m.visibleItems()
	if len(vis) == 0 {
		if m.query != "" {
			return []string{dim.Render("  no matches")}
		}
		return []string{dim.Render("  none")}
	}
	slice, top := windowItems(vis, m.cursor, maxRows)
	labelW := m.leftWidth() - 2
	rows := make([]string, 0, len(slice))
	for off, idx := range slice {
		label := truncate(m.itemLabel(m.section, idx), labelW)
		if top+off == m.cursor {
			rows = append(rows, barS.Render("▌")+" "+accent.Bold(true).Render(label))
		} else {
			rows = append(rows, "  "+label)
		}
	}
	return rows
}

func (m model) itemLabel(s section, i int) string {
	switch s {
	case secTools:
		return m.tools[i].Name
	case secResources:
		return m.resources[i].URI
	default:
		return m.prompts[i].Name
	}
}

// rightPane shows the selected item's details, bounded to the body height and
// wrapped to the remaining width.
func (m model) rightPane(height, leftWidth int) string {
	w := m.width - leftWidth - 3
	if w < 8 {
		w = 8
	}
	content := m.sectionHelp()
	if i := m.selected(); i >= 0 {
		content = m.detail(m.section, i)
	}
	return lipgloss.NewStyle().Width(w).Height(height).MaxHeight(height).Padding(1, 3).Render(content)
}

// sectionHelp explains the active section when it has nothing selected, so an
// empty Resources or Prompts pane says what it is instead of looking broken.
func (m model) sectionHelp() string {
	switch m.section {
	case secTools:
		return bold.Render("Tools") + "\n\n" + dim.Render("Functions the server can run for you. This one exposes none.")
	case secResources:
		return bold.Render("Resources") + "\n\n" + dim.Render("Readable data the server exposes (files, records, documents) that an agent can pull in as context. This server exposes none.")
	default:
		return bold.Render("Prompts") + "\n\n" + dim.Render("Prepared prompt templates the server offers; open one to render it. This server exposes none.")
	}
}

func (m model) detail(s section, i int) string {
	switch s {
	case secTools:
		return m.toolDetail(m.tools[i])
	case secResources:
		r := m.resources[i]
		head := bold.Render(r.URI)
		if r.MIMEType != "" {
			head += "\n" + dim.Render(r.MIMEType)
		}
		return head + "\n\n" + r.Description + "\n\n" + dim.Render("enter") + dim.Render(" to read")
	default:
		p := m.prompts[i]
		return bold.Render(p.Name) + "\n\n" + p.Description + "\n\n" + accent.Render("enter") + dim.Render(" to render")
	}
}

func (m model) toolDetail(t *sdk.Tool) string {
	var b strings.Builder
	b.WriteString(accent.Bold(true).Render(t.Name))
	if t.Description != "" {
		b.WriteString("\n\n" + t.Description)
	}
	if args := toolArgs(t); len(args) > 0 {
		b.WriteString("\n\n" + dim.Render("ARGUMENTS"))
		for _, a := range args {
			name := bold.Render(a.Name)
			if a.Required {
				name += accent.Render("*")
			}
			b.WriteString("\n\n" + name + "  " + dim.Render(a.Type))
			if a.Desc != "" {
				b.WriteString("\n" + dim.Render(a.Desc))
			}
			if len(a.Enum) > 0 {
				b.WriteString("\n" + dim.Render("one of: ") + dim.Render(strings.Join(a.Enum, ", ")))
			}
			if a.Default != "" {
				b.WriteString("\n" + dim.Render("default: "+a.Default))
			}
		}
	}
	b.WriteString("\n\n" + accent.Render("enter") + dim.Render(" to call"))
	return b.String()
}

func (m model) footerView() string {
	left := "  " + dim.Render("enter open  ·  / search  ·  tab section  ·  T trace  ·  ? keys")
	if m.searching {
		left = "  " + accent.Render("/"+m.query) + dim.Render("   enter open  ·  esc cancel")
	}
	badge := dim.Render("vim off")
	if m.vim {
		badge = accent.Render("vim on")
	}
	right := badge
	if n := len(m.visibleItems()); n > 0 {
		right += "   " + dim.Render(fmt.Sprintf("%d/%d", m.cursor+1, n))
	}
	return m.rule() + "\n" + m.spread(left, right+"  ")
}

// windowItems returns the slice of items visible around the cursor and the index
// it starts at, so a long list scrolls to keep the cursor on screen.
func windowItems(vis []int, cursor, rows int) (slice []int, top int) {
	if rows <= 0 || len(vis) == 0 {
		return nil, 0
	}
	if len(vis) <= rows {
		return vis, 0
	}
	top = cursor - rows/2
	if top < 0 {
		top = 0
	}
	if top+rows > len(vis) {
		top = len(vis) - rows
	}
	return vis[top : top+rows], top
}

// truncate shortens s to w columns with an ellipsis, so list rows never wrap.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}
