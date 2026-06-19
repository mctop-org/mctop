// Command mctop is a terminal client for MCP servers: explore a server in a
// TUI, script it headless, or assert its contract in CI. See DESIGN.md.
package main

import (
	"fmt"
	"os"

	"github.com/aloki-alok/mctop/internal/cli"
)

// version is overridden at release time via -ldflags.
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
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
	case "record":
		fmt.Fprintf(os.Stderr, "mctop: %q is not implemented yet\n", args[0])
		return 2
	default:
		// A bare target opens the TUI; not implemented yet.
		fmt.Fprintf(os.Stderr, "mctop: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `mctop: a terminal client for MCP servers.

Usage:
  mctop <target>              open the TUI against a server
  mctop ls <target>           list tools, resources, and prompts
  mctop call <target> <tool>  call one tool and print the result
  mctop test <spec.yaml>      run a contract, exit 0 on pass, 1 on fail
  mctop login <url>           log in to an OAuth-protected server
  mctop logout <url>          forget a server's cached token

A target is a command to spawn ("uvx mcp-server-time") or an http(s):// URL.
`)
}
