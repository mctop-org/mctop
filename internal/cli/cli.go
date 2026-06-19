// Package cli implements mctop's headless subcommands. Each returns a process
// exit code; main dispatches to them.
package cli

import (
	"fmt"
	"strings"
	"time"
)

// dialTimeout bounds connecting to and querying a server, so a stuck server
// never hangs the command.
const dialTimeout = 30 * time.Second

// extractHeaders pulls repeated -H/--header "Name: value" flags out of args and
// returns them alongside the remaining positional arguments. It lets every
// subcommand accept auth headers (a bearer token is just -H "Authorization:
// Bearer ...") without each reimplementing flag scanning.
func extractHeaders(args []string) (headers map[string]string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg != "-H" && arg != "--header" {
			rest = append(rest, arg)
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("%s needs a \"Name: value\" argument", arg)
		}
		name, value, ok := strings.Cut(args[i+1], ":")
		if !ok {
			return nil, nil, fmt.Errorf("header %q must be \"Name: value\"", args[i+1])
		}
		if headers == nil {
			headers = make(map[string]string)
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
		i++
	}
	return headers, rest, nil
}
