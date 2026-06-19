package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aloki-alok/mctop/internal/mcp"
)

// Call invokes one tool and prints its result. Arguments are given as key=value
// pairs (each value parsed as JSON, falling back to a string) or as a single
// --json object. A tool that reports an error exits non-zero.
func Call(args []string) int {
	headers, rest, err := extractHeaders(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}
	if len(rest) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mctop call <target> <tool> [key=value ...] | --json '<object>' [-H \"Name: value\"]")
		return 2
	}
	target, tool := rest[0], rest[1]

	toolArgs, err := parseToolArgs(rest[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	client, err := mcp.Connect(ctx, target, mcp.Options{Headers: withAuth(ctx, target, headers)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		hintLogin(target, err)
		return 1
	}
	defer client.Close()

	res, err := client.Call(ctx, tool, toolArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop: call:", err)
		return 1
	}

	out := mcp.RenderContent(res.Content)
	if res.IsError {
		fmt.Fprintln(os.Stderr, out)
		return 1
	}
	fmt.Println(out)
	return 0
}

func parseToolArgs(args []string) (map[string]any, error) {
	if len(args) == 0 {
		return nil, nil
	}
	if args[0] == "--json" {
		if len(args) != 2 {
			return nil, fmt.Errorf("--json takes exactly one object argument")
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(args[1]), &m); err != nil {
			return nil, fmt.Errorf("invalid --json: %w", err)
		}
		return m, nil
	}

	m := make(map[string]any, len(args))
	for _, pair := range args {
		key, raw, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("argument %q must be key=value", pair)
		}
		var val any
		if err := json.Unmarshal([]byte(raw), &val); err != nil {
			val = raw
		}
		m[key] = val
	}
	return m, nil
}
