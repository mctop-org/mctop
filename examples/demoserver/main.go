// Command demoserver is a tiny MCP server over streamable HTTP, used to exercise
// mctop's http transport and as a public endpoint to point mctop at:
//
//	mctop ls http://localhost:8080/mcp
//
// It listens on $PORT (default 8080) and serves the MCP endpoint at /mcp.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoIn struct {
	Text string `json:"text" jsonschema:"the text to echo back"`
}

type addIn struct {
	A float64 `json:"a" jsonschema:"the first addend"`
	B float64 `json:"b" jsonschema:"the second addend"`
}

type addOut struct {
	Sum float64 `json:"sum" jsonschema:"the sum of a and b"`
}

func newServer() *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "mctop-demo", Version: "1"}, nil)

	sdk.AddTool(s, &sdk.Tool{Name: "echo", Description: "Echo the given text back."},
		func(_ context.Context, _ *sdk.CallToolRequest, in echoIn) (*sdk.CallToolResult, any, error) {
			return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: in.Text}}}, nil, nil
		})

	sdk.AddTool(s, &sdk.Tool{Name: "add", Description: "Add two numbers."},
		func(_ context.Context, _ *sdk.CallToolRequest, in addIn) (*sdk.CallToolResult, addOut, error) {
			return nil, addOut{Sum: in.A + in.B}, nil
		})

	s.AddPrompt(&sdk.Prompt{
		Name:        "greeting",
		Description: "A short greeting for someone, in a given tone.",
		Arguments: []*sdk.PromptArgument{
			{Name: "name", Description: "who to greet", Required: true},
			{Name: "tone", Description: "e.g. formal, cheerful"},
		},
	}, func(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
		tone := req.Params.Arguments["tone"]
		if tone == "" {
			tone = "friendly"
		}
		text := fmt.Sprintf("Write a %s greeting for %s.", tone, req.Params.Arguments["name"])
		return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{
			{Role: "user", Content: &sdk.TextContent{Text: text}},
		}}, nil
	})

	return s
}

// requireToken guards next with a bearer token when one is configured; an empty
// token leaves the endpoint open so the server runs unauthenticated by default.
func requireToken(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return newServer() }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", requireToken(os.Getenv("DEMO_TOKEN"), handler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	addr := ":" + port
	log.Printf("mctop demo MCP server listening on %s (endpoint /mcp)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
