package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mctop-org/mctop/internal/oauth"
)

// Login runs the OAuth flow for an http(s) MCP server and caches the token so
// later commands authenticate automatically.
func Login(args []string) int {
	if len(args) != 1 || !isHTTP(args[0]) {
		fmt.Fprintln(os.Stderr, "usage: mctop login <http(s) url>")
		return 2
	}
	target := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	creds, err := oauth.Login(ctx, target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	if err := oauth.Save(target, creds); err != nil {
		fmt.Fprintln(os.Stderr, "mctop: save credentials:", err)
		return 1
	}
	fmt.Println("logged in to", target)
	return 0
}

// Logout forgets the cached token for a server.
func Logout(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: mctop logout <http(s) url>")
		return 2
	}
	if err := oauth.Delete(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	fmt.Println("logged out of", args[0])
	return 0
}
