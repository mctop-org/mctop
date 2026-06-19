// Package tui is mctop's interactive terminal client: connect to a server and
// move through it step by step, browse what it exposes, fill a tool's arguments,
// run it, and read the result, without leaving the keyboard.
package tui

import (
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

var (
	accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	bold    = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Faint(true)
	cursorS = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
)

// model holds the connected session and the browse cursor. Later screens (form,
// result) extend this struct.
type model struct {
	server    string
	client    *mcp.Client
	tools     []*sdk.Tool
	resources []*sdk.Resource
	prompts   []*sdk.Prompt

	section section
	cursor  [3]int

	width, height int
}

// New builds the model from an already-connected client and its loaded surface.
func New(server string, client *mcp.Client, tools []*sdk.Tool, resources []*sdk.Resource, prompts []*sdk.Prompt) tea.Model {
	return model{server: server, client: client, tools: tools, resources: resources, prompts: prompts}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.section = (m.section + 1) % 3
		case "shift+tab", "left", "h":
			m.section = (m.section + 2) % 3
		case "up", "k":
			if m.cursor[m.section] > 0 {
				m.cursor[m.section]--
			}
		case "down", "j":
			if m.cursor[m.section] < m.count(m.section)-1 {
				m.cursor[m.section]++
			}
		}
	}
	return m, nil
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
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.leftPane(), m.rightPane())
	return strings.Join([]string{m.headerView(), body, m.footerView()}, "\n")
}

func (m model) headerView() string {
	left := accent.Render("mctop") + dim.Render(" · "+m.server)
	right := accent.Render("● ") + dim.Render("connected")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right + "\n" + dim.Render(strings.Repeat("─", m.width))
}

func (m model) leftPane() string {
	var b strings.Builder
	for s := secTools; s <= secPrompts; s++ {
		label := fmt.Sprintf("%s (%d)", strings.ToUpper(s.String()), m.count(s))
		if s == m.section {
			b.WriteString(accent.Render(label) + "\n")
			b.WriteString(m.itemList(s))
		} else {
			b.WriteString(dim.Render(label) + "\n")
		}
	}
	return lipgloss.NewStyle().Width(34).Render(b.String())
}

func (m model) itemList(s section) string {
	var b strings.Builder
	for i := 0; i < m.count(s); i++ {
		prefix := "  "
		label := m.itemLabel(s, i)
		if i == m.cursor[s] {
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
	if m.count(m.section) == 0 {
		return style.Render(dim.Render("nothing here"))
	}
	return style.Render(m.detail(m.section, m.cursor[m.section]))
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
	keys := "↑↓ move   tab section   q quit"
	return dim.Render(strings.Repeat("─", m.width)) + "\n" + dim.Render("  "+keys)
}
