package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordNeedsTarget(t *testing.T) {
	if got := Record(nil); got != 2 {
		t.Fatalf("no target should exit 2, got %d", got)
	}
}

func TestRecordOutNeedsValue(t *testing.T) {
	if got := Record([]string{"http://x/mcp", "-o"}); got != 2 {
		t.Fatalf("-o without a path should exit 2, got %d", got)
	}
}

func TestRecordRefusesExistingSpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.yaml")
	if err := os.WriteFile(path, []byte("server:\n  url: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Record([]string{"http://x/mcp", "-o", path}); got != 2 {
		t.Fatalf("existing spec should be refused up front, got %d", got)
	}
}
