package mcp

import (
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRenderResourceTextAndBlob(t *testing.T) {
	out := RenderResource(&sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{
		{Text: "hello"},
		{Blob: []byte{1, 2, 3}, MIMEType: "image/png"},
	}})
	if !strings.Contains(out, "hello") {
		t.Errorf("missing text content: %q", out)
	}
	if !strings.Contains(out, "3 bytes of image/png") {
		t.Errorf("blob not summarized: %q", out)
	}
}

func TestRenderResultPrefersStructure(t *testing.T) {
	// a plain JSON text block passes through unchanged
	jsonText := `{"records":[{"id":1}]}`
	if got := RenderResult(&sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: jsonText}}}); got != jsonText {
		t.Errorf("json text block = %q, want %q", got, jsonText)
	}

	// JSON wrapped in a markdown fence is unwrapped so it can be parsed
	fenced := "```json\n{\"a\":1}\n```"
	if got := RenderResult(&sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: fenced}}}); got != `{"a":1}` {
		t.Errorf("fenced json = %q, want %q", got, `{"a":1}`)
	}

	// non-JSON text falls back to the structured payload when the server set one
	res := &sdk.CallToolResult{
		Content:           []sdk.Content{&sdk.TextContent{Text: "Here are your results."}},
		StructuredContent: map[string]any{"total": 2},
	}
	if got := RenderResult(res); got != `{"total":2}` {
		t.Errorf("structured fallback = %q, want %q", got, `{"total":2}`)
	}

	// plain prose with no structured payload stays as text
	prose := &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "all good"}}}
	if got := RenderResult(prose); got != "all good" {
		t.Errorf("prose = %q, want %q", got, "all good")
	}
}

func TestUnwrapJSON(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```": `{"a":1}`,
		"```\n[1,2]\n```":         `[1,2]`,
		"  {\"a\":1}  ":           `{"a":1}`,
		"plain text":              "plain text",
	}
	for in, want := range cases {
		if got := unwrapJSON(in); got != want {
			t.Errorf("unwrapJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderPromptRolesAndText(t *testing.T) {
	out := RenderPrompt(&sdk.GetPromptResult{Messages: []*sdk.PromptMessage{
		{Role: "user", Content: &sdk.TextContent{Text: "summarize this"}},
	}})
	if !strings.Contains(out, "## user") || !strings.Contains(out, "summarize this") {
		t.Errorf("prompt not rendered: %q", out)
	}
}
