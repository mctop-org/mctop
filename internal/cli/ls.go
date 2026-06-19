package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aloki-alok/mctop/internal/mcp"
)

// LS connects to the target and prints its tools, resources, and prompts.
func LS(args []string) int {
	headers, rest, err := extractHeaders(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mctop ls <target> [-H \"Name: value\"]")
		return 2
	}
	target := strings.Join(rest, " ")

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	client, err := mcp.Connect(ctx, target, mcp.Options{Headers: headers})
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	defer client.Close()

	if name, version := client.Server(); name != "" {
		fmt.Printf("%s %s\n", name, version)
	}

	tools, err := client.Tools(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop: list tools:", err)
		return 1
	}
	fmt.Printf("\ntools (%d)\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  %-24s %s\n", t.Name, firstLine(t.Description))
	}

	resources, err := client.Resources(ctx)
	if err == nil && len(resources) > 0 {
		fmt.Printf("\nresources (%d)\n", len(resources))
		for _, r := range resources {
			fmt.Printf("  %-24s %s\n", r.URI, firstLine(r.Description))
		}
	}

	prompts, err := client.Prompts(ctx)
	if err == nil && len(prompts) > 0 {
		fmt.Printf("\nprompts (%d)\n", len(prompts))
		for _, p := range prompts {
			fmt.Printf("  %-24s %s\n", p.Name, firstLine(p.Description))
		}
	}

	return 0
}

// firstLine keeps listings to one row per item even when descriptions wrap.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
