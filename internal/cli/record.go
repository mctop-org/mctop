package cli

import (
	"fmt"
	"os"
)

// defaultRecordPath is where a recording lands when -o is not given.
const defaultRecordPath = "mctop.spec.yaml"

// Record opens the TUI against a target and captures the session's tool calls
// into a spec file for mctop test.
func Record(args []string) int {
	path := defaultRecordPath
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o", "--out":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "mctop: %s needs a file path\n", args[i])
				return 2
			}
			path = args[i+1]
			i++
		default:
			rest = append(rest, args[i])
		}
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mctop record <target> [-o spec.yaml]")
		return 2
	}
	// Refuse to clobber an existing spec up front, not after the user has done a
	// whole session: a hand-sharpened contract is exactly the file this would
	// silently destroy.
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "mctop: %s already exists; pick another with -o\n", path)
		return 2
	}
	return runTUI(rest, path)
}
