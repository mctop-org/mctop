package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestConnectOverSSE checks that the SSE option dials the legacy HTTP+SSE
// transport and runs a real list-and-call against a server that speaks it.
func TestConnectOverSSE(t *testing.T) {
	srv := sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	sdk.AddTool(srv, &sdk.Tool{Name: "ping", Description: "reply ok"},
		func(_ context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "ok"}}}, nil, nil
		})
	handler := sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return srv }, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := Connect(ctx, ts.URL, Options{SSE: true})
	if err != nil {
		t.Fatalf("connect over sse: %v", err)
	}
	defer c.Close()

	tools, err := c.Tools(ctx)
	if err != nil {
		t.Fatalf("list tools over sse: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("tools = %v, want [ping]", tools)
	}
	res, err := c.Call(ctx, "ping", map[string]any{})
	if err != nil {
		t.Fatalf("call over sse: %v", err)
	}
	if res.IsError {
		t.Fatalf("call over sse returned isError")
	}
}
