package tui

import (
	"fmt"
	"sort"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Arg is one tool input parameter, flattened from the tool's JSON schema for
// display in the details pane and the call form.
type Arg struct {
	Name     string
	Type     string
	Desc     string
	Required bool
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
