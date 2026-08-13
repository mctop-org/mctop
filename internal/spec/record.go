package spec

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Recorder accumulates the tool calls of a live session into a Spec, so an
// interactive run can be replayed as a CI contract. It is safe for concurrent
// Record calls, which arrive from the TUI's background commands.
type Recorder struct {
	mu     sync.Mutex
	server Server
	calls  []Call
}

// NewRecorder starts a recording against the given target. Header values are
// written as $NAME environment references, never their real values, so a
// recorded spec is committable without leaking a secret.
func NewRecorder(target string, headers map[string]string, sse bool) *Recorder {
	srv := Server{SSE: sse}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		srv.URL = target
	} else {
		srv.Command = target
	}
	if len(headers) > 0 {
		srv.Headers = make(map[string]string, len(headers))
		for name := range headers {
			srv.Headers[name] = "$" + envName(name)
		}
	}
	return &Recorder{server: srv}
}

// envName is the environment variable a header's value is expected in:
// "Authorization" becomes AUTHORIZATION, "X-Api-Key" becomes X_API_KEY.
func envName(header string) string {
	up := strings.ToUpper(header)
	return strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, up)
}

// Record appends one completed tool call. An isError call is captured with a
// not_error: false assertion, so replaying asserts what was actually observed.
func (r *Recorder) Record(tool string, args map[string]any, isError bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := Call{Tool: tool, Args: args}
	if isError {
		f := false
		c.Assert.NotError = &f
	}
	r.calls = append(r.calls, c)
}

// Len is how many calls have been recorded.
func (r *Recorder) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// Spec assembles the recording: the server, the called tools as expectations
// (unique, in first-call order), and the calls themselves.
func (r *Recorder) Spec() *Spec {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[string]bool)
	var tools []string
	for _, c := range r.calls {
		if !seen[c.Tool] {
			seen[c.Tool] = true
			tools = append(tools, c.Tool)
		}
	}
	return &Spec{Server: r.server, Expect: Expect{Tools: tools}, Calls: r.calls}
}

// Write saves the recording to path as a spec file mctop test runs. The
// recorded assertions only pin error status; sharpening them with contains is
// left to the author, since recorded output is often volatile.
func (r *Recorder) Write(path string) error {
	data, err := yaml.Marshal(r.Spec())
	if err != nil {
		return err
	}
	head := "# Recorded by mctop record. Each call asserts its observed error status;\n" +
		"# add assert.contains where it strengthens the contract.\n"
	if err := os.WriteFile(path, []byte(head+string(data)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
