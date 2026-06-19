// Package tui is mctop's interactive terminal client: connect to a server and
// move through it step by step, browse what it exposes, fill a tool's arguments,
// run it, and read the result, without leaving the keyboard.
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aloki-alok/mctop/internal/mcp"
)

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
	accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	bold    = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Faint(true)
	cursorS = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	red     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

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
}

// dispatch starts an action (tool call, resource read, prompt render), showing
// the result screen in a running state and remembering the command so r can
// re-run it.
func (m model) dispatch(title string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.running = true
	m.resultTitle = title
	m.lastCmd = cmd
	m.screen = result
	return m, cmd
}

// New builds the model from an already-connected client and its loaded surface.
func New(ctx context.Context, server string, client *mcp.Client, tools []*sdk.Tool, resources []*sdk.Resource, prompts []*sdk.Prompt) tea.Model {
	return model{ctx: ctx, server: server, client: client, tools: tools, resources: resources, prompts: prompts}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case callResultMsg:
		m.running = false
		m.output, m.resultErr, m.elapsed = msg.output, msg.err, msg.elapsed
		m.screen = result
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
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/":
		m.searching = true
	case "esc":
		m.query, m.cursor = "", 0
	case "tab":
		m.section, m.cursor, m.query = (m.section+1)%3, 0, ""
	case "shift+tab":
		m.section, m.cursor, m.query = (m.section+2)%3, 0, ""
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visibleItems())-1 {
			m.cursor++
		}
	case "enter", "right", "l":
		return m.open()
	}
	return m, nil
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
		return m.openForm(m.tools[i]), nil
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
	if m.width == 0 {
		return "connecting..."
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

func (m model) viewBrowse() string {
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.leftPane(), m.rightPane())
	return strings.Join([]string{m.headerView(), body, m.footerView()}, "\n")
}

func (m model) headerView() string {
	return m.header(m.server, accent.Render("● ")+dim.Render("connected"))
}

func (m model) header(title, right string) string {
	left := accent.Render("mctop") + dim.Render(" · "+title)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right + "\n" + dim.Render(strings.Repeat("─", m.width))
}

func (m model) rule() string { return dim.Render(strings.Repeat("─", m.width)) }

func (m model) leftPane() string {
	var b strings.Builder
	for s := secTools; s <= secPrompts; s++ {
		label := fmt.Sprintf("%s (%d)", strings.ToUpper(s.String()), m.count(s))
		if s == m.section {
			b.WriteString(accent.Render(label) + "\n")
			if m.searching || m.query != "" {
				b.WriteString(dim.Render("  /"+m.query) + "\n")
			}
			b.WriteString(m.itemList())
		} else {
			b.WriteString(dim.Render(label) + "\n")
		}
	}
	return lipgloss.NewStyle().Width(34).Render(b.String())
}

func (m model) itemList() string {
	var b strings.Builder
	for pos, i := range m.visibleItems() {
		prefix := "  "
		label := m.itemLabel(m.section, i)
		if pos == m.cursor {
			prefix = cursorS.Render("▸ ")
			label = bold.Render(label)
		}
		b.WriteString(prefix + label + "\n")
	}
	return b.String()
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

func (m model) rightPane() string {
	width := m.width - 36
	if width < 10 {
		width = 10
	}
	style := lipgloss.NewStyle().Width(width).PaddingLeft(2)
	i := m.selected()
	if i < 0 {
		return style.Render(dim.Render("nothing here"))
	}
	return style.Render(m.detail(m.section, i))
}

func (m model) detail(s section, i int) string {
	switch s {
	case secTools:
		return m.toolDetail(m.tools[i])
	case secResources:
		r := m.resources[i]
		return bold.Render(r.URI) + "\n" + dim.Render(r.MIMEType) + "\n\n" + r.Description
	default:
		p := m.prompts[i]
		return bold.Render(p.Name) + "\n\n" + p.Description
	}
}

func (m model) toolDetail(t *sdk.Tool) string {
	var b strings.Builder
	b.WriteString(bold.Render(t.Name) + "\n")
	if t.Description != "" {
		b.WriteString(t.Description + "\n")
	}
	args := toolArgs(t)
	if len(args) > 0 {
		b.WriteString("\n" + accent.Render("arguments") + "\n")
		for _, a := range args {
			name := a.Name
			if a.Required {
				name += "*"
			}
			b.WriteString(fmt.Sprintf("  %-16s %s  %s\n", bold.Render(name), dim.Render(a.Type), dim.Render(a.Desc)))
		}
	}
	return b.String()
}

func (m model) footerView() string {
	if m.searching {
		return m.rule() + "\n" + accent.Render("  /"+m.query) + dim.Render("   enter open   esc cancel")
	}
	return m.rule() + "\n" + dim.Render("  ↑↓ move   enter open   / search   tab section   q quit")
}
