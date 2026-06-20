package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

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

	// ctx lives for the whole session and owns the client connection, so it must
	// not be cancelled until the program exits.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mcp.Connect(ctx, target, mcp.Options{Headers: withAuth(ctx, target, headers)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		hintLogin(target, err)
		return 1
	}
	defer client.Close()

	// Only request resources and prompts when the server advertises them: asking
	// a server that does not can return an error that closes the session.
	loadCtx, loadCancel := context.WithTimeout(ctx, dialTimeout)
	tools, err := client.Tools(loadCtx)
	var resources []*sdk.Resource
	var prompts []*sdk.Prompt
	if client.HasResources() {
		resources, _ = client.Resources(loadCtx)
	}
	if client.HasPrompts() {
		prompts, _ = client.Prompts(loadCtx)
	}
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
