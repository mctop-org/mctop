package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mctop-org/mctop/internal/mcp"
	"github.com/mctop-org/mctop/internal/spec"
	"github.com/mctop-org/mctop/internal/ui"
)

// Test runs a contract spec against its server and exits 0 when every check
// passes, 1 when any fails, so it can gate CI.
func Test(args []string) int {
	path, asJSON, err := parseTestArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usage: mctop test <spec.yaml> [--report json]")
		return 2
	}

	s, err := spec.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	client, err := mcp.Connect(ctx, s.Server.Target(), mcp.Options{Headers: withAuth(ctx, s.Server.Target(), s.Server.Headers), SSE: s.Server.SSE})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	defer client.Close()

	results := spec.Run(ctx, client, s)
	if failed := report(results, asJSON); failed > 0 {
		return 1
	}
	return 0
}

func parseTestArgs(args []string) (path string, asJSON bool, err error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--report" {
			if i+1 >= len(args) || args[i+1] != "json" {
				return "", false, fmt.Errorf("--report only supports json")
			}
			asJSON = true
			i++
			continue
		}
		if path != "" {
			return "", false, fmt.Errorf("unexpected argument %q", args[i])
		}
		path = args[i]
	}
	if path == "" {
		return "", false, fmt.Errorf("missing spec path")
	}
	return path, asJSON, nil
}

func report(results []spec.Result, asJSON bool) (failed int) {
	for _, r := range results {
		if !r.Pass {
			failed++
		}
	}

	if asJSON {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))
		return failed
	}

	s := ui.For(os.Stdout)
	for _, r := range results {
		mark := s.Green("PASS")
		if !r.Pass {
			mark = s.Red("FAIL")
		}
		fmt.Printf("  %s  %s  %s\n", mark, r.Name, s.Dim("("+r.Detail+")"))
	}
	summary := fmt.Sprintf("%d passed, %d failed", len(results)-failed, failed)
	if failed > 0 {
		summary = s.Red(summary)
	} else {
		summary = s.Green(summary)
	}
	fmt.Printf("\n%s\n", summary)
	return failed
}
