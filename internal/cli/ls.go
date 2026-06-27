package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mctop-org/mctop/internal/mcp"
	"github.com/mctop-org/mctop/internal/ui"
)

// LS connects to the target and prints its tools, resources, and prompts.
func LS(args []string) int {
	opts, rest, err := extractConn(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mctop ls <target> [--sse] [-H \"Name: value\"]")
		return 2
	}
	target := strings.Join(rest, " ")

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	opts.Headers = withAuth(ctx, target, opts.Headers)
	client, err := mcp.Connect(ctx, target, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		hintLogin(target, err)
		return 1
	}
	defer client.Close()

	s := ui.For(os.Stdout)
	if name, version := client.Server(); name != "" {
		fmt.Printf("%s %s\n", s.Bold(name), s.Dim(version))
	}

	tools, err := client.Tools(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop: list tools:", err)
		return 1
	}
	rows := make([][2]string, len(tools))
	for i, t := range tools {
		rows[i] = [2]string{t.Name, firstLine(t.Description)}
	}
	printSection(s, "tools", rows)

	// Only request resources and prompts when advertised: asking a server that
	// lacks them can error and close the session.
	if client.HasResources() {
		if resources, err := client.Resources(ctx); err == nil && len(resources) > 0 {
			rows := make([][2]string, len(resources))
			for i, r := range resources {
				rows[i] = [2]string{r.URI, firstLine(r.Description)}
			}
			printSection(s, "resources", rows)
		}
	}

	if client.HasPrompts() {
		if prompts, err := client.Prompts(ctx); err == nil && len(prompts) > 0 {
			rows := make([][2]string, len(prompts))
			for i, p := range prompts {
				rows[i] = [2]string{p.Name, firstLine(p.Description)}
			}
			printSection(s, "prompts", rows)
		}
	}

	return 0
}

// printSection prints an accented "label (n)" header followed by one aligned
// "name  description" row each, padding on the plain name so styling does not
// skew the column.
func printSection(s ui.Style, label string, rows [][2]string) {
	fmt.Printf("\n%s\n", s.Accent(fmt.Sprintf("%s (%d)", label, len(rows))))
	for _, r := range rows {
		gap := 24 - len(r[0])
		if gap < 1 {
			gap = 1
		}
		fmt.Printf("  %s%s%s\n", s.Bold(r[0]), strings.Repeat(" ", gap), s.Dim(r[1]))
	}
}

// firstLine keeps listings to one row per item even when descriptions wrap.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
