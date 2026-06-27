package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mctop/mctop/internal/upgrade"
)

// Upgrade replaces the running binary with the latest release. version is this
// build's version, used to skip the download when already current.
func Upgrade(version string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("checking for updates...")
	tag, updated, err := upgrade.Run(ctx, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mctop:", err)
		return 1
	}
	if !updated {
		fmt.Println("already up to date (" + tag + ")")
		return 0
	}
	fmt.Println("updated to " + tag)
	return 0
}
