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

func TestRenderPromptRolesAndText(t *testing.T) {
	out := RenderPrompt(&sdk.GetPromptResult{Messages: []*sdk.PromptMessage{
		{Role: "user", Content: &sdk.TextContent{Text: "summarize this"}},
	}})
	if !strings.Contains(out, "[user]") || !strings.Contains(out, "summarize this") {
		t.Errorf("prompt not rendered: %q", out)
	}
}
