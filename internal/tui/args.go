package tui

import (
	"fmt"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Arg is one tool input parameter, flattened from the tool's JSON schema for
// display in the details pane and the call form.
type Arg struct {
	Name     string
	Type     string
	Desc     string
	Required bool
	Enum     []string // allowed values, when the schema constrains them
	Default  string   // the schema's default, rendered for display
	Format   string   // a string format hint (e.g. date-time, uri, email)
}

// hint is the most useful thing to show in an empty field: the allowed values,
// then the default, then the format. It is empty when the schema says nothing
// extra, since the type already labels the field.
func (a Arg) hint() string {
	switch {
	case len(a.Enum) > 0:
		return strings.Join(a.Enum, " | ")
	case a.Default != "":
		return "default " + a.Default
	case a.Format != "":
		return a.Format
	default:
		return ""
	}
}

// toolArgs reads a tool's input schema into an ordered argument list: required
// parameters first in their declared order, then the rest alphabetically, since
// the schema arrives as an unordered object.
func toolArgs(t *sdk.Tool) []Arg {
	schema, _ := t.InputSchema.(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return nil
	}

	required := map[string]bool{}
	var order []string
	if list, ok := schema["required"].([]any); ok {
		for _, r := range list {
			if name, ok := r.(string); ok && props[name] != nil {
				required[name] = true
				order = append(order, name)
			}
		}
	}

	var rest []string
	for name := range props {
		if !required[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)

	args := make([]Arg, 0, len(order))
	for _, name := range order {
		p, _ := props[name].(map[string]any)
		args = append(args, Arg{
			Name:     name,
			Type:     asString(p["type"]),
			Desc:     asString(p["description"]),
			Required: required[name],
			Enum:     asStrings(p["enum"]),
			Default:  asString(p["default"]),
			Format:   asString(p["format"]),
		})
	}
	return args
}

// promptArgs flattens a prompt's argument list for the details pane and the
// form. Prompt arguments are plain named strings (no JSON schema), so the type
// is always string and the server's declared order is kept.
func promptArgs(p *sdk.Prompt) []Arg {
	args := make([]Arg, 0, len(p.Arguments))
	for _, a := range p.Arguments {
		args = append(args, Arg{
			Name:     a.Name,
			Type:     "string",
			Desc:     a.Description,
			Required: a.Required,
		})
	}
	return args
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// asStrings flattens a JSON enum array to display strings, skipping a nil entry.
func asStrings(v any) []string {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e != nil {
			out = append(out, fmt.Sprint(e))
		}
	}
	return out
}
