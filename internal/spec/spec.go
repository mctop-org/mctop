// Package spec defines mctop's CI contract format and the engine that runs it
// against a connected server. Parsing and evaluation live here; the cli package
// owns connecting and printing.
package spec

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Spec is a server contract: where the server is, what it must expose, and the
// calls whose results must hold.
type Spec struct {
	Server Server `yaml:"server"`
	Expect Expect `yaml:"expect"`
	Calls  []Call `yaml:"calls"`
}

// Server locates the server under test. Exactly one of Command or URL is set.
// Headers are sent with each HTTP request; their values are expanded against the
// environment ($TOKEN, ${TOKEN}) so a secret stays out of the committed spec.
type Server struct {
	Command string            `yaml:"command"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

// Expect lists invariants about the server's surface.
type Expect struct {
	Tools []string `yaml:"tools"`
}

// Call invokes a tool and asserts on its result.
type Call struct {
	Tool   string         `yaml:"tool"`
	Args   map[string]any `yaml:"args"`
	Assert Assert         `yaml:"assert"`
}

// Assert holds the conditions a call's result must satisfy. NotError defaults to
// true when omitted, since a call is expected to succeed unless stated.
type Assert struct {
	NotError *bool  `yaml:"not_error"`
	Contains string `yaml:"contains"`
}

// Target returns the server target string the mcp client understands.
func (s Server) Target() string {
	if s.URL != "" {
		return s.URL
	}
	return s.Command
}

// Load reads and strictly parses a spec file, rejecting unknown keys so typos
// fail loudly instead of being silently ignored.
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var s Spec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for k, v := range s.Server.Headers {
		s.Server.Headers[k] = os.ExpandEnv(v)
	}
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

func (s Spec) validate() error {
	switch {
	case s.Server.Command == "" && s.Server.URL == "":
		return fmt.Errorf("server needs a command or a url")
	case s.Server.Command != "" && s.Server.URL != "":
		return fmt.Errorf("server has both command and url; set one")
	}
	for i, c := range s.Calls {
		if c.Tool == "" {
			return fmt.Errorf("calls[%d] is missing a tool", i)
		}
	}
	return nil
}
