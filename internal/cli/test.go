package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aloki-alok/mctop/internal/mcp"
	"github.com/aloki-alok/mctop/internal/spec"
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

	client, err := mcp.Connect(ctx, s.Server.Target(), mcp.Options{})
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

	for _, r := range results {
		mark := "PASS"
		if !r.Pass {
			mark = "FAIL"
		}
		fmt.Printf("  %s  %s  (%s)\n", mark, r.Name, r.Detail)
	}
	fmt.Printf("\n%d passed, %d failed\n", len(results)-failed, failed)
	return failed
}
