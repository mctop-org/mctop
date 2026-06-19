package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aloki-alok/mctop/internal/mcp"
	"github.com/aloki-alok/mctop/internal/tui"
)

// TUI connects to a target and opens the interactive browser. It is what a bare
// "mctop <target>" runs.
func TUI(args []string) int {
	headers, rest, err := extractHeaders(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mctop <target>")
		return 2
	}
	target := strings.Join(rest, " ")

	// ctx lives for the whole session; the connect step gets its own deadline.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connectCtx, connectCancel := context.WithTimeout(ctx, dialTimeout)
	client, err := mcp.Connect(connectCtx, target, mcp.Options{Headers: withAuth(connectCtx, target, headers)})
	connectCancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		hintLogin(target, err)
		return 1
	}
	defer client.Close()

	// Resources and prompts are optional capabilities, so their errors are not
	// fatal: a server may expose only tools.
	loadCtx, loadCancel := context.WithTimeout(ctx, dialTimeout)
	tools, err := client.Tools(loadCtx)
	resources, _ := client.Resources(loadCtx)
	prompts, _ := client.Prompts(loadCtx)
	loadCancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop: list tools:", err)
		return 1
	}

	program := tea.NewProgram(tui.New(ctx, target, client, tools, resources, prompts), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	return 0
}
