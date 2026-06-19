package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func toolWithArgs() *sdk.Tool {
	return &sdk.Tool{
		Name: "get_time",
		InputSchema: map[string]any{
			"properties": map[string]any{
				"timezone": map[string]any{"type": "string"},
				"hour24":   map[string]any{"type": "boolean"},
			},
			"required": []any{"timezone"},
		},
	}
}

func TestEnterOpensForm(t *testing.T) {
	m := model{tools: []*sdk.Tool{toolWithArgs()}, width: 80}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.screen != form {
		t.Fatalf("enter should open the form, screen=%v", got.screen)
	}
	if len(got.inputs) != 2 {
		t.Fatalf("want 2 inputs, got %d", len(got.inputs))
	}
}

func TestCollectArgsCoercesTypes(t *testing.T) {
	mk := func(name, val string) formInput {
		ti := textinput.New()
		ti.SetValue(val)
		return formInput{arg: Arg{Name: name}, input: ti}
	}
	args := collectArgs([]formInput{
		mk("count", "5"),
		mk("name", "hello"),
		mk("on", "true"),
		mk("empty", "  "),
	})
	if args["count"] != float64(5) {
		t.Errorf("count: want 5, got %v (%T)", args["count"], args["count"])
	}
	if args["name"] != "hello" {
		t.Errorf("name: want hello, got %v", args["name"])
	}
	if args["on"] != true {
		t.Errorf("on: want true, got %v", args["on"])
	}
	if _, ok := args["empty"]; ok {
		t.Error("blank field should be omitted")
	}
}

func TestResultMsgSwitchesToResult(t *testing.T) {
	m := model{formTool: toolWithArgs(), screen: form, width: 80}
	next, _ := m.Update(callResultMsg{output: "12:00", elapsed: "8ms"})
	got := next.(model)
	if got.screen != result || got.output != "12:00" {
		t.Fatalf("result msg should show result, screen=%v output=%q", got.screen, got.output)
	}
}
