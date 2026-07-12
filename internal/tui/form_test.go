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

func TestEnterOnResourceOpensResult(t *testing.T) {
	m := model{
		resources: []*sdk.Resource{{URI: "file://readme"}},
		section:   secResources,
		width:     80,
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.screen != result || !got.running || got.resultTitle != "file://readme" {
		t.Fatalf("resource enter: screen=%v running=%v title=%q", got.screen, got.running, got.resultTitle)
	}
	if cmd == nil || got.lastCmd == nil {
		t.Fatal("resource enter should set a re-runnable command")
	}
}

func promptWithArgs() *sdk.Prompt {
	return &sdk.Prompt{
		Name: "code_review",
		Arguments: []*sdk.PromptArgument{
			{Name: "language", Required: true, Description: "the source language"},
			{Name: "style"},
		},
	}
}

func TestEnterOnPromptWithArgsOpensForm(t *testing.T) {
	m := model{prompts: []*sdk.Prompt{promptWithArgs()}, section: secPrompts, width: 80}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.screen != form {
		t.Fatalf("enter on a prompt with args should open the form, screen=%v", got.screen)
	}
	if got.formPrompt == nil || got.formTool != nil {
		t.Fatalf("form should run on the prompt, formPrompt=%v formTool=%v", got.formPrompt, got.formTool)
	}
	if len(got.inputs) != 2 {
		t.Fatalf("want 2 inputs, got %d", len(got.inputs))
	}
	if got.inputs[0].arg.Name != "language" || !got.inputs[0].arg.Required {
		t.Fatalf("declared order and required flag should carry over, first=%+v", got.inputs[0].arg)
	}
}

func TestEnterOnPromptWithoutArgsRendersDirectly(t *testing.T) {
	m := model{prompts: []*sdk.Prompt{{Name: "plain"}}, section: secPrompts, width: 80}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.screen != result || !got.running || got.resultTitle != "plain" {
		t.Fatalf("zero-arg prompt should render directly: screen=%v running=%v title=%q", got.screen, got.running, got.resultTitle)
	}
	if cmd == nil || got.lastCmd == nil {
		t.Fatal("prompt enter should set a re-runnable command")
	}
}

func TestPromptFormEnterRequiresRequiredArg(t *testing.T) {
	m := model{prompts: []*sdk.Prompt{promptWithArgs()}, section: secPrompts, width: 80}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(model)
	if got.screen != form || got.formMsg == "" {
		t.Fatalf("blank required arg should hold the form with a message, screen=%v msg=%q", got.screen, got.formMsg)
	}
}

func TestPromptFormEnterDispatchesRender(t *testing.T) {
	m := model{prompts: []*sdk.Prompt{promptWithArgs()}, section: secPrompts, width: 80}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	got.inputs[0].input.SetValue("go")
	next, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(model)
	if got.screen != result || !got.running || got.resultTitle != "code_review" {
		t.Fatalf("filled prompt form should dispatch: screen=%v running=%v title=%q", got.screen, got.running, got.resultTitle)
	}
	if cmd == nil || got.lastCmd == nil {
		t.Fatal("prompt form enter should set a re-runnable command")
	}
}

func TestEditReopensPromptForm(t *testing.T) {
	m := model{formPrompt: promptWithArgs(), screen: result, width: 80}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	got := next.(model)
	if got.screen != form {
		t.Fatalf("e should reopen the prompt form, screen=%v", got.screen)
	}
}

func TestCollectStringArgsKeepsRawStrings(t *testing.T) {
	mk := func(name, val string) formInput {
		ti := textinput.New()
		ti.SetValue(val)
		return formInput{arg: Arg{Name: name}, input: ti}
	}
	args := collectStringArgs([]formInput{
		mk("count", "5"),
		mk("empty", "  "),
	})
	if args["count"] != "5" {
		t.Errorf("prompt args stay strings: want \"5\", got %q", args["count"])
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
