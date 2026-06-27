// Package cli implements mctop's headless subcommands. Each returns a process
// exit code; main dispatches to them.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mctop/mctop/internal/mcp"
)

// dialTimeout bounds connecting to and querying a server, so a stuck server
// never hangs the command.
const dialTimeout = 30 * time.Second

// loginTimeout bounds the interactive OAuth flow, which waits on a human in a
// browser, so it is far longer than dialTimeout.
const loginTimeout = 5 * time.Minute

// extractConn pulls connection flags out of args and returns them as Options
// alongside the remaining positional arguments, so every subcommand accepts auth
// headers (a bearer token is just -H "Authorization: Bearer ...") and --sse
// without each reimplementing flag scanning.
func extractConn(args []string) (opts mcp.Options, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--sse":
			opts.SSE = true
			continue
		case "-H", "--header":
			if i+1 >= len(args) {
				return mcp.Options{}, nil, fmt.Errorf("%s needs a \"Name: value\" argument", arg)
			}
			name, value, ok := strings.Cut(args[i+1], ":")
			if !ok {
				return mcp.Options{}, nil, fmt.Errorf("header %q must be \"Name: value\"", args[i+1])
			}
			if opts.Headers == nil {
				opts.Headers = make(map[string]string)
			}
			opts.Headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
			i++
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}
