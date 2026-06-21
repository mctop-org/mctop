package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aloki-alok/mctop/internal/mcp"
	"github.com/aloki-alok/mctop/internal/tui"
)

// TUI connects to a target and opens the interactive browser. It is what a bare
// "mctop <target>" runs.
func TUI(args []string) int {
	opts, rest, err := extractConn(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mctop <target>")
		return 2
	}
	target := strings.Join(rest, " ")

	// ctx lives for the whole session and owns in-session calls, so it is only
	// cancelled when the program exits.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts.Headers = withAuth(ctx, target, opts.Headers)

	// Bound the initial connect by wall clock. The SDK's transports detach the
	// context handed to Connect (so a live session survives a connect deadline),
	// which also means a stalled handshake does not honor that deadline and would
	// otherwise freeze the TUI forever (the DESIGN's cardinal failure). Race the
	// connect against a timer and bail with an error instead.
	fmt.Fprintf(os.Stderr, "connecting to %s ...\n", target)
	type dialResult struct {
		client *mcp.Client
		err    error
	}
	dialed := make(chan dialResult, 1)
	go func() {
		c, e := mcp.Connect(ctx, target, opts)
		dialed <- dialResult{c, e}
	}()

	var client *mcp.Client
	select {
	case d := <-dialed:
		client, err = d.client, d.err
	case <-time.After(dialTimeout):
		fmt.Fprintf(os.Stderr, "mctop: timed out connecting to %s after %s\n", target, dialTimeout)
		return 1
	}
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
