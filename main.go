// Command mctop is a terminal client for MCP servers: explore a server in a
// TUI, script it headless, or assert its contract in CI. See DESIGN.md.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mctop-org/mctop/internal/cli"
	"github.com/mctop-org/mctop/internal/ui"
)

// version is overridden at release time via -ldflags.
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		welcome(os.Stdout)
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("mctop", version)
		return 0
	case "help", "--help", "-h":
		usage(os.Stdout)
		return 0
	case "ls":
		return cli.LS(args[1:])
	case "call":
		return cli.Call(args[1:])
	case "test":
		return cli.Test(args[1:])
	case "login":
		return cli.Login(args[1:])
	case "logout":
		return cli.Logout(args[1:])
	case "upgrade":
		return cli.Upgrade(version)
	case "record":
		fmt.Fprintf(os.Stderr, "mctop: %q is not implemented yet\n", args[0])
		return 2
	default:
		// A bare http(s):// URL or spawn command opens the interactive TUI.
		return cli.TUI(args)
	}
}

// welcome is the friendly first-run screen for a bare invocation: the wordmark
// and a short quickstart, rather than a wall of usage.
func welcome(w *os.File) {
	s := ui.For(w)
	fmt.Fprintln(w, s.Banner())
	fmt.Fprintf(w, "\n%s  %s  %s\n\n", s.Dim("a terminal client for MCP servers"), s.Dim("·"), s.Dim(version))
	fmt.Fprintln(w, s.Accent("quickstart"))
	fmt.Fprintln(w, quickstart(s, "mctop ls uvx mcp-server-time", "explore a server"))
	fmt.Fprintln(w, quickstart(s, "mctop login <url>", "log in to an OAuth server"))
	fmt.Fprintln(w, quickstart(s, "mctop call <url> <tool> k=v", "call a tool"))
	fmt.Fprintln(w, quickstart(s, "mctop test spec.yaml", "gate a server in CI"))
	fmt.Fprintf(w, "\nrun %s for all commands.\n", s.Bold("mctop help"))
}

// quickstart renders one aligned "command  description" row, padding on the
// plain command so ANSI codes never throw off the column.
func quickstart(s ui.Style, command, desc string) string {
	const width = 30
	gap := width - len(command)
	if gap < 1 {
		gap = 1
	}
	return "  " + s.Bold(command) + strings.Repeat(" ", gap) + s.Dim(desc)
}

func usage(w *os.File) {
	s := ui.For(w)
	fmt.Fprintln(w, s.Banner())
	fmt.Fprint(w, `
Usage:
  mctop <target>              open the TUI against a server
  mctop ls <target>           list tools, resources, and prompts
  mctop call <target> <tool>  call one tool and print the result
  mctop test <spec.yaml>      run a contract, exit 0 on pass, 1 on fail
  mctop login <url>           log in to an OAuth-protected server
  mctop logout <url>          forget a server's cached token
  mctop upgrade               update mctop to the latest release

A target is a command to spawn ("uvx mcp-server-time") or an http(s):// URL.
`)
}
