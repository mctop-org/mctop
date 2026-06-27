package spec

import (
	"context"
	"fmt"
	"strings"

	"github.com/mctop/mctop/internal/mcp"
)

// Result is the outcome of one checked invariant.
type Result struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// Run evaluates a spec against a connected client and returns one Result per
// checked invariant, in order. It never returns early: a failure is recorded,
// not thrown, so the report covers the whole contract.
func Run(ctx context.Context, c *mcp.Client, s *Spec) []Result {
	var results []Result
	results = append(results, checkTools(ctx, c, s.Expect.Tools)...)
	for _, call := range s.Calls {
		results = append(results, runCall(ctx, c, call))
	}
	return results
}

func checkTools(ctx context.Context, c *mcp.Client, want []string) []Result {
	if len(want) == 0 {
		return nil
	}
	tools, err := c.Tools(ctx)
	if err != nil {
		return []Result{{Name: "list tools", Pass: false, Detail: err.Error()}}
	}
	present := make(map[string]bool, len(tools))
	for _, t := range tools {
		present[t.Name] = true
	}

	results := make([]Result, len(want))
	for i, name := range want {
		results[i] = Result{Name: "tool " + name, Pass: present[name], Detail: "missing"}
		if present[name] {
			results[i].Detail = "present"
		}
	}
	return results
}

func runCall(ctx context.Context, c *mcp.Client, call Call) Result {
	name := "call " + call.Tool
	res, err := c.Call(ctx, call.Tool, call.Args)
	if err != nil {
		return Result{Name: name, Pass: false, Detail: "call failed: " + err.Error()}
	}

	output := mcp.RenderContent(res.Content)
	wantNoError := call.Assert.NotError == nil || *call.Assert.NotError
	switch {
	case wantNoError && res.IsError:
		return Result{Name: name, Pass: false, Detail: "tool returned an error: " + output}
	case !wantNoError && !res.IsError:
		return Result{Name: name, Pass: false, Detail: "expected an error but the call succeeded"}
	}

	if call.Assert.Contains != "" && !strings.Contains(output, call.Assert.Contains) {
		return Result{Name: name, Pass: false, Detail: fmt.Sprintf("result does not contain %q", call.Assert.Contains)}
	}
	return Result{Name: name, Pass: true, Detail: "ok"}
}
