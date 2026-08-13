package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecorderBuildsSpec(t *testing.T) {
	r := NewRecorder("https://mcp.example.com/mcp", nil, false)
	r.Record("echo", map[string]any{"text": "hi"}, false)
	r.Record("add", map[string]any{"a": 1.0, "b": 2.0}, false)
	r.Record("echo", map[string]any{"text": "again"}, false)

	s := r.Spec()
	if s.Server.URL != "https://mcp.example.com/mcp" || s.Server.Command != "" {
		t.Fatalf("http target should record as url, got %+v", s.Server)
	}
	if len(s.Calls) != 3 {
		t.Fatalf("want 3 calls, got %d", len(s.Calls))
	}
	want := []string{"echo", "add"}
	if len(s.Expect.Tools) != 2 || s.Expect.Tools[0] != want[0] || s.Expect.Tools[1] != want[1] {
		t.Fatalf("expect.tools should be unique in first-call order, got %v", s.Expect.Tools)
	}
}

func TestRecorderCommandTarget(t *testing.T) {
	r := NewRecorder("uvx mcp-server-time", nil, false)
	if s := r.Spec(); s.Server.Command != "uvx mcp-server-time" || s.Server.URL != "" {
		t.Fatalf("spawn target should record as command, got %+v", s.Server)
	}
}

func TestRecorderRedactsHeaders(t *testing.T) {
	r := NewRecorder("https://x.dev/mcp", map[string]string{
		"Authorization": "Bearer sekret",
		"X-Api-Key":     "sekret2",
	}, false)
	s := r.Spec()
	if got := s.Server.Headers["Authorization"]; got != "$AUTHORIZATION" {
		t.Errorf("Authorization: want $AUTHORIZATION, got %q", got)
	}
	if got := s.Server.Headers["X-Api-Key"]; got != "$X_API_KEY" {
		t.Errorf("X-Api-Key: want $X_API_KEY, got %q", got)
	}
}

func TestRecorderCapturesErrorStatus(t *testing.T) {
	r := NewRecorder("https://x.dev/mcp", nil, false)
	r.Record("boom", nil, true)
	s := r.Spec()
	if s.Calls[0].Assert.NotError == nil || *s.Calls[0].Assert.NotError {
		t.Fatalf("an isError call should record not_error: false, got %+v", s.Calls[0].Assert)
	}
}

// The written file must parse back through the strict loader, so a recording
// is a valid contract as-is.
func TestRecorderWriteRoundTrips(t *testing.T) {
	r := NewRecorder("https://mcp.example.com/mcp", map[string]string{"Authorization": "Bearer s"}, true)
	r.Record("echo", map[string]any{"text": "hi"}, false)
	r.Record("boom", nil, true)

	path := filepath.Join(t.TempDir(), "rec.yaml")
	if err := r.Write(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTHORIZATION", "Bearer live")

	s, err := Load(path)
	if err != nil {
		data, _ := os.ReadFile(path)
		t.Fatalf("recorded spec should load strictly: %v\n%s", err, data)
	}
	if !s.Server.SSE || s.Server.URL == "" {
		t.Fatalf("server section lost in round trip: %+v", s.Server)
	}
	if s.Server.Headers["Authorization"] != "Bearer live" {
		t.Fatalf("header env reference should expand on load, got %q", s.Server.Headers["Authorization"])
	}
	if len(s.Calls) != 2 || s.Calls[0].Args["text"] != "hi" {
		t.Fatalf("calls lost in round trip: %+v", s.Calls)
	}
}
